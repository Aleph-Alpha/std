package minio

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// MinioLogger defines the logging interface used by MinIO client.
// This interface allows for flexible logger injection while maintaining compatibility
// with both structured loggers (like the logger package) and simple loggers.
type MinioLogger interface {
	// Debug logs debug-level messages
	Debug(msg string, err error, fields ...map[string]any)
	// Info logs informational messages
	Info(msg string, err error, fields ...map[string]any)
	// Warn logs warning messages
	Warn(msg string, err error, fields ...map[string]any)
	// Error logs error messages
	Error(msg string, err error, fields ...map[string]any)
	// Fatal logs fatal messages and should terminate the application
	Fatal(msg string, err error, fields ...map[string]any)
}

// LoggerAdapter adapts the logger package's Logger to implement MinioLogger interface.
// This allows seamless integration with the structured logger from the logger package.
type LoggerAdapter struct {
	logger interface {
		Debug(msg string, err error, fields ...map[string]interface{})
		Info(msg string, err error, fields ...map[string]interface{})
		Warn(msg string, err error, fields ...map[string]interface{})
		Error(msg string, err error, fields ...map[string]interface{})
		Fatal(msg string, err error, fields ...map[string]interface{})
	}
}

// NewLoggerAdapter creates a new LoggerAdapter that wraps the logger package's Logger.
// This function provides a bridge between the logger package and MinIO's logging interface.
func NewLoggerAdapter(logger interface {
	Debug(msg string, err error, fields ...map[string]interface{})
	Info(msg string, err error, fields ...map[string]interface{})
	Warn(msg string, err error, fields ...map[string]interface{})
	Error(msg string, err error, fields ...map[string]interface{})
	Fatal(msg string, err error, fields ...map[string]interface{})
}) MinioLogger {
	return &LoggerAdapter{logger: logger}
}

// Debug implements MinioLogger interface by delegating to the wrapped logger
func (la *LoggerAdapter) Debug(msg string, err error, fields ...map[string]any) {
	convertedFields := convertFieldMaps(fields...)
	la.logger.Debug(msg, err, convertedFields...)
}

// Info implements MinioLogger interface by delegating to the wrapped logger
func (la *LoggerAdapter) Info(msg string, err error, fields ...map[string]any) {
	convertedFields := convertFieldMaps(fields...)
	la.logger.Info(msg, err, convertedFields...)
}

// Warn implements MinioLogger interface by delegating to the wrapped logger
func (la *LoggerAdapter) Warn(msg string, err error, fields ...map[string]any) {
	convertedFields := convertFieldMaps(fields...)
	la.logger.Warn(msg, err, convertedFields...)
}

// Error implements MinioLogger interface by delegating to the wrapped logger
func (la *LoggerAdapter) Error(msg string, err error, fields ...map[string]any) {
	convertedFields := convertFieldMaps(fields...)
	la.logger.Error(msg, err, convertedFields...)
}

// Fatal implements MinioLogger interface by delegating to the wrapped logger
func (la *LoggerAdapter) Fatal(msg string, err error, fields ...map[string]any) {
	convertedFields := convertFieldMaps(fields...)
	la.logger.Fatal(msg, err, convertedFields...)
}

// convertFieldMaps converts []map[string]any to []map[string]interface{} for compatibility
func convertFieldMaps(fields ...map[string]any) []map[string]interface{} {
	converted := make([]map[string]interface{}, len(fields))
	for i, field := range fields {
		convertedField := make(map[string]interface{})
		for k, v := range field {
			convertedField[k] = v
		}
		converted[i] = convertedField
	}
	return converted
}

// fallbackLogger implements MinioLogger using Go's standard log package.
// This provides a fallback when no structured logger is available.
type fallbackLogger struct {
	stdLogger *log.Logger
}

// newFallbackLogger creates a new fallback logger using Go's standard log package.
func newFallbackLogger() MinioLogger {
	return &fallbackLogger{
		stdLogger: log.New(os.Stdout, "[MINIO] ", log.LstdFlags|log.Lshortfile),
	}
}

// Debug implements MinioLogger interface
func (f *fallbackLogger) Debug(msg string, err error, fields ...map[string]any) {
	f.logWithLevel("DEBUG", msg, err, fields...)
}

// Info implements MinioLogger interface
func (f *fallbackLogger) Info(msg string, err error, fields ...map[string]any) {
	f.logWithLevel("INFO", msg, err, fields...)
}

