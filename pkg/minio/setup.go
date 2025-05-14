package minio

import (
	"bytes"
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

//go:generate mockgen -source=setup.go -destination=mock_logger.go -package=minio
type Logger interface {
	Info(msg string, err error, fields ...map[string]interface{})
	Debug(msg string, err error, fields ...map[string]interface{})
	Warn(msg string, err error, fields ...map[string]interface{})
	Error(msg string, err error, fields ...map[string]interface{})
	Fatal(msg string, err error, fields ...map[string]interface{})
}

// Minio represents a MinIO client with additional functionality
type Minio struct {
	Client          *minio.Client
	CoreClient      *minio.Core // Added a core client for low-level operations
	cfg             Config
	logger          Logger
	mu              sync.RWMutex
	shutdownSignal  chan struct{}
	reconnectSignal chan error
	bufferPool      *BufferPool
}

// BufferPool implements a pool of bytes.Buffers
type BufferPool struct {
	pool sync.Pool
}

// NewBufferPool creates a new BufferPool
func NewBufferPool() *BufferPool {
	return &BufferPool{
		pool: sync.Pool{
			New: func() interface{} {
				return new(bytes.Buffer)
			},
		},
	}
}

// Get returns a buffer from the pool
func (bp *BufferPool) Get() *bytes.Buffer {
	return bp.pool.Get().(*bytes.Buffer)
}

// Put returns a buffer to the pool
func (bp *BufferPool) Put(b *bytes.Buffer) {
	bp.pool.Put(b)
}

// NewClient creates and validates a new MinIO client
func NewClient(cfg Config, logger Logger) (*Minio, error) {
	// Create the standard client
	client, err := connectToMinio(cfg, logger)
	if err != nil {
		logger.Error("failed to connect to minio", err, map[string]interface{}{
			"endpoint": cfg.Connection.Endpoint,
			"region":   cfg.Connection.Region,
			"secure":   cfg.Connection.UseSSL,
			"bucket":   cfg.Connection.BucketName,
		})
		return nil, err
	}

	// Create the core client using the same connection parameters
	coreClient, err := connectToMinioCore(cfg, logger)
	if err != nil {
		logger.Error("failed to create core minio client", err, map[string]interface{}{
			"endpoint": cfg.Connection.Endpoint,
			"region":   cfg.Connection.Region,
			"secure":   cfg.Connection.UseSSL,
		})
		return nil, err
	}

	minioClient := &Minio{
		Client:          client,
		CoreClient:      coreClient,
		cfg:             cfg,
		logger:          logger,
		shutdownSignal:  make(chan struct{}),
		reconnectSignal: make(chan error),
		bufferPool:      NewBufferPool(),
	}

	timeoutCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := minioClient.validateConnection(timeoutCtx); err != nil {
		logger.Error("failed to validate minio connection", err, map[string]interface{}{
			"endpoint": cfg.Connection.Endpoint,
			"region":   cfg.Connection.Region,
			"secure":   cfg.Connection.UseSSL,
			"bucket":   cfg.Connection.BucketName,
		})
		return nil, err
	}
	if err := minioClient.ensureBucketExists(timeoutCtx); err != nil {
		logger.Error("failed to verify bucket", err, map[string]interface{}{
			"endpoint": cfg.Connection.Endpoint,
			"region":   cfg.Connection.Region,
			"secure":   cfg.Connection.UseSSL,
			"bucket":   cfg.Connection.BucketName,
		})
		return nil, err
	}

	return minioClient, nil
}

// monitorConnection periodically checks the MinIO connection and triggers reconnecting if needed
func (m *Minio) monitorConnection(ctx context.Context) {
	ticker := time.NewTicker(connectionHealthCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// Check if the connection is still alive
			checkCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			err := m.validateConnection(checkCtx)
			cancel()

			if err != nil {
				m.logger.Error("MinIO connection health check failed", err, map[string]interface{}{
					"endpoint": m.cfg.Connection.Endpoint,
				})

				// Signal connection problem to the retry goroutine
				select {
				case m.reconnectSignal <- err: // Non-blocking send
				default: // Channel already has a pending reconnected signal
				}
			}

		case <-m.shutdownSignal:
			return

		case <-ctx.Done():
			return
		}
	}
}

