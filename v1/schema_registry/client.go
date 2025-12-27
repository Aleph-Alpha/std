package schema_registry

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/Aleph-Alpha/std/v1/observability"
)

// Registry provides an interface for interacting with a Confluent Schema Registry.
// It handles schema registration, retrieval, and caching for efficient serialization.
type Registry interface {
	// GetSchemaByID retrieves a schema by its ID
	GetSchemaByID(id int) (string, error)

	// GetLatestSchema retrieves the latest version of a schema for a subject
	GetLatestSchema(subject string) (*Metadata, error)

	// RegisterSchema registers a new schema for a subject
	RegisterSchema(subject, schema, schemaType string) (int, error)

	// CheckCompatibility checks if a schema is compatible with the latest version
	CheckCompatibility(subject, schema, schemaType string) (bool, error)
}

// Metadata contains metadata about a registered schema
type Metadata struct {
	ID      int    `json:"id"`
	Version int    `json:"version"`
	Schema  string `json:"schema"`
	Subject string `json:"subject"`
	Type    string `json:"schemaType,omitempty"`
}

// Client is the default implementation of Registry
// that communicates with Confluent Schema Registry over HTTP.
type Client struct {
	url        string
	httpClient *http.Client

	// Cache for schemas by ID
	schemaCache      map[int]string
	schemaCacheMutex sync.RWMutex

	// Cache for schema IDs by subject and schema
	idCache      map[string]int
	idCacheMutex sync.RWMutex

	// Authentication
	username string
	password string

	// observer provides optional observability hooks for tracking operations
	observer observability.Observer

	// logger provides optional context-aware logging capabilities
	logger Logger
}

// Config holds configuration for schema registry client
type Config struct {
	// URL is the schema registry endpoint (e.g., "http://localhost:8081")
	URL string

	// Username for basic auth (optional)
	Username string

	// Password for basic auth (optional)
	Password string

	// Timeout for HTTP requests
	Timeout time.Duration
}

// Logger is an interface that matches the std/v1/logger.Logger interface.
// It provides context-aware structured logging with optional error and field parameters.
type Logger interface {
	// InfoWithContext logs an informational message with trace context.
	InfoWithContext(ctx context.Context, msg string, err error, fields ...map[string]interface{})

	// WarnWithContext logs a warning message with trace context.
	WarnWithContext(ctx context.Context, msg string, err error, fields ...map[string]interface{})

	// ErrorWithContext logs an error message with trace context.
	ErrorWithContext(ctx context.Context, msg string, err error, fields ...map[string]interface{})
}

// NewClient creates a new schema registry client
// Returns the concrete *Client type.
func NewClient(config Config) (*Client, error) {
	if config.URL == "" {
		return nil, fmt.Errorf("schema registry URL is required")
	}

	if config.Timeout == 0 {
		config.Timeout = 10 * time.Second
	}

	return &Client{
		url: config.URL,
		httpClient: &http.Client{
			Timeout: config.Timeout,
		},
		schemaCache: make(map[int]string),
		idCache:     make(map[string]int),
		username:    config.Username,
		password:    config.Password,
	}, nil
}