// Warn implements MinioLogger interface
func (f *fallbackLogger) Warn(msg string, err error, fields ...map[string]any) {
	f.logWithLevel("WARN", msg, err, fields...)
}

// Error implements MinioLogger interface
func (f *fallbackLogger) Error(msg string, err error, fields ...map[string]any) {
	f.logWithLevel("ERROR", msg, err, fields...)
}

// Fatal implements MinioLogger interface
func (f *fallbackLogger) Fatal(msg string, err error, fields ...map[string]any) {
	f.logWithLevel("FATAL", msg, err, fields...)
	os.Exit(1)
}

// logWithLevel is a helper method to format and log messages with level, error, and fields
func (f *fallbackLogger) logWithLevel(level, msg string, err error, fields ...map[string]any) {
	logMsg := fmt.Sprintf("%s: %s", level, msg)

	if err != nil {
		logMsg += fmt.Sprintf(" | error: %v", err)
	}

	// Add structured fields if provided
	for _, fieldMap := range fields {
		for key, value := range fieldMap {
			logMsg += fmt.Sprintf(" | %s: %v", key, value)
		}
	}

	f.stdLogger.Print(logMsg)
}

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

	// logger provides structured logging capabilities
	logger MinioLogger

	// shutdownSignal is used to signal the connection monitor to stop
	shutdownSignal chan struct{}

	// reconnectSignal is used to trigger reconnection attempts
	reconnectSignal chan error

	// bufferPool manages reusable byte buffers to reduce memory allocations
	bufferPool *BufferPool

	// resourceMonitor tracks resource usage and performance metrics
	resourceMonitor *ResourceMonitor

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

// ResourceMonitor tracks resource usage, performance metrics, and connection health
type ResourceMonitor struct {
	// Connection metrics
	totalConnections   int64
	activeConnections  int64
	failedConnections  int64
	connectionAttempts int64

	// Request metrics
	totalRequests      int64
	successfulRequests int64
	failedRequests     int64

	// Performance metrics
	totalRequestDuration int64 // in nanoseconds
	maxRequestDuration   int64 // in nanoseconds
	minRequestDuration   int64 // in nanoseconds

	// Memory metrics
	totalMemoryAllocated int64
	currentMemoryUsage   int64
	maxMemoryUsage       int64

	// Buffer pool metrics (reference to buffer pool stats)
	bufferPool *BufferPool

	// Timestamps
	startTime time.Time
	lastReset time.Time

	// Mutex for thread-safe operations
	mu sync.RWMutex
}

// NewResourceMonitor creates a new resource monitor instance
func NewResourceMonitor(bufferPool *BufferPool) *ResourceMonitor {
	now := time.Now()
	return &ResourceMonitor{
		bufferPool:         bufferPool,
		startTime:          now,
		lastReset:          now,
		minRequestDuration: int64(^uint64(0) >> 1), // Max int64 value initially
	}
}

// RecordConnectionAttempt records a connection attempt
func (rm *ResourceMonitor) RecordConnectionAttempt() {
	atomic.AddInt64(&rm.connectionAttempts, 1)
}

// RecordConnectionSuccess records a successful connection
func (rm *ResourceMonitor) RecordConnectionSuccess() {
	atomic.AddInt64(&rm.totalConnections, 1)
	atomic.AddInt64(&rm.activeConnections, 1)
}

// RecordConnectionFailure records a failed connection
func (rm *ResourceMonitor) RecordConnectionFailure() {
	atomic.AddInt64(&rm.failedConnections, 1)
}

// RecordConnectionClosure records a connection closure
func (rm *ResourceMonitor) RecordConnectionClosure() {
	atomic.AddInt64(&rm.activeConnections, -1)
}

// RecordRequest records the start of a request and returns a function to record completion
func (rm *ResourceMonitor) RecordRequest() func(success bool) {
	atomic.AddInt64(&rm.totalRequests, 1)
	startTime := time.Now()

	return func(success bool) {
		duration := time.Since(startTime).Nanoseconds()
		atomic.AddInt64(&rm.totalRequestDuration, duration)

		// Update min/max duration atomically
		for {
			current := atomic.LoadInt64(&rm.maxRequestDuration)
			if duration <= current || atomic.CompareAndSwapInt64(&rm.maxRequestDuration, current, duration) {
				break
			}
		}

		for {
			current := atomic.LoadInt64(&rm.minRequestDuration)
			if duration >= current || atomic.CompareAndSwapInt64(&rm.minRequestDuration, current, duration) {
				break
			}
		}

		if success {
			atomic.AddInt64(&rm.successfulRequests, 1)
		} else {
			atomic.AddInt64(&rm.failedRequests, 1)
		}
	}
}

