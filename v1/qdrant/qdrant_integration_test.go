package qdrant

import (
	"context"
	"fmt"
	"net"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/Aleph-Alpha/std/v1/vectordb"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/go-connections/nat"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"go.uber.org/fx"
	"go.uber.org/fx/fxtest"
)

// QdrantContainer represents a Qdrant container for testing
type QdrantContainer struct {
	testcontainers.Container
	Host string
	Port string
}

// setupQdrantContainer sets up a Qdrant container for testing
func setupQdrantContainer(ctx context.Context) (*QdrantContainer, error) {
	// Get a random free port
	port, err := getFreePort()
	if err != nil {
		return nil, fmt.Errorf("could not get free port: %w", err)
	}

	portStr := fmt.Sprintf("%d", port)
	portBindings := nat.PortMap{
		"6334/tcp": []nat.PortBinding{{HostPort: portStr}},
	}

	// Define container request
	req := testcontainers.ContainerRequest{
		Image: "qdrant/qdrant:v1.11.0",
		Env: map[string]string{
			"QDRANT__SERVICE__GRPC_PORT": "6334",
		},
		ExposedPorts: []string{"6334/tcp"},
		HostConfigModifier: func(cfg *container.HostConfig) {
			cfg.PortBindings = portBindings
		},
		WaitingFor: wait.ForListeningPort("6334/tcp").WithStartupTimeout(60 * time.Second),
	}

	// Start container
	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to start qdrant container: %w", err)
	}

	// Get host
	host, err := container.Host(ctx)
	if err != nil {
		_ = container.Terminate(ctx)
		return nil, fmt.Errorf("failed to get host: %w", err)
	}

	// Get mapped port
	mappedPort, err := container.MappedPort(ctx, "6334")
	if err != nil {
		_ = container.Terminate(ctx)
		return nil, fmt.Errorf("failed to get mapped port: %w", err)
	}

	portStr = mappedPort.Port()

	// Wait for Qdrant to be fully ready
	fmt.Printf("Waiting for Qdrant to be ready on %s:%s...\n", host, portStr)
	err = waitForQdrantReady(host, portStr, 30*time.Second)
	if err != nil {
		_ = container.Terminate(ctx)
		return nil, fmt.Errorf("qdrant container not ready: %w", err)
	}
	fmt.Printf("Qdrant is ready on %s:%s\n", host, portStr)

	return &QdrantContainer{
		Container: container,
		Host:      host,
		Port:      portStr,
	}, nil
}

// getFreePort gets a free port from the OS
func getFreePort() (int, error) {
	addr, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		return 0, err
	}
	defer func(addr net.Listener) {
		err := addr.Close()
		if err != nil {
			fmt.Printf("Failed to close listener: %v", err)
		}
	}(addr)

	return addr.Addr().(*net.TCPAddr).Port, nil
}

// waitForQdrantReady attempts to connect to Qdrant until it's ready or times out
func waitForQdrantReady(host, port string, timeout time.Duration) error {
	startTime := time.Now()
	for {
		if time.Since(startTime) > timeout {
			return fmt.Errorf("timed out waiting for Qdrant to be ready after %s", timeout)
		}

		// Try to establish a TCP connection
		conn, err := net.DialTimeout("tcp", net.JoinHostPort(host, port), 2*time.Second)
		if err == nil {
			_ = conn.Close()
			// Additional wait to ensure the service is fully ready
			time.Sleep(2 * time.Second)
			return nil
		}

		time.Sleep(500 * time.Millisecond)
	}
}

// TestMain sets up the testing environment
func TestMain(m *testing.M) {
	code := m.Run()
	os.Exit(code)
}

