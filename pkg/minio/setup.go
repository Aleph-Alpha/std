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

// Logger defines the interface for logging operations within the MinIO client.
// This interface allows for dependency injection of any compatible logger implementation.
//
//go:generate mockgen -source=setup.go -destination=mock_logger.go -package=minio
type Logger interface {
	// Info logs informational messages with optional error and additional fields
	Info(msg string, err error, fields ...map[string]interface{})

	// Debug logs debug-level messages with optional error and additional fields
	Debug(msg string, err error, fields ...map[string]interface{})

	// Warn logs warning messages with optional error and additional fields
	Warn(msg string, err error, fields ...map[string]interface{})

	// Error logs error messages with the associated error and optional additional fields
	Error(msg string, err error, fields ...map[string]interface{})

	// Fatal logs critical error messages that typically require immediate attention
	Fatal(msg string, err error, fields ...map[string]interface{})
}

// Minio represents a MinIO client with additional functionality.
// It wraps the standard MinIO client with features for connection management,
// reconnection handling, and thread-safety.
type Minio struct {
	// Client is the standard MinIO client for high-level operations
	Client *minio.Client

	// CoreClient provides access to low-level operations not available in the standard client
	CoreClient *minio.Core

	// cfg holds the configuration for this MinIO client instance
	cfg Config

	// logger is used for logging operations and errors
	logger Logger

	// mu provides thread-safety for client operations
	mu sync.RWMutex

	// shutdownSignal is used to signal the connection monitor to stop
	shutdownSignal chan struct{}

	// reconnectSignal is used to trigger reconnection attempts
	reconnectSignal chan error

	// bufferPool manages reusable byte buffers to reduce memory allocations
	bufferPool *BufferPool
}

// BufferPool implements a pool of bytes.Buffers to reduce memory allocations.
// It's used for temporary buffer operations when reading or writing objects.
type BufferPool struct {
	// pool is the underlying sync.Pool that manages the buffer objects
	pool sync.Pool
}

// NewBufferPool creates a new BufferPool instance.
// The pool will create new bytes.Buffer instances as needed when none are available.
//
// Returns a configured BufferPool ready for use.
func NewBufferPool() *BufferPool {
	return &BufferPool{
		pool: sync.Pool{
			New: func() interface{} {
				return new(bytes.Buffer)
			},
		},
	}
}

// Get returns a buffer from the pool.
// The returned buffer may be newly allocated or reused from a previous Put call.
// The caller should Reset the buffer before use if its previous contents are not needed.
//
// Returns a *bytes.Buffer that should be returned to the pool when no longer needed.
func (bp *BufferPool) Get() *bytes.Buffer {
	return bp.pool.Get().(*bytes.Buffer)
}

// Put returns a buffer to the pool for future reuse.
// The buffer should not be used after calling Put as it may be provided to another goroutine.
//
// Parameters:
//   - b: The buffer to return to the pool
func (bp *BufferPool) Put(b *bytes.Buffer) {
	bp.pool.Put(b)
}

// NewClient creates and validates a new MinIO client.
// It establishes connections to both the standard and core MinIO APIs,
// validates the connection, and ensures the configured bucket exists.
//
// Parameters:
//   - cfg: Configuration for the MinIO client
//   - logger: Logger for recording operations and errors
//
// Returns a configured and validated MinIO client or an error if initialization fails.
//
// Example:
//
//	client, err := minio.NewClient(config, myLogger)
//	if err != nil {
//	    return fmt.Errorf("failed to initialize MinIO client: %w", err)
//	}
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

// monitorConnection periodically checks the MinIO connection and triggers reconnecting if needed.
// This method runs as a goroutine and monitors the health of the MinIO connection,
// triggering reconnection attempts when issues are detected.
//
// Parameters:
//   - ctx: Context for controlling the monitor's lifecycle
func (m *Minio) monitorConnection(ctx context.Context) {
	defer close(m.reconnectSignal)
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

// retryConnection manages reconnection to MinIO when connection issues are detected.
// This method runs as a goroutine and attempts to reestablish the connection
// when the monitor detects issues or when manually triggered.
//
// Parameters:
//   - ctx: Context for controlling the retry loop's lifecycle
func (m *Minio) retryConnection(ctx context.Context) {
outerLoop:
	for {
		select {
		case <-m.shutdownSignal:
			m.logger.Info("Stopping MinIO connection retry loop due to shutdown signal", nil, nil)
			return

		case <-ctx.Done():
			m.logger.Info("Stopping MinIO connection retry loop due to context cancellation", nil, nil)
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
					return

				case <-ctx.Done():
					m.logger.Info("Stopping MinIO connection retry loop during reconnection due to context cancellation", nil, nil)
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

// connectToMinio creates a new standard MinIO client.
// This is an internal helper method used during initial connection and reconnection.
//
// Parameters:
//   - cfg: Configuration for the MinIO connection
//   - logger: Logger for recording operations and errors
//
// Returns a configured MinIO client or an error if the connection fails.
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

// connectToMinioCore creates a new MinIO Core client for low-level operations.
// This is an internal helper method used during initial connection and reconnection.
//
// Parameters:
//   - cfg: Configuration for the MinIO connection
//   - logger: Logger for recording operations and errors
//
// Returns a configured MinIO Core client or an error if the connection fails.
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

// validateConnection performs a simple operation to validate connectivity to MinIO.
// It attempts to list buckets to ensure the connection and credentials are valid.
//
// Parameters:
//   - ctx: Context for controlling the validation operation
//
// Returns nil if the connection is valid, or an error if the validation fails.
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

// ensureBucketExists checks if the configured bucket exists and creates it if necessary.
// This method is called during initialization and reconnection to ensure the
// bucket specified in the configuration is available.
//
// Parameters:
//   - ctx: Context for controlling the bucket check/creation operation
//
// Returns nil if the bucket exists or was successfully created, or an error if the operation fails.
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