// RecordMemoryUsage records current memory usage
func (rm *ResourceMonitor) RecordMemoryUsage(bytes int64) {
	atomic.AddInt64(&rm.totalMemoryAllocated, bytes)
	atomic.StoreInt64(&rm.currentMemoryUsage, bytes)

	// Update max memory usage
	for {
		current := atomic.LoadInt64(&rm.maxMemoryUsage)
		if bytes <= current || atomic.CompareAndSwapInt64(&rm.maxMemoryUsage, current, bytes) {
			break
		}
	}
}

// ResourceStats contains comprehensive resource usage statistics
type ResourceStats struct {
	// Connection statistics
	TotalConnections      int64   `json:"totalConnections"`
	ActiveConnections     int64   `json:"activeConnections"`
	FailedConnections     int64   `json:"failedConnections"`
	ConnectionAttempts    int64   `json:"connectionAttempts"`
	ConnectionSuccessRate float64 `json:"connectionSuccessRate"`

	// Request statistics
	TotalRequests      int64   `json:"totalRequests"`
	SuccessfulRequests int64   `json:"successfulRequests"`
	FailedRequests     int64   `json:"failedRequests"`
	RequestSuccessRate float64 `json:"requestSuccessRate"`

	// Performance statistics
	AverageRequestDuration time.Duration `json:"averageRequestDuration"`
	MaxRequestDuration     time.Duration `json:"maxRequestDuration"`
	MinRequestDuration     time.Duration `json:"minRequestDuration"`
	RequestsPerSecond      float64       `json:"requestsPerSecond"`

	// Memory statistics
	TotalMemoryAllocated int64 `json:"totalMemoryAllocated"`
	CurrentMemoryUsage   int64 `json:"currentMemoryUsage"`
	MaxMemoryUsage       int64 `json:"maxMemoryUsage"`

	// Buffer pool statistics
	BufferPoolStats BufferPoolStats `json:"bufferPoolStats"`

	// Runtime information
	Uptime        time.Duration `json:"uptime"`
	LastResetTime time.Time     `json:"lastResetTime"`

	// System memory info
	SystemMemoryStats runtime.MemStats `json:"systemMemoryStats"`
}

// GetStats returns comprehensive resource usage statistics
func (rm *ResourceMonitor) GetStats() ResourceStats {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	now := time.Now()
	uptime := now.Sub(rm.startTime)

	totalConns := atomic.LoadInt64(&rm.totalConnections)
	connAttempts := atomic.LoadInt64(&rm.connectionAttempts)
	totalReqs := atomic.LoadInt64(&rm.totalRequests)
	successReqs := atomic.LoadInt64(&rm.successfulRequests)
	totalDuration := atomic.LoadInt64(&rm.totalRequestDuration)

	var connSuccessRate, reqSuccessRate, avgDuration, reqPerSec float64

	if connAttempts > 0 {
		connSuccessRate = float64(totalConns) / float64(connAttempts)
	}

	if totalReqs > 0 {
		reqSuccessRate = float64(successReqs) / float64(totalReqs)
		avgDuration = float64(totalDuration) / float64(totalReqs)
		reqPerSec = float64(totalReqs) / uptime.Seconds()
	}

	// Get system memory stats
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	// Get buffer pool stats
	var bufferStats BufferPoolStats
	if rm.bufferPool != nil {
		bufferStats = rm.bufferPool.GetStats()
	}

	return ResourceStats{
		TotalConnections:      totalConns,
		ActiveConnections:     atomic.LoadInt64(&rm.activeConnections),
		FailedConnections:     atomic.LoadInt64(&rm.failedConnections),
		ConnectionAttempts:    connAttempts,
		ConnectionSuccessRate: connSuccessRate,

		TotalRequests:      totalReqs,
		SuccessfulRequests: successReqs,
		FailedRequests:     atomic.LoadInt64(&rm.failedRequests),
		RequestSuccessRate: reqSuccessRate,

		AverageRequestDuration: time.Duration(avgDuration),
		MaxRequestDuration:     time.Duration(atomic.LoadInt64(&rm.maxRequestDuration)),
		MinRequestDuration:     time.Duration(atomic.LoadInt64(&rm.minRequestDuration)),
		RequestsPerSecond:      reqPerSec,

		TotalMemoryAllocated: atomic.LoadInt64(&rm.totalMemoryAllocated),
		CurrentMemoryUsage:   atomic.LoadInt64(&rm.currentMemoryUsage),
		MaxMemoryUsage:       atomic.LoadInt64(&rm.maxMemoryUsage),

		BufferPoolStats: bufferStats,

		Uptime:        uptime,
		LastResetTime: rm.lastReset,

		SystemMemoryStats: memStats,
	}
}