// GetSchemaByID retrieves a schema from the registry by its ID
func (c *Client) GetSchemaByID(id int) (string, error) {
	start := time.Now()

	// Check cache first
	c.schemaCacheMutex.RLock()
	if schema, ok := c.schemaCache[id]; ok {
		c.schemaCacheMutex.RUnlock()
		c.observeOperation("get_schema_by_id", "registry", fmt.Sprintf("%d", id), time.Since(start), nil, map[string]interface{}{
			"cache_hit": true,
		})
		return schema, nil
	}
	c.schemaCacheMutex.RUnlock()

	// Fetch from registry
	url := fmt.Sprintf("%s/schemas/ids/%d", c.url, id)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		c.observeOperation("get_schema_by_id", "registry", fmt.Sprintf("%d", id), time.Since(start), err, map[string]interface{}{
			"cache_hit": false,
		})
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	if c.username != "" {
		req.SetBasicAuth(c.username, c.password)
	}
	req.Header.Set("Accept", "application/vnd.schemaregistry.v1+json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		c.observeOperation("get_schema_by_id", "registry", fmt.Sprintf("%d", id), time.Since(start), err, map[string]interface{}{
			"cache_hit": false,
		})
		return "", fmt.Errorf("failed to fetch schema: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		err := fmt.Errorf("schema registry returned status %d: %s", resp.StatusCode, string(body))
		c.observeOperation("get_schema_by_id", "registry", fmt.Sprintf("%d", id), time.Since(start), err, map[string]interface{}{
			"cache_hit":   false,
			"status_code": resp.StatusCode,
		})
		return "", err
	}

	var result struct {
		Schema string `json:"schema"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		c.observeOperation("get_schema_by_id", "registry", fmt.Sprintf("%d", id), time.Since(start), err, map[string]interface{}{
			"cache_hit": false,
		})
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	// Cache the schema
	c.schemaCacheMutex.Lock()
	c.schemaCache[id] = result.Schema
	c.schemaCacheMutex.Unlock()

	c.observeOperation("get_schema_by_id", "registry", fmt.Sprintf("%d", id), time.Since(start), nil, map[string]interface{}{
		"cache_hit": false,
	})
	return result.Schema, nil
}

// GetLatestSchema retrieves the latest version of a schema for a subject
func (c *Client) GetLatestSchema(subject string) (*Metadata, error) {
	start := time.Now()

	url := fmt.Sprintf("%s/subjects/%s/versions/latest", c.url, subject)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		c.observeOperation("get_latest_schema", subject, "latest", time.Since(start), err, nil)
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	if c.username != "" {
		req.SetBasicAuth(c.username, c.password)
	}
	req.Header.Set("Accept", "application/vnd.schemaregistry.v1+json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		c.observeOperation("get_latest_schema", subject, "latest", time.Since(start), err, nil)
		return nil, fmt.Errorf("failed to fetch schema: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		err := fmt.Errorf("schema registry returned status %d: %s", resp.StatusCode, string(body))
		c.observeOperation("get_latest_schema", subject, "latest", time.Since(start), err, map[string]interface{}{
			"status_code": resp.StatusCode,
		})
		return nil, err
	}

	var metadata Metadata
	if err := json.NewDecoder(resp.Body).Decode(&metadata); err != nil {
		c.observeOperation("get_latest_schema", subject, "latest", time.Since(start), err, nil)
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	metadata.Subject = subject

	// Cache the schema
	c.schemaCacheMutex.Lock()
	c.schemaCache[metadata.ID] = metadata.Schema
	c.schemaCacheMutex.Unlock()

	c.observeOperation("get_latest_schema", subject, "latest", time.Since(start), nil, map[string]interface{}{
		"schema_id":   metadata.ID,
		"version":     metadata.Version,
		"schema_type": metadata.Type,
	})
	return &metadata, nil
}

// RegisterSchema registers a new schema with the schema registry
func (c *Client) RegisterSchema(subject, schema, schemaType string) (int, error) {
	start := time.Now()

	// Check cache first
	cacheKey := fmt.Sprintf("%s:%s:%s", subject, schemaType, schema)
	c.idCacheMutex.RLock()
	if id, ok := c.idCache[cacheKey]; ok {
		c.idCacheMutex.RUnlock()
		c.observeOperation("register_schema", subject, fmt.Sprintf("%d", id), time.Since(start), nil, map[string]interface{}{
			"cache_hit":   true,
			"schema_type": schemaType,
		})
		return id, nil
	}
	c.idCacheMutex.RUnlock()

	url := fmt.Sprintf("%s/subjects/%s/versions", c.url, subject)

	payload := map[string]interface{}{
		"schema": schema,
	}
	if schemaType != "" && schemaType != "AVRO" {
		payload["schemaType"] = schemaType
	}

	body, err := json.Marshal(payload)
	if err != nil {
		c.observeOperation("register_schema", subject, "", time.Since(start), err, map[string]interface{}{
			"cache_hit":   false,
			"schema_type": schemaType,
		})
		return 0, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		c.observeOperation("register_schema", subject, "", time.Since(start), err, map[string]interface{}{
			"cache_hit":   false,
			"schema_type": schemaType,
		})
		return 0, fmt.Errorf("failed to create request: %w", err)
	}

	if c.username != "" {
		req.SetBasicAuth(c.username, c.password)
	}
	req.Header.Set("Content-Type", "application/vnd.schemaregistry.v1+json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		c.observeOperation("register_schema", subject, "", time.Since(start), err, map[string]interface{}{
			"cache_hit":   false,
			"schema_type": schemaType,
		})
		return 0, fmt.Errorf("failed to register schema: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		err := fmt.Errorf("schema registry returned status %d: %s", resp.StatusCode, string(respBody))
		c.observeOperation("register_schema", subject, "", time.Since(start), err, map[string]interface{}{
			"cache_hit":   false,
			"schema_type": schemaType,
			"status_code": resp.StatusCode,
		})
		return 0, err
	}

	var result struct {
		ID int `json:"id"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		c.observeOperation("register_schema", subject, "", time.Since(start), err, map[string]interface{}{
			"cache_hit":   false,
			"schema_type": schemaType,
		})
		return 0, fmt.Errorf("failed to decode response: %w", err)
	}

	// Cache the ID
	c.idCacheMutex.Lock()
	c.idCache[cacheKey] = result.ID
	c.idCacheMutex.Unlock()

	c.observeOperation("register_schema", subject, fmt.Sprintf("%d", result.ID), time.Since(start), nil, map[string]interface{}{
		"cache_hit":   false,
		"schema_type": schemaType,
		"schema_id":   result.ID,
	})
	return result.ID, nil
}