// TestQdrantWithFXModule tests the qdrant package using the existing FX module
func TestQdrantWithFXModule(t *testing.T) {
	// Skip if running in short mode
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	containerInstance, err := setupQdrantContainer(ctx)
	require.NoError(t, err)
	defer func() {
		if err := containerInstance.Terminate(ctx); err != nil {
			t.Fatalf("failed to terminate container: %s", err)
		}
	}()

	// Print connection details for debugging
	t.Logf("Using Qdrant on %s:%s", containerInstance.Host, containerInstance.Port)

	// Convert port to uint
	portNum, err := strconv.Atoi(containerInstance.Port)
	require.NoError(t, err)

	var qdrantClient *QdrantClient

	// Create a test app using the existing FXModule
	app := fxtest.New(t,
		fx.Provide(
			func() *Config {
				return &Config{
					Endpoint:           containerInstance.Host,
					Port:               portNum,
					CheckCompatibility: false,
					Timeout:            10 * time.Second,
				}
			},
		),
		FXModule,
		fx.Populate(&qdrantClient),
	)

	// Start the application
	err = app.Start(ctx)
	require.NoError(t, err)

	// Check if qdrant client was populated
	require.NotNil(t, qdrantClient)
	require.NotNil(t, qdrantClient.api)

	// Verify the connection is working via health check
	err = qdrantClient.healthCheck()
	assert.NoError(t, err)

	// Create adapter for operations
	adapter := NewAdapter(qdrantClient.api)

	// Test collection operations
	t.Run("EnsureCollection", func(t *testing.T) {
		// First call should create the collection
		err := adapter.EnsureCollection(ctx, "test_collection_1", 1536)
		assert.NoError(t, err)

		// Second call should be idempotent
		err = adapter.EnsureCollection(ctx, "test_collection_1", 1536)
		assert.NoError(t, err)

		// Empty collection name should fail
		err = adapter.EnsureCollection(ctx, "", 1536)
		assert.Error(t, err)
	})

	// Test basic CRUD operations
	t.Run("BasicCRUDOperations", func(t *testing.T) {
		collectionName := "test_crud"
		err := adapter.EnsureCollection(ctx, collectionName, 1536)
		require.NoError(t, err)

		// Insert single embedding (use UUID format)
		embedding := vectordb.EmbeddingInput{
			ID:     "00000000-0000-0000-0000-000000000001",
			Vector: generateRandomVector(1536),
			Payload: map[string]any{
				"title":   "Test Document 1",
				"content": "This is a test document",
			},
		}

		err = adapter.Insert(ctx, collectionName, []vectordb.EmbeddingInput{embedding})
		assert.NoError(t, err)

		// Search for the inserted embedding
		time.Sleep(1 * time.Second) // Allow time for indexing
		batchResults, err := adapter.Search(ctx, vectordb.SearchRequest{
			CollectionName: collectionName,
			Vector:         embedding.Vector,
			TopK:           5,
		})
		assert.NoError(t, err)
		assert.Greater(t, len(batchResults), 0)
		results := batchResults[0]
		assert.Greater(t, len(results), 0)

		// Verify the result
		if len(results) > 0 {
			assert.Equal(t, embedding.ID, results[0].ID)
			assert.Greater(t, results[0].Score, float32(0.9)) // Should be very similar
		}

		// Delete the embedding
		err = adapter.Delete(ctx, collectionName, []string{embedding.ID})
		assert.NoError(t, err)
	})

	// Test batch insert
	t.Run("BatchInsert", func(t *testing.T) {
		collectionName := "test_batch"
		err := adapter.EnsureCollection(ctx, collectionName, 1536)
		require.NoError(t, err)

		// Create multiple embeddings (use UUID format)
		embeddings := make([]vectordb.EmbeddingInput, 10)
		for i := 0; i < 10; i++ {
			embeddings[i] = vectordb.EmbeddingInput{
				ID:     fmt.Sprintf("00000000-0000-0000-0000-%012d", i+1),
				Vector: generateRandomVector(1536),
				Payload: map[string]any{
					"title": fmt.Sprintf("Document %d", i),
					"index": i,
				},
			}
		}

		// Batch insert
		err = adapter.Insert(ctx, collectionName, embeddings)
		assert.NoError(t, err)

		// Search and verify
		time.Sleep(1 * time.Second) // Allow time for indexing
		batchResults, err := adapter.Search(ctx, vectordb.SearchRequest{
			CollectionName: collectionName,
			Vector:         embeddings[0].Vector,
			TopK:           10,
		})
		assert.NoError(t, err)
		assert.Greater(t, len(batchResults[0]), 0)

		// Clean up
		ids := make([]string, len(embeddings))
		for i, emb := range embeddings {
			ids[i] = emb.ID
		}
		err = adapter.Delete(ctx, collectionName, ids)
		assert.NoError(t, err)
	})

	// Test empty operations
	t.Run("EmptyOperations", func(t *testing.T) {
		collectionName := "test_empty"
		err := adapter.EnsureCollection(ctx, collectionName, 1536)
		require.NoError(t, err)

		// Empty batch insert should be no-op
		err = adapter.Insert(ctx, collectionName, []vectordb.EmbeddingInput{})
		assert.NoError(t, err)

		// Empty delete should be no-op
		err = adapter.Delete(ctx, collectionName, []string{})
		assert.NoError(t, err)
	})

	// Stop the application
	require.NoError(t, app.Stop(ctx))
}

