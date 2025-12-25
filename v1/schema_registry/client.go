package schema_registry

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
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
	// Check cache first
	c.schemaCacheMutex.RLock()
	if schema, ok := c.schemaCache[id]; ok {
		c.schemaCacheMutex.RUnlock()
		return schema, nil
	}
	c.schemaCacheMutex.RUnlock()

	// Fetch from registry
	url := fmt.Sprintf("%s/schemas/ids/%d", c.url, id)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	if c.username != "" {
		req.SetBasicAuth(c.username, c.password)
	}
	req.Header.Set("Accept", "application/vnd.schemaregistry.v1+json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to fetch schema: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("schema registry returned status %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Schema string `json:"schema"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	// Cache the schema
	c.schemaCacheMutex.Lock()
	c.schemaCache[id] = result.Schema
	c.schemaCacheMutex.Unlock()

	return result.Schema, nil
}

// GetLatestSchema retrieves the latest version of a schema for a subject
func (c *Client) GetLatestSchema(subject string) (*Metadata, error) {
	url := fmt.Sprintf("%s/subjects/%s/versions/latest", c.url, subject)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	if c.username != "" {
		req.SetBasicAuth(c.username, c.password)
	}
	req.Header.Set("Accept", "application/vnd.schemaregistry.v1+json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch schema: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("schema registry returned status %d: %s", resp.StatusCode, string(body))
	}

	var metadata Metadata
	if err := json.NewDecoder(resp.Body).Decode(&metadata); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	metadata.Subject = subject

	// Cache the schema
	c.schemaCacheMutex.Lock()
	c.schemaCache[metadata.ID] = metadata.Schema
	c.schemaCacheMutex.Unlock()

	return &metadata, nil
}

// RegisterSchema registers a new schema with the schema registry
func (c *Client) RegisterSchema(subject, schema, schemaType string) (int, error) {
	// Check cache first
	cacheKey := fmt.Sprintf("%s:%s:%s", subject, schemaType, schema)
	c.idCacheMutex.RLock()
	if id, ok := c.idCache[cacheKey]; ok {
		c.idCacheMutex.RUnlock()
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
		return 0, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		return 0, fmt.Errorf("failed to create request: %w", err)
	}

	if c.username != "" {
		req.SetBasicAuth(c.username, c.password)
	}
	req.Header.Set("Content-Type", "application/vnd.schemaregistry.v1+json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("failed to register schema: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return 0, fmt.Errorf("schema registry returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		ID int `json:"id"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, fmt.Errorf("failed to decode response: %w", err)
	}

	// Cache the ID
	c.idCacheMutex.Lock()
	c.idCache[cacheKey] = result.ID
	c.idCacheMutex.Unlock()

	return result.ID, nil
}

// CheckCompatibility checks if a schema is compatible with the existing schema for a subject
func (c *Client) CheckCompatibility(subject, schema, schemaType string) (bool, error) {
	url := fmt.Sprintf("%s/compatibility/subjects/%s/versions/latest", c.url, subject)

	payload := map[string]interface{}{
		"schema": schema,
	}
	if schemaType != "" && schemaType != "AVRO" {
		payload["schemaType"] = schemaType
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return false, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		return false, fmt.Errorf("failed to create request: %w", err)
	}

	if c.username != "" {
		req.SetBasicAuth(c.username, c.password)
	}
	req.Header.Set("Content-Type", "application/vnd.schemaregistry.v1+json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return false, fmt.Errorf("failed to check compatibility: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return false, fmt.Errorf("schema registry returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		IsCompatible bool `json:"is_compatible"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return false, fmt.Errorf("failed to decode response: %w", err)
	}

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