// CheckCompatibility checks if a schema is compatible with the existing schema for a subject
func (c *Client) CheckCompatibility(subject, schema, schemaType string) (bool, error) {
	start := time.Now()

	url := fmt.Sprintf("%s/compatibility/subjects/%s/versions/latest", c.url, subject)

	payload := map[string]interface{}{
		"schema": schema,
	}
	if schemaType != "" && schemaType != "AVRO" {
		payload["schemaType"] = schemaType
	}

	body, err := json.Marshal(payload)
	if err != nil {
		c.observeOperation("check_compatibility", subject, "latest", time.Since(start), err, map[string]interface{}{
			"schema_type": schemaType,
		})
		return false, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		c.observeOperation("check_compatibility", subject, "latest", time.Since(start), err, map[string]interface{}{
			"schema_type": schemaType,
		})
		return false, fmt.Errorf("failed to create request: %w", err)
	}

	if c.username != "" {
		req.SetBasicAuth(c.username, c.password)
	}
	req.Header.Set("Content-Type", "application/vnd.schemaregistry.v1+json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		c.observeOperation("check_compatibility", subject, "latest", time.Since(start), err, map[string]interface{}{
			"schema_type": schemaType,
		})
		return false, fmt.Errorf("failed to check compatibility: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		err := fmt.Errorf("schema registry returned status %d: %s", resp.StatusCode, string(respBody))
		c.observeOperation("check_compatibility", subject, "latest", time.Since(start), err, map[string]interface{}{
			"schema_type": schemaType,
			"status_code": resp.StatusCode,
		})
		return false, err
	}

	var result struct {
		IsCompatible bool `json:"is_compatible"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		c.observeOperation("check_compatibility", subject, "latest", time.Since(start), err, map[string]interface{}{
			"schema_type": schemaType,
		})
		return false, fmt.Errorf("failed to decode response: %w", err)
	}

	c.observeOperation("check_compatibility", subject, "latest", time.Since(start), nil, map[string]interface{}{
		"schema_type":   schemaType,
		"is_compatible": result.IsCompatible,
	})
	return result.IsCompatible, nil
}

// EncodeSchemaID encodes a schema ID in the Confluent wire format
// Format: [magic_byte][schema_id]
// - magic_byte: 0x0 (1 byte)
// - schema_id: 4 bytes (big-endian)
func EncodeSchemaID(schemaID int) []byte {
	buf := make([]byte, 5)
	buf[0] = 0x0 // Magic byte
	binary.BigEndian.PutUint32(buf[1:], uint32(schemaID))
	return buf
}

// DecodeSchemaID decodes a schema ID from the Confluent wire format
// Returns the schema ID and the remaining payload (after the 5-byte header)
func DecodeSchemaID(data []byte) (int, []byte, error) {
	if len(data) < 5 {
		return 0, nil, fmt.Errorf("data too short: expected at least 5 bytes, got %d", len(data))
	}

	if data[0] != 0x0 {
		return 0, nil, fmt.Errorf("invalid magic byte: expected 0x0, got 0x%x", data[0])
	}

	schemaID := int(binary.BigEndian.Uint32(data[1:5]))
	payload := data[5:]

	return schemaID, payload, nil
}

// WithObserver sets the observer for this client and returns the client for method chaining.
// The observer receives events about schema registry operations (e.g., register, get, check compatibility).
//
// Example:
//
//	client := client.WithObserver(myObserver).WithLogger(myLogger)
func (c *Client) WithObserver(observer observability.Observer) *Client {
	c.observer = observer
	return c
}

// WithLogger sets the logger for this client and returns the client for method chaining.
// The logger is used for structured logging of client operations and errors.
//
// Example:
//
//	client := client.WithObserver(myObserver).WithLogger(myLogger)
func (c *Client) WithLogger(logger Logger) *Client {
	c.logger = logger
	return c
}

// logInfo logs an informational message if a logger is configured
func (c *Client) logInfo(ctx context.Context, msg string, fields map[string]interface{}) {
	if c.logger != nil {
		c.logger.InfoWithContext(ctx, msg, nil, fields)
	}
}

// logWarn logs a warning message if a logger is configured
func (c *Client) logWarn(ctx context.Context, msg string, fields map[string]interface{}) {
	if c.logger != nil {
		c.logger.WarnWithContext(ctx, msg, nil, fields)
	}
}

// logError logs an error message if a logger is configured
func (c *Client) logError(ctx context.Context, msg string, err error, fields map[string]interface{}) {
	if c.logger != nil {
		c.logger.ErrorWithContext(ctx, msg, err, fields)
	}
}