// TestVectorDBAdapterOperations tests various adapter operations
func TestVectorDBAdapterOperations(t *testing.T) {
	// Skip if running in short mode
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	containerInstance, err := setupQdrantContainer(ctx)
	require.NoError(t, err)
	defer func() {
		if err := containerInstance.Terminate(ctx); err != nil {
			t.Fatalf("failed to terminate container: %s", err)
		}
	}()

	// Convert port to uint
	portNum, err := strconv.Atoi(containerInstance.Port)
	require.NoError(t, err)

	cfg := &Config{
		Endpoint:           containerInstance.Host,
		Port:               portNum,
		CheckCompatibility: false,
		Timeout:            10 * time.Second,
	}

	client, err := NewQdrantClient(QdrantParams{Config: cfg})
	require.NoError(t, err)
	require.NotNil(t, client)
	defer client.Close()

	// Create adapter
	adapter := NewAdapter(client.api)

	collectionName := "test_operations"
	// Ensure collection exists
	err = adapter.EnsureCollection(ctx, collectionName, 1536)
	require.NoError(t, err)

	t.Run("GetCollectionByName", func(t *testing.T) {
		// Fetch collection info using GetCollection
		col, err := adapter.GetCollection(ctx, collectionName)
		assert.NoError(t, err, "expected GetCollection to succeed")
		assert.NotNil(t, col, "expected non-nil collection info")

		// Validate expected metadata fields
		assert.GreaterOrEqual(t, int(col.VectorCount), 0, "vector count should be >= 0")
		assert.GreaterOrEqual(t, int(col.PointCount), 0, "points count should be >= 0")

		// Validate vector config details (size and distance)
		assert.NotZero(t, col.VectorSize, "vector size should not be zero")
		assert.NotEmpty(t, col.Distance, "distance metric should not be empty")

		// Log for debugging
		t.Logf("Collection '%s': status=%s, vectors=%d, points=%d, vectorSize=%d, distance=%s",
			col.Name,
			col.Status,
			col.VectorCount,
			col.PointCount,
			col.VectorSize,
			col.Distance,
		)
	})

	t.Run("SearchReturnsTopK", func(t *testing.T) {
		// Insert multiple embeddings (use UUID format)
		embeddings := make([]vectordb.EmbeddingInput, 20)
		for i := 0; i < 20; i++ {
			embeddings[i] = vectordb.EmbeddingInput{
				ID:      fmt.Sprintf("00000000-0000-0000-0001-%012d", i+1),
				Vector:  generateRandomVector(1536),
				Payload: map[string]any{"index": i},
			}
		}

		err := adapter.Insert(ctx, collectionName, embeddings)
		require.NoError(t, err)

		time.Sleep(1 * time.Second) // Allow time for indexing

		// Search with topK = 5
		batchResults, err := adapter.Search(ctx, vectordb.SearchRequest{
			CollectionName: collectionName,
			Vector:         embeddings[0].Vector,
			TopK:           5,
		})
		assert.NoError(t, err)
		assert.LessOrEqual(t, len(batchResults[0]), 5)

		// Search with topK = 10
		batchResults, err = adapter.Search(ctx, vectordb.SearchRequest{
			CollectionName: collectionName,
			Vector:         embeddings[0].Vector,
			TopK:           10,
		})
		assert.NoError(t, err)
		assert.LessOrEqual(t, len(batchResults[0]), 10)

		// Clean up
		ids := make([]string, len(embeddings))
		for i, emb := range embeddings {
			ids[i] = emb.ID
		}
		err = adapter.Delete(ctx, collectionName, ids)
		assert.NoError(t, err)
	})

	t.Run("SearchWithMetadata", func(t *testing.T) {
		// Insert embedding with rich metadata (UUID format, simple types only)
		embedding := vectordb.EmbeddingInput{
			ID:     "00000000-0000-0000-0002-000000000001",
			Vector: generateRandomVector(1536),
			Payload: map[string]any{
				"title":     "Test Title",
				"author":    "Test Author",
				"timestamp": time.Now().Unix(),
				"category":  "test",
			},
		}

		err := adapter.Insert(ctx, collectionName, []vectordb.EmbeddingInput{embedding})
		require.NoError(t, err)

		time.Sleep(1 * time.Second)

		// Search and verify metadata
		batchResults, err := adapter.Search(ctx, vectordb.SearchRequest{
			CollectionName: collectionName,
			Vector:         embedding.Vector,
			TopK:           1,
		})
		assert.NoError(t, err)
		assert.Greater(t, len(batchResults[0]), 0)

		if len(batchResults[0]) > 0 {
			payload := batchResults[0][0].Payload
			assert.NotNil(t, payload)
		}

		// Clean up
		err = adapter.Delete(ctx, collectionName, []string{embedding.ID})
		assert.NoError(t, err)
	})

	t.Run("LargeBatchInsert", func(t *testing.T) {
		collectionName := "test_large_batch"
		err := adapter.EnsureCollection(ctx, collectionName, 1536)
		require.NoError(t, err)

		// Create a large batch (more than defaultBatchSize, use UUID format)
		largeCount := 500
		embeddings := make([]vectordb.EmbeddingInput, largeCount)
		for i := 0; i < largeCount; i++ {
			embeddings[i] = vectordb.EmbeddingInput{
				ID:      fmt.Sprintf("00000000-0000-0000-0003-%012d", i+1),
				Vector:  generateRandomVector(1536),
				Payload: map[string]any{"index": i},
			}
		}

		// Should handle batching automatically
		err = adapter.Insert(ctx, collectionName, embeddings)
		assert.NoError(t, err)

		time.Sleep(2 * time.Second)

		// Verify some embeddings exist
		batchResults, err := adapter.Search(ctx, vectordb.SearchRequest{
			CollectionName: collectionName,
			Vector:         embeddings[0].Vector,
			TopK:           10,
		})
		assert.NoError(t, err)
		assert.Greater(t, len(batchResults[0]), 0)

		// Clean up
		ids := make([]string, len(embeddings))
		for i, emb := range embeddings {
			ids[i] = emb.ID
		}
		err = adapter.Delete(ctx, collectionName, ids)
		assert.NoError(t, err)
	})
}