// retryConnection manages reconnection to MinIO using a similar pattern to the Rabbit implementation
func (m *Minio) retryConnection(ctx context.Context) {
outerLoop:
	for {
		select {
		case <-m.shutdownSignal:
			m.logger.Info("Stopping MinIO connection retry loop due to shutdown signal", nil, nil)
			close(m.reconnectSignal)
			return

		case <-ctx.Done():
			m.logger.Info("Stopping MinIO connection retry loop due to context cancellation", nil, nil)
			close(m.reconnectSignal)
			return

		case err := <-m.reconnectSignal:
			m.logger.Warn("MinIO connection issue detected, attempting reconnection", err, map[string]interface{}{
				"endpoint": m.cfg.Connection.Endpoint,
			})

		reconnectLoop:
			for {
				select {
				case <-m.shutdownSignal:
					m.logger.Info("Stopping MinIO connection retry loop during reconnection due to shutdown signal", nil, nil)
					close(m.reconnectSignal)
					return

				case <-ctx.Done():
					m.logger.Info("Stopping MinIO connection retry loop during reconnection due to context cancellation", nil, nil)
					close(m.reconnectSignal)
					return

				default:
					// Create a context with timeout for the reconnection attempt
					ctxReconnect, cancel := context.WithTimeout(context.Background(), 10*time.Second)

					// Attempt to create new clients
					newClient, err := connectToMinio(m.cfg, m.logger)
					if err != nil {
						cancel() // Cancel the context to free resources
						m.logger.Error("MinIO reconnection failed", err, map[string]interface{}{
							"endpoint":      m.cfg.Connection.Endpoint,
							"will_retry_in": "1 second",
						})
						time.Sleep(time.Second)
						continue reconnectLoop
					}

					newCoreClient, err := connectToMinioCore(m.cfg, m.logger)
					if err != nil {
						cancel() // Cancel the context to free resources
						m.logger.Error("MinIO core client reconnection failed", err, map[string]interface{}{
							"endpoint":      m.cfg.Connection.Endpoint,
							"will_retry_in": "1 second",
						})
						time.Sleep(time.Second)
						continue reconnectLoop
					}

					// Check if the new connection is healthy
					m.mu.Lock()
					m.Client = newClient
					m.mu.Unlock()

					err = m.validateConnection(ctxReconnect)
					if err != nil {
						cancel() // Cancel the context to free resources
						m.logger.Error("MinIO connection validation failed", err, nil)
						time.Sleep(time.Second)
						continue reconnectLoop
					}

					// Update the client references
					m.mu.Lock()
					m.Client = newClient
					m.CoreClient = newCoreClient
					m.mu.Unlock()

					// Verify bucket existence after reconnection
					err = m.ensureBucketExists(ctxReconnect)
					cancel() // Cancel the context to free resources

					if err != nil {
						m.logger.Error("Failed to verify bucket after reconnection", err, nil)
						time.Sleep(time.Second)
						continue reconnectLoop
					}

					m.logger.Info("Successfully reconnected to MinIO", nil, map[string]interface{}{
						"endpoint": m.cfg.Connection.Endpoint,
						"bucket":   m.cfg.Connection.BucketName,
					})
					continue outerLoop
				}
			}
		}
	}
}

func connectToMinio(cfg Config, logger Logger) (*minio.Client, error) {
	// Add validation for an empty endpoint
	if cfg.Connection.Endpoint == "" {
		return nil, fmt.Errorf("minio endpoint cannot be empty")
	}

	connectionFields := map[string]interface{}{
		"endpoint": cfg.Connection.Endpoint,
		"region":   cfg.Connection.Region,
		"secure":   cfg.Connection.UseSSL,
	}

	logger.Info("Connecting to MinIO", nil, connectionFields)

	client, err := minio.New(cfg.Connection.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.Connection.AccessKeyID, cfg.Connection.SecretAccessKey, ""),
		Secure: cfg.Connection.UseSSL,
		Region: cfg.Connection.Region,
	})

	if err != nil {
		return nil, err
	}
	return client, nil
}

// New function to create a Core client
func connectToMinioCore(cfg Config, logger Logger) (*minio.Core, error) {
	// Add validation for an empty endpoint
	if cfg.Connection.Endpoint == "" {
		return nil, fmt.Errorf("minio endpoint cannot be empty")
	}

	connectionFields := map[string]interface{}{
		"endpoint": cfg.Connection.Endpoint,
		"region":   cfg.Connection.Region,
		"secure":   cfg.Connection.UseSSL,
		"type":     "core client",
	}

	logger.Info("Creating MinIO Core client", nil, connectionFields)

	coreClient, err := minio.NewCore(cfg.Connection.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.Connection.AccessKeyID, cfg.Connection.SecretAccessKey, ""),
		Secure: cfg.Connection.UseSSL,
		Region: cfg.Connection.Region,
	})

	if err != nil {
		return nil, err
	}
	return coreClient, nil
}

// validateConnection performs a simple operation to validate connectivity
func (m *Minio) validateConnection(ctx context.Context) error {
	// Set a timeout for validation
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	// Check if we can list buckets (requires minimal permissions)
	_, err := m.Client.ListBuckets(ctx)
	if err != nil {
		return err
	}

	return nil
}

// ensureBucketExists checks if the configured bucket exists and creates it if necessary
func (m *Minio) ensureBucketExists(ctx context.Context) error {
	bucketName := m.cfg.Connection.BucketName
	if bucketName == "" {
		return fmt.Errorf("bucket name is empty")
	}

	// Set a timeout for the operation
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	exists, err := m.Client.BucketExists(ctx, bucketName)
	if err != nil {
		return fmt.Errorf("failed to check if bucket exists, bucket: %v, err: %w", bucketName, err)
	}

	if !exists {
		m.logger.Info("Bucket does not exist, creating it", nil, map[string]interface{}{
			"bucket": bucketName,
			"region": m.cfg.Connection.Region,
		})

		err = m.Client.MakeBucket(ctx, bucketName, minio.MakeBucketOptions{
			Region: m.cfg.Connection.Region,
		})

		if err != nil {
			return err
		}

		m.logger.Info("Successfully created bucket", nil, map[string]interface{}{
			"bucket": bucketName,
		})
	}

	return nil
}
