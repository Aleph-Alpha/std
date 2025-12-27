package minio

import (
	"bytes"
	"context"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Aleph-Alpha/std/v1/observability"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// MinioClient represents a MinIO client with additional functionality.
// It wraps the standard MinIO client with features for connection management,
// reconnection handling, and resource monitoring.
type MinioClient struct {
	// client is the standard MinIO client for high-level operations.
	// It is stored in an atomic pointer so it can be swapped during reconnection
	// without racing with concurrent operations.
	client atomic.Pointer[minio.Client]

	// coreClient provides access to low-level operations not available in the standard client.
	// It is stored in an atomic pointer so it can be swapped during reconnection
	// without racing with concurrent operations.
	coreClient atomic.Pointer[minio.Core]

	// cfg holds the configuration for this MinIO client instance
	cfg Config

	// observer provides optional observability hooks for tracking operations
	observer observability.Observer

	// logger provides optional context-aware logging capabilities
	logger Logger

	// shutdownSignal is used to signal the connection monitor to stop
	shutdownSignal chan struct{}

	// reconnectSignal is used to trigger reconnection attempts
	reconnectSignal chan error

	// bufferPool manages reusable byte buffers to reduce memory allocations
	bufferPool *BufferPool

	closeShutdownOnce sync.Once
}

// BufferPoolConfig contains configuration for the buffer pool
type BufferPoolConfig struct {
	// MaxBufferSize is the maximum size a buffer can grow to before being discarded
	MaxBufferSize int
	// MaxPoolSize is the maximum number of buffers to keep in the pool
	MaxPoolSize int
	// InitialBufferSize is the initial size for new buffers
	InitialBufferSize int
}

// DefaultBufferPoolConfig returns the default buffer pool configuration
func DefaultBufferPoolConfig() BufferPoolConfig {
	return BufferPoolConfig{
		MaxBufferSize:     32 * 1024 * 1024, // 32MB max buffer size
		MaxPoolSize:       100,              // Max 100 buffers in pool
		InitialBufferSize: 64 * 1024,        // 64KB initial size
	}
}

// BufferPool implements an advanced pool of bytes.Buffers with size limits and monitoring.
// It prevents memory leaks by limiting buffer sizes and pool capacity.
type BufferPool struct {
	// pool is the underlying sync.Pool that manages the buffer objects
	pool sync.Pool
	// config holds the buffer pool configuration
	config BufferPoolConfig
	// poolSize tracks the current number of buffers in the pool
	poolSize int64
	// totalBuffersCreated tracks total buffers created for monitoring
	totalBuffersCreated int64
	// totalBuffersReused tracks total buffer reuses for monitoring
	totalBuffersReused int64
	// totalBuffersDiscarded tracks buffers discarded due to size limits
	totalBuffersDiscarded int64
	// mu protects pool size modifications
	mu sync.RWMutex
}

// NewBufferPool creates a new BufferPool instance with default configuration.
// The pool will create new bytes.Buffer instances as needed when none are available,
// with built-in size limits to prevent memory leaks.
//
// Returns a configured BufferPool ready for use.
func NewBufferPool() *BufferPool {
	return NewBufferPoolWithConfig(DefaultBufferPoolConfig())
}

// NewBufferPoolWithConfig creates a new BufferPool with custom configuration.
// This allows fine-tuning of buffer sizes and pool limits for specific use cases.
//
// Parameters:
//   - config: Configuration for buffer pool behavior
//
// Returns a configured BufferPool ready for use.
func NewBufferPoolWithConfig(config BufferPoolConfig) *BufferPool {
	bp := &BufferPool{
		config: config,
	}

	bp.pool = sync.Pool{
		New: func() interface{} {
			atomic.AddInt64(&bp.totalBuffersCreated, 1)
			buf := bytes.NewBuffer(make([]byte, 0, bp.config.InitialBufferSize))
			return buf
		},
	}

	return bp
}

// Get returns a buffer from the pool.
// The returned buffer may be newly allocated or reused from a previous Put call.
// The buffer is automatically reset and ready for use.
//
// Returns a *bytes.Buffer that should be returned to the pool when no longer needed.
func (bp *BufferPool) Get() *bytes.Buffer {
	buf := bp.pool.Get().(*bytes.Buffer)
	buf.Reset() // Always reset the buffer for clean state
	atomic.AddInt64(&bp.totalBuffersReused, 1)
	return buf
}

// Put returns a buffer to the pool for future reuse.
// The buffer will be discarded if it exceeds the maximum size limit or if the pool is full.
// This prevents memory leaks from oversized buffers accumulating in the pool.
//
// Parameters:
//   - b: The buffer to return to the pool
func (bp *BufferPool) Put(b *bytes.Buffer) {
	if b == nil {
		return
	}

	// Check if buffer is too large - discard it to prevent memory leaks
	if b.Cap() > bp.config.MaxBufferSize {
		atomic.AddInt64(&bp.totalBuffersDiscarded, 1)
		return
	}

	// Check if pool is at capacity
	bp.mu.RLock()
	currentSize := atomic.LoadInt64(&bp.poolSize)
	bp.mu.RUnlock()

	if currentSize >= int64(bp.config.MaxPoolSize) {
		atomic.AddInt64(&bp.totalBuffersDiscarded, 1)
		return
	}

	// Reset buffer and return to pool
	b.Reset()
	bp.pool.Put(b)
	atomic.AddInt64(&bp.poolSize, 1)
}

// Stats returns statistics about buffer pool usage for monitoring and debugging.
type BufferPoolStats struct {
	// CurrentPoolSize is the current number of buffers in the pool
	CurrentPoolSize int64 `json:"currentPoolSize"`
	// TotalBuffersCreated is the total number of buffers created since start
	TotalBuffersCreated int64 `json:"totalBuffersCreated"`
	// TotalBuffersReused is the total number of buffer reuses
	TotalBuffersReused int64 `json:"totalBuffersReused"`
	// TotalBuffersDiscarded is the total number of buffers discarded due to limits
	TotalBuffersDiscarded int64 `json:"totalBuffersDiscarded"`
	// MaxBufferSize is the maximum allowed buffer size
	MaxBufferSize int `json:"maxBufferSize"`
	// MaxPoolSize is the maximum allowed pool size
	MaxPoolSize int `json:"maxPoolSize"`
	// ReuseRatio is the ratio of reused buffers to created buffers
	ReuseRatio float64 `json:"reuseRatio"`
}

// GetStats returns current buffer pool statistics for monitoring.
// This is useful for understanding memory usage patterns and pool effectiveness.
func (bp *BufferPool) GetStats() BufferPoolStats {
	bp.mu.RLock()
	defer bp.mu.RUnlock()

	created := atomic.LoadInt64(&bp.totalBuffersCreated)
	reused := atomic.LoadInt64(&bp.totalBuffersReused)
	discarded := atomic.LoadInt64(&bp.totalBuffersDiscarded)
	poolSize := atomic.LoadInt64(&bp.poolSize)

	var reuseRatio float64
	if created > 0 {
		reuseRatio = float64(reused) / float64(created)
	}

	return BufferPoolStats{
		CurrentPoolSize:       poolSize,
		TotalBuffersCreated:   created,
		TotalBuffersReused:    reused,
		TotalBuffersDiscarded: discarded,
		MaxBufferSize:         bp.config.MaxBufferSize,
		MaxPoolSize:           bp.config.MaxPoolSize,
		ReuseRatio:            reuseRatio,
	}
}

// Cleanup forces cleanup of the buffer pool, releasing all buffers.
// This is useful during shutdown or when memory pressure is high.
func (bp *BufferPool) Cleanup() {
	bp.mu.Lock()
	defer bp.mu.Unlock()

	// Create a new pool to replace the old one
	bp.pool = sync.Pool{
		New: func() interface{} {
			atomic.AddInt64(&bp.totalBuffersCreated, 1)
			buf := bytes.NewBuffer(make([]byte, 0, bp.config.InitialBufferSize))
			return buf
		},
	}

	atomic.StoreInt64(&bp.poolSize, 0)

	// Force garbage collection to free the discarded buffers
	runtime.GC()
}

// ConnectionPoolConfig contains configuration for connection management
type ConnectionPoolConfig struct {
	// MaxIdleConnections is the maximum number of idle connections to maintain
	MaxIdleConnections int
	// MaxConnectionsPerHost is the maximum connections per host
	MaxConnectionsPerHost int
	// IdleConnectionTimeout is how long to keep idle connections
	IdleConnectionTimeout time.Duration
	// ConnectionTimeout is the timeout for establishing connections
	ConnectionTimeout time.Duration
}

// DefaultConnectionPoolConfig returns default connection pool configuration
func DefaultConnectionPoolConfig() ConnectionPoolConfig {
	return ConnectionPoolConfig{
		MaxIdleConnections:    50,
		MaxConnectionsPerHost: 100,
		IdleConnectionTimeout: 90 * time.Second,
		ConnectionTimeout:     30 * time.Second,
	}
}

// NewClient creates and validates a new MinIO client.
// It establishes connections to both the standard and core MinIO APIs,
// validates the connection, and ensures the configured bucket exists.
//
// Parameters:
//   - config: Configuration for the MinIO client
//
// Returns a configured and validated MinioClient or an error if initialization fails.
//
// Example:
//
//	client, err := minio.NewClient(config)
//	if err != nil {
//	    return fmt.Errorf("failed to initialize MinIO client: %w", err)
//	}
//
//	// Optionally attach logger and observer
//	client = client.
//	    WithLogger(myLogger).
//	    WithObserver(myObserver)
//
//	defer client.GracefulShutdown()
func NewClient(config Config) (*MinioClient, error) {
	// Create the standard client
	client, err := connectToMinio(config)
	if err != nil {
		return nil, err
	}

	// Create the core client using the same connection parameters
	coreClient, err := connectToMinioCore(config)
	if err != nil {
		return nil, err
	}

	// Create buffer pool
	bufferPool := NewBufferPool()

	minioClient := &MinioClient{
		cfg:             config,
		observer:        nil, // No observer by default
		logger:          nil, // No logger by default
		shutdownSignal:  make(chan struct{}),
		reconnectSignal: make(chan error, 1),
		bufferPool:      bufferPool,
	}
	minioClient.client.Store(client)
	minioClient.coreClient.Store(coreClient)

	timeoutCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := minioClient.validateConnection(timeoutCtx); err != nil {
		return nil, err
	}
	if err := minioClient.ensureBucketExists(timeoutCtx); err != nil {
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
func (m *MinioClient) monitorConnection(ctx context.Context) {
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
				m.logError(ctx, "MinIO connection health check failed", map[string]interface{}{
					"endpoint": m.cfg.Connection.Endpoint,
					"error":    err.Error(),
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
func (m *MinioClient) retryConnection(ctx context.Context) {
outerLoop:
	for {
		select {
		case <-m.shutdownSignal:
			m.logInfo(ctx, "Stopping MinIO connection retry loop due to shutdown signal", nil)
			return

		case <-ctx.Done():
			m.logInfo(ctx, "Stopping MinIO connection retry loop due to context cancellation", nil)
			return

		case err, ok := <-m.reconnectSignal:
			if !ok {
				return
			}
			m.logWarn(ctx, "MinIO connection issue detected, attempting reconnection", map[string]interface{}{
				"endpoint": m.cfg.Connection.Endpoint,
				"error":    err.Error(),
			})

		reconnectLoop:
			for {
				select {
				case <-m.shutdownSignal:
					m.logInfo(ctx, "Stopping MinIO connection retry loop during reconnection due to shutdown signal", nil)
					return

				case <-ctx.Done():
					m.logInfo(ctx, "Stopping MinIO connection retry loop during reconnection due to context cancellation", nil)
					return

				default:
					// Create a context with timeout for the reconnection attempt
					ctxReconnect, cancel := context.WithTimeout(context.Background(), 10*time.Second)

					// Attempt to create new clients
					newClient, err := connectToMinio(m.cfg)
					if err != nil {
						cancel() // Cancel the context to free resources
						m.logError(ctx, "MinIO reconnection failed", map[string]interface{}{
							"endpoint":      m.cfg.Connection.Endpoint,
							"will_retry_in": "1s",
							"error":         err.Error(),
						})
						time.Sleep(time.Second)
						continue reconnectLoop
					}

					newCoreClient, err := connectToMinioCore(m.cfg)
					if err != nil {
						cancel() // Cancel the context to free resources
						m.logError(ctx, "MinIO core client reconnection failed", map[string]interface{}{
							"endpoint":      m.cfg.Connection.Endpoint,
							"will_retry_in": "1s",
							"error":         err.Error(),
						})
						time.Sleep(time.Second)
						continue reconnectLoop
					}

					// Validate the new connection before swapping pointers.
					// Prefer bucket-scoped validation to avoid requiring ListAllMyBuckets permissions.
					if bucket := m.cfg.Connection.BucketName; bucket != "" {
						_, err = newClient.BucketExists(ctxReconnect, bucket)
					} else {
						_, err = newClient.ListBuckets(ctxReconnect)
					}
					if err != nil {
						cancel() // Cancel the context to free resources
						m.logError(ctx, "MinIO connection validation failed", map[string]interface{}{
							"error": err.Error(),
						})
						time.Sleep(time.Second)
						continue reconnectLoop
					}

					// Update the client references
					oldClient := m.client.Load()
					oldCoreClient := m.coreClient.Load()
					m.client.Store(newClient)
					m.coreClient.Store(newCoreClient)

					// Verify bucket existence after reconnection
					err = m.ensureBucketExists(ctxReconnect)
					cancel() // Cancel the context to free resources

					if err != nil {
						// Revert to the previous clients to avoid leaving the instance in a broken state.
						m.client.Store(oldClient)
						m.coreClient.Store(oldCoreClient)
						m.logError(ctx, "Failed to verify bucket after reconnection", map[string]interface{}{
							"error": err.Error(),
						})
						time.Sleep(time.Second)
						continue reconnectLoop
					}

					m.logInfo(ctx, "Successfully reconnected to MinIO", map[string]interface{}{
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
//
// Returns a configured MinIO client or an error if the connection fails.
func connectToMinio(cfg Config) (*minio.Client, error) {
	// Add validation for an empty endpoint
	if cfg.Connection.Endpoint == "" {
		return nil, fmt.Errorf("minio endpoint cannot be empty")
	}

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
//
// Returns a configured MinIO Core client or an error if the connection fails.
func connectToMinioCore(cfg Config) (*minio.Core, error) {
	// Add validation for an empty endpoint
	if cfg.Connection.Endpoint == "" {
		return nil, fmt.Errorf("minio endpoint cannot be empty")
	}

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
func (m *MinioClient) validateConnection(ctx context.Context) error {
	// Set a timeout for validation
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	c := m.client.Load()
	if c == nil {
		return ErrConnectionFailed
	}

	// Prefer bucket-scoped validation so credentials do not need ListAllMyBuckets.
	if bucket := m.cfg.Connection.BucketName; bucket != "" {
		_, err := c.BucketExists(ctx, bucket)
		return err
	}

	// Fallback: if no bucket configured, validate by listing buckets.
	_, err := c.ListBuckets(ctx)
	return err
}

// ensureBucketExists checks if the configured bucket exists and creates it if necessary.
// This method is called during initialization and reconnection to ensure the
// bucket specified in the configuration is available.
//
// Parameters:
//   - ctx: Context for controlling the bucket check/creation operation
//
// Returns nil if the bucket exists or was successfully created, or an error if the operation fails.
func (m *MinioClient) ensureBucketExists(ctx context.Context) error {
	bucketName := m.cfg.Connection.BucketName
	if bucketName == "" {
		return fmt.Errorf("bucket name is empty")
	}

	// Set a timeout for the operation
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	c := m.client.Load()
	if c == nil {
		return ErrConnectionFailed
	}

	exists, err := c.BucketExists(ctx, bucketName)
	if err != nil {
		return fmt.Errorf("failed to check if bucket exists, bucket: %v, err: %w", bucketName, err)
	}

	if !exists && m.cfg.Connection.AccessBucketCreation {
		m.logInfo(ctx, "Bucket does not exist, creating it", map[string]interface{}{
			"bucket": bucketName,
			"region": m.cfg.Connection.Region,
		})

		err = c.MakeBucket(ctx, bucketName, minio.MakeBucketOptions{
			Region: m.cfg.Connection.Region,
		})

		if err != nil {
			return err
		}

		m.logInfo(ctx, "Successfully created bucket", map[string]interface{}{
			"bucket": bucketName,
		})
	} else if !exists {
		return fmt.Errorf("bucket does not exist, please create it manually")
	}

	return nil
}

// GetBufferPoolStats returns buffer pool statistics for monitoring buffer efficiency.
// This is useful for understanding memory usage patterns and optimizing buffer sizes.
//
// Returns:
//   - BufferPoolStats: Statistics about buffer pool usage and efficiency
//
// Example:
//
//	stats := minioClient.GetBufferPoolStats()
//	fmt.Printf("Buffer reuse ratio: %.2f%%\n", stats.ReuseRatio*100)
//	fmt.Printf("Buffers in pool: %d\n", stats.CurrentPoolSize)
func (m *MinioClient) GetBufferPoolStats() BufferPoolStats {
	if m.bufferPool == nil {
		return BufferPoolStats{}
	}
	return m.bufferPool.GetStats()
}

// CleanupResources performs cleanup of buffer pools and forces garbage collection.
// This method is useful during shutdown or when memory pressure is high.
//
// Example:
//
//	// Clean up resources during shutdown
//	defer minioClient.CleanupResources()
func (m *MinioClient) CleanupResources() {
	if m.bufferPool != nil {
		m.bufferPool.Cleanup()
	}

	// Force garbage collection to free memory
	runtime.GC()
}

// WithObserver attaches an observer to the MinIO client for observability hooks.
// This method uses the builder pattern and returns the client for method chaining.
//
// The observer will be notified of all storage operations, allowing
// external systems to track metrics, traces, or other observability data.
//
// This is useful for non-FX usage where you want to attach an observer after
// creating the client. When using FX, the observer is automatically injected via NewClientWithDI.
//
// Example:
//
//	client, err := minio.NewClient(config)
//	if err != nil {
//	    return err
//	}
//	client = client.WithObserver(myObserver)
//	defer client.GracefulShutdown()
func (m *MinioClient) WithObserver(observer observability.Observer) *MinioClient {
	m.observer = observer
	return m
}

// WithLogger attaches a logger to the MinIO client for internal logging.
// This method uses the builder pattern and returns the client for method chaining.
//
// The logger will be used for lifecycle events, connection monitoring, and error logging.
// This is particularly useful for debugging and monitoring connection health.
//
// This is useful for non-FX usage where you want to enable logging after
// creating the client. When using FX, the logger is automatically injected via NewClientWithDI.
//
// Example:
//
//	client, err := minio.NewClient(config)
//	if err != nil {
//	    return err
//	}
//	client = client.WithLogger(myLogger)
//	defer client.GracefulShutdown()
func (m *MinioClient) WithLogger(logger Logger) *MinioClient {
	m.logger = logger
	return m
}

// logInfo logs an informational message using the configured logger if available.
// This is used for lifecycle and background operation logging.
func (m *MinioClient) logInfo(ctx context.Context, msg string, fields map[string]interface{}) {
	if m.logger != nil {
		m.logger.InfoWithContext(ctx, msg, nil, fields)
	}
	// Silently skip if no logger configured
}

// logWarn logs a warning message using the configured logger if available.
// This is used for non-critical issues during connection monitoring or background operations.
func (m *MinioClient) logWarn(ctx context.Context, msg string, fields map[string]interface{}) {
	if m.logger != nil {
		m.logger.WarnWithContext(ctx, msg, nil, fields)
	}
	// Silently skip if no logger configured
}

// logError logs an error message using the configured logger if available.
// This is only used for errors in background goroutines that can't be returned to the caller.
func (m *MinioClient) logError(ctx context.Context, msg string, fields map[string]interface{}) {
	if m.logger != nil {
		m.logger.ErrorWithContext(ctx, msg, nil, fields)
	}
	// Silently skip if no logger configured
}