// TestQdrantErrorHandling tests error scenarios
func TestQdrantErrorHandling(t *testing.T) {
	// Skip if running in short mode
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	containerInstance, err := setupQdrantContainer(ctx)
	require.NoError(t, err)
	defer func() {
		if err := containerInstance.Terminate(ctx); err != nil {
			t.Fatalf("failed to terminate container: %s", err)
		}
	}()

	// Convert port to uint
	portNum, err := strconv.Atoi(containerInstance.Port)
	require.NoError(t, err)

	cfg := &Config{
		Endpoint:           containerInstance.Host,
		Port:               portNum,
		CheckCompatibility: false,
		Timeout:            10 * time.Second,
	}

	client, err := NewQdrantClient(QdrantParams{Config: cfg})
	require.NoError(t, err)
	require.NotNil(t, client)
	defer client.Close()

	adapter := NewAdapter(client.api)

	t.Run("InvalidEndpoint", func(t *testing.T) {
		invalidCfg := &Config{
			Endpoint:           "invalid-host:9999",
			CheckCompatibility: false,
			Timeout:            2 * time.Second,
		}

		_, err := NewQdrantClient(QdrantParams{Config: invalidCfg})
		assert.Error(t, err)
	})

	t.Run("EmptyCollectionName", func(t *testing.T) {
		err := adapter.EnsureCollection(ctx, "", 1536)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "collection name cannot be empty")
	})

	t.Run("SearchOnNonExistentCollection", func(t *testing.T) {
		vector := generateRandomVector(1536)
		_, err := adapter.Search(ctx, vectordb.SearchRequest{
			CollectionName: "non_existent_collection",
			Vector:         vector,
			TopK:           5,
		})
		assert.Error(t, err)
	})
}

// TestQdrantLifecycleAndHealthCheck verifies basic lifecycle operations
func TestQdrantLifecycleAndHealthCheck(t *testing.T) {
	// Skip if running in short mode
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	containerInstance, err := setupQdrantContainer(ctx)
	require.NoError(t, err)
	defer func() {
		if err := containerInstance.Terminate(ctx); err != nil {
			t.Fatalf("failed to terminate container: %s", err)
		}
	}()

	// Convert port to uint
	portNum, err := strconv.Atoi(containerInstance.Port)
	require.NoError(t, err)

	cfg := &Config{
		Endpoint:           containerInstance.Host,
		Port:               portNum,
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

	// Create adapter and ensure collection exists
	adapter := NewAdapter(client.api)
	collectionName := "test_collection"
	err = adapter.EnsureCollection(context.Background(), collectionName, 1536)
	require.NoError(t, err, "failed to ensure collection")

	// Close client
	err = client.Close()
	require.NoError(t, err, "client close failed")

	t.Log("Qdrant client lifecycle test passed successfully")
}

// Helper function to generate random vectors for testing
func generateRandomVector(size int) []float32 {
	vector := make([]float32, size)
	for i := range vector {
		// Use a simple deterministic pattern for testing
		vector[i] = float32(i%100) / 100.0
	}
	return vector
}
