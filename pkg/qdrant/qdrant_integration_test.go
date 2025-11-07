package qdrant

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestQdrantLifecycleAndHealthCheck verifies that the Qdrant client can
// connect to a running Qdrant instance (e.g., one started via Docker Compose).
//
// Prerequisite:
//
//	Run `docker compose up -d` with a Qdrant service exposing port 6333
//	before executing this test.
func TestQdrantLifecycleAndHealthCheck(t *testing.T) {
	t.Log("Starting Qdrant lifecycle test (no Testcontainers)...")

	// Point directly to local or remote Qdrant instance
	cfg := &Config{
		Endpoint:           "localhost", // assuming Qdrant runs locally
		Collection:         "test_collection",
		CheckCompatibility: false,
		Timeout:            5 * time.Second,
	}

	// Create a new Qdrant client
	client, err := NewQdrantClient(QdrantParams{Config: cfg})
	require.NoError(t, err, "client initialization failed")
	require.NotNil(t, client, "expected non-nil Qdrant client")

	// Perform a health check
	err = client.healthCheck()
	require.NoError(t, err, "Qdrant health check failed")

	// Ensure collection exists
	err = client.EnsureCollection(context.Background(), cfg.Collection)
	require.NoError(t, err, "failed to ensure collection")

	// Close client
	err = client.Close()
	require.NoError(t, err, "client close failed")

	t.Log("Qdrant client lifecycle test passed successfully")
}