// ResetStats resets all statistics counters
func (rm *ResourceMonitor) ResetStats() {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	atomic.StoreInt64(&rm.totalConnections, 0)
	atomic.StoreInt64(&rm.activeConnections, 0)
	atomic.StoreInt64(&rm.failedConnections, 0)
	atomic.StoreInt64(&rm.connectionAttempts, 0)

	atomic.StoreInt64(&rm.totalRequests, 0)
	atomic.StoreInt64(&rm.successfulRequests, 0)
	atomic.StoreInt64(&rm.failedRequests, 0)

	atomic.StoreInt64(&rm.totalRequestDuration, 0)
	atomic.StoreInt64(&rm.maxRequestDuration, 0)
	atomic.StoreInt64(&rm.minRequestDuration, int64(^uint64(0)>>1))

	atomic.StoreInt64(&rm.totalMemoryAllocated, 0)
	atomic.StoreInt64(&rm.currentMemoryUsage, 0)
	atomic.StoreInt64(&rm.maxMemoryUsage, 0)

	rm.lastReset = time.Now()
}

// NewClient creates and validates a new MinIO client.
// It establishes connections to both the standard and core MinIO APIs,
// validates the connection, and ensures the configured bucket exists.
//
// Parameters:
//   - config: Configuration for the MinIO client
//   - logger: Optional logger for recording operations and errors. If nil, a fallback logger will be used.
//
// Returns a configured and validated MinioClient or an error if initialization fails.
//
// Example:
//
//	// With custom logger
//	client, err := minio.NewClient(config, myLogger)
//	if err != nil {
//	    return fmt.Errorf("failed to initialize MinIO client: %w", err)
//	}
//
//	// Without logger (uses fallback)
//	client, err := minio.NewClient(config, nil)
//	if err != nil {
//	    return fmt.Errorf("failed to initialize MinIO client: %w", err)
//	}
func NewClient(config Config, logger MinioLogger) (*MinioClient, error) {
	// Use fallback logger if none provided
	if logger == nil {
		logger = newFallbackLogger()
	}

	// Create the standard client
	client, err := connectToMinio(config, logger)
	if err != nil {
		logger.Error("failed to connect to minio", err, map[string]any{
			"endpoint": config.Connection.Endpoint,
			"region":   config.Connection.Region,
			"secure":   config.Connection.UseSSL,
			"bucket":   config.Connection.BucketName,
		})
		return nil, err
	}

	// Create the core client using the same connection parameters
	coreClient, err := connectToMinioCore(config, logger)
	if err != nil {
		logger.Error("failed to create core minio client", err, map[string]any{
			"endpoint": config.Connection.Endpoint,
			"region":   config.Connection.Region,
			"secure":   config.Connection.UseSSL,
		})
		return nil, err
	}

	// Create buffer pool and resource monitor
	bufferPool := NewBufferPool()
	resourceMonitor := NewResourceMonitor(bufferPool)

	minioClient := &MinioClient{
		cfg:             config,
		logger:          logger,
		shutdownSignal:  make(chan struct{}),
		reconnectSignal: make(chan error, 1),
		bufferPool:      bufferPool,
		resourceMonitor: resourceMonitor,
	}
	minioClient.client.Store(client)
	minioClient.coreClient.Store(coreClient)

	timeoutCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := minioClient.validateConnection(timeoutCtx); err != nil {
		logger.Error("failed to validate minio connection", err, map[string]any{
			"endpoint": config.Connection.Endpoint,
			"region":   config.Connection.Region,
			"secure":   config.Connection.UseSSL,
			"bucket":   config.Connection.BucketName,
		})
		return nil, err
	}
	if err := minioClient.ensureBucketExists(timeoutCtx); err != nil {
		logger.Error("failed to verify bucket", err, map[string]any{
			"endpoint": config.Connection.Endpoint,
			"region":   config.Connection.Region,
			"secure":   config.Connection.UseSSL,
			"bucket":   config.Connection.BucketName,
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
				m.logger.Error("MinIO connection health check failed", err, map[string]any{
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
func (m *MinioClient) retryConnection(ctx context.Context) {
outerLoop:
	for {
		select {
		case <-m.shutdownSignal:
			m.logger.Info("Stopping MinIO connection retry loop due to shutdown signal", nil, nil)
			return

		case <-ctx.Done():
			m.logger.Info("Stopping MinIO connection retry loop due to context cancellation", nil, nil)
			return

		case err, ok := <-m.reconnectSignal:
			if !ok {
				return
			}
			m.logger.Warn("MinIO connection issue detected, attempting reconnection", err, map[string]any{
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
						m.logger.Error("MinIO reconnection failed", err, map[string]any{
							"endpoint":      m.cfg.Connection.Endpoint,
							"will_retry_in": "1s",
						})
						time.Sleep(time.Second)
						continue reconnectLoop
					}

					newCoreClient, err := connectToMinioCore(m.cfg, m.logger)
					if err != nil {
						cancel() // Cancel the context to free resources
						m.logger.Error("MinIO core client reconnection failed", err, map[string]any{
							"endpoint":      m.cfg.Connection.Endpoint,
							"will_retry_in": "1s",
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
						m.logger.Error("MinIO connection validation failed", err, nil)
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
						m.logger.Error("Failed to verify bucket after reconnection", err, nil)
						time.Sleep(time.Second)
						continue reconnectLoop
					}

					m.logger.Info("Successfully reconnected to MinIO", nil, map[string]any{
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
func connectToMinio(cfg Config, logger MinioLogger) (*minio.Client, error) {
	// Add validation for an empty endpoint
	if cfg.Connection.Endpoint == "" {
		return nil, fmt.Errorf("minio endpoint cannot be empty")
	}

	logger.Info("Connecting to MinIO", nil, map[string]any{
		"endpoint": cfg.Connection.Endpoint,
		"region":   cfg.Connection.Region,
		"secure":   cfg.Connection.UseSSL,
	})

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
func connectToMinioCore(cfg Config, logger MinioLogger) (*minio.Core, error) {
	// Add validation for an empty endpoint
	if cfg.Connection.Endpoint == "" {
		return nil, fmt.Errorf("minio endpoint cannot be empty")
	}

	logger.Info("Creating MinIO Core client", nil, map[string]any{
		"endpoint": cfg.Connection.Endpoint,
		"region":   cfg.Connection.Region,
		"secure":   cfg.Connection.UseSSL,
		"type":     "core client",
	})

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
		m.logger.Info("Bucket does not exist, creating it", nil, map[string]any{
			"bucket": bucketName,
			"region": m.cfg.Connection.Region,
		})

		err = c.MakeBucket(ctx, bucketName, minio.MakeBucketOptions{
			Region: m.cfg.Connection.Region,
		})

		if err != nil {
			return err
		}

		m.logger.Info("Successfully created bucket", nil, map[string]any{
			"bucket": bucketName,
		})
	} else if !exists {
		return fmt.Errorf("bucket does not exist, please create it manually")
	}

	return nil
}

// GetResourceStats returns comprehensive resource usage statistics for monitoring.
// This method provides insights into connection health, request performance, memory usage,
// and buffer pool efficiency.
//
// Returns:
//   - ResourceStats: Comprehensive statistics about resource usage
//
// Example:
//
//	stats := minioClient.GetResourceStats()
//	fmt.Printf("Active connections: %d\n", stats.ActiveConnections)
//	fmt.Printf("Request success rate: %.2f%%\n", stats.RequestSuccessRate*100)
//	fmt.Printf("Average request duration: %v\n", stats.AverageRequestDuration)
func (m *MinioClient) GetResourceStats() ResourceStats {
	if m.resourceMonitor == nil {
		return ResourceStats{}
	}
	return m.resourceMonitor.GetStats()
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

// ResetResourceStats resets all resource monitoring statistics.
// This is useful for getting fresh metrics for a specific time period.
//
// Example:
//
//	// Reset stats at the beginning of a monitoring period
//	minioClient.ResetResourceStats()
//	// ... perform operations ...
//	stats := minioClient.GetResourceStats()
func (m *MinioClient) ResetResourceStats() {
	if m.resourceMonitor != nil {
		m.resourceMonitor.ResetStats()
	}
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
