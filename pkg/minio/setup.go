package minio

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os"
	"sync"
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

	// logger provides structured logging capabilities
	logger MinioLogger

	// mu provides thread-safety for client operations
	mu sync.RWMutex

	// shutdownSignal is used to signal the connection monitor to stop
	shutdownSignal chan struct{}

	// reconnectSignal is used to trigger reconnection attempts
	reconnectSignal chan error

	// bufferPool manages reusable byte buffers to reduce memory allocations
	bufferPool *BufferPool

	closeShutdownOnce  sync.Once
	closeReconnectOnce sync.Once
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
//   - config: Configuration for the MinIO client
//   - logger: Optional logger for recording operations and errors. If nil, a fallback logger will be used.
//
// Returns a configured and validated MinIO client or an error if initialization fails.
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
func NewClient(config Config, logger MinioLogger) (*Minio, error) {
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

	minioClient := &Minio{
		Client:          client,
		CoreClient:      coreClient,
		cfg:             config,
		logger:          logger,
		shutdownSignal:  make(chan struct{}),
		reconnectSignal: make(chan error),
		bufferPool:      NewBufferPool(),
	}

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
func (m *Minio) monitorConnection(ctx context.Context) {
	defer m.closeReconnectOnce.Do(func() {
		close(m.reconnectSignal)
	})
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
func (m *Minio) retryConnection(ctx context.Context) {
	defer m.closeShutdownOnce.Do(func() {
		close(m.shutdownSignal)
	})
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

	if !exists && m.cfg.Connection.AccessBucketCreation {
		m.logger.Info("Bucket does not exist, creating it", nil, map[string]any{
			"bucket": bucketName,
			"region": m.cfg.Connection.Region,
		})

		err = m.Client.MakeBucket(ctx, bucketName, minio.MakeBucketOptions{
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
