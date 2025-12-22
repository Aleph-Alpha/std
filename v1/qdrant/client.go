package qdrant

import (
	"context"
	"fmt"
	"log"
	"time"

	qdrant "github.com/qdrant/go-client/qdrant"
)

//
// ──────────────────────────────────────────────────────────────
//   QDRANT CLIENT WRAPPER
// ──────────────────────────────────────────────────────────────
//
// This file defines a thin wrapper around the official Qdrant Go client,
// providing application-level operations for managing embeddings,
// collections, and similarity search.
//
// The goal is to abstract away low-level SDK details while preserving
// fine-grained control over how Qdrant is accessed.
//
// Responsibilities:
//   • Establish and validate connectivity with Qdrant.
//   • Manage collections (create if missing).
//   • Insert, batch insert, delete, and search embeddings.
//   • Offer a safe API suitable for Fx dependency injection.
//

// QdrantClient wraps the official Qdrant Go client
// and provides higher-level operations for working with embeddings and vectors.

type QdrantClient struct {
	api     *qdrant.Client
	cfg     *Config
	started bool
}

const (
	defaultBatchSize      = 200 // default chunk size for batch inserts
	maxConcurrentSearches = 10  // default maximum concurrent searches
)

// NewQdrantClient ──────────────────────────────────────────────────────────────
// NewQdrantClient
// ──────────────────────────────────────────────────────────────
//
// NewQdrantClient constructs a new instance of QdrantClient and validates
// connectivity via a health check.
//
// The Qdrant Go SDK creates lightweight gRPC connections, so this method
// performs an immediate health check to fail fast if the service is unreachable.
//
// Example:
//
//	client, _ := qdrant.NewQdrantClient(qdrant.QdrantParams{Config: cfg})
func NewQdrantClient(p QdrantParams) (*QdrantClient, error) {
	log.Printf("[Qdrant] Connecting to endpoint: %s:%d", p.Config.Endpoint, p.Config.Port)

	// Set default port if not specified
	port := p.Config.Port
	if port == 0 {
		port = 6334
	}

	client, err := qdrant.NewClient(&qdrant.Config{
		Host:                   p.Config.Endpoint,
		Port:                   port,
		APIKey:                 p.Config.ApiKey,
		SkipCompatibilityCheck: !p.Config.CheckCompatibility,
	})
	if err != nil {
		return nil, fmt.Errorf("[Qdrant] failed to initialize client: %w", err)
	}

	qc := &QdrantClient{
		api:     client,
		cfg:     p.Config,
		started: true,
	}

	if err := qc.healthCheck(); err != nil {
		return nil, fmt.Errorf("[Qdrant] health check failed: %w", err)
	}

	log.Println("[Qdrant] Client connected successfully")
	return qc, nil
}

// ──────────────────────────────────────────────────────────────
// healthCheck
// ──────────────────────────────────────────────────────────────
//
// healthCheck verifies the availability of the Qdrant service
// by calling the `/healthz` endpoint through the SDK.
//
// It should be lightweight and fast — typically used during startup or readiness probes.
func (c *QdrantClient) healthCheck() error {
	if !c.started {
		return fmt.Errorf("[Qdrant] client not started")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if c.api == nil {
		return fmt.Errorf("[Qdrant] client not initialized")
	}

	resp, err := c.api.HealthCheck(ctx)
	if err != nil {
		return fmt.Errorf("[Qdrant] health check failed: %w", err)
	}

	log.Printf("[Qdrant] Health check passed (title=%s, version=%s, endpoint=%s)", resp.Title, resp.Version, c.cfg.Endpoint)

	return nil
}

// Client returns the underlying Qdrant SDK client.
// This is useful for direct access to low-level operations.
func (c *QdrantClient) Client() *qdrant.Client {
	return c.api
}

// Close ──────────────────────────────────────────────────────────────
// Close
// ──────────────────────────────────────────────────────────────
//
// Close gracefully shuts down the Qdrant client.
//
// Since the official Qdrant Go SDK doesn't maintain persistent connections,
// this is currently a no-op. It exists for lifecycle symmetry and future safety.
func (c *QdrantClient) Close() error {
	if !c.started {
		return nil
	}

	log.Println("[Qdrant] closing client (no-op)")
	return nil
}
