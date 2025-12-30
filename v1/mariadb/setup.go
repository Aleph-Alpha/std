package mariadb

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Aleph-Alpha/std/v1/observability"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// MariaDB is a wrapper around gorm.DB that provides connection monitoring,
// automatic reconnection, and standardized database operations for MariaDB/MySQL.
//
// Implements both mariadb.Client (deprecated) and database.Client interfaces.
//
// Concurrency: the active `*gorm.DB` pointer is stored in an atomic pointer and can be
// swapped during reconnection without blocking readers.
type MariaDB struct {
	cfg             Config
	client          atomic.Pointer[gorm.DB]
	observer        observability.Observer
	logger          Logger
	shutdownSignal  chan struct{}
	retryChanSignal chan error

	closeRetryChanOnce sync.Once
	closeShutdownOnce  sync.Once
}

// NewMariaDB creates a new MariaDB instance with the provided configuration.
// It establishes the initial database connection and sets up the internal state
// for connection monitoring and recovery. If the initial connection fails,
// it returns an error.
//
// Returns *MariaDB concrete type (following Go best practice: "accept interfaces, return structs").
func NewMariaDB(cfg Config) (*MariaDB, error) {
	conn, err := connectToMariaDB(cfg)
	if err != nil {
		return nil, fmt.Errorf("error in connecting to MariaDB after all retries: %w", err)
	}

	db := &MariaDB{
		cfg:             cfg,
		observer:        nil, // No observer by default
		logger:          nil, // No logger by default
		shutdownSignal:  make(chan struct{}),
		retryChanSignal: make(chan error, 1),
	}
	db.client.Store(conn)
	return db, nil
}

// connectToMariaDB establishes a connection to the MariaDB/MySQL database using the provided
// configuration. It sets up the connection DSN, opens the connection with GORM,
// and configures the connection pool with appropriate parameters for performance.
// Returns the initialized GORM DB instance or an error if the connection fails.
func connectToMariaDB(mariadbConfig Config) (*gorm.DB, error) {
	// Set defaults
	charset := mariadbConfig.Connection.Charset
	if charset == "" {
		charset = "utf8mb4"
	}

	parseTime := "True"
	if !mariadbConfig.Connection.ParseTime {
		parseTime = "False"
	}

	loc := mariadbConfig.Connection.Loc
	if loc == "" {
		loc = "Local"
	}

	// Build DSN (Data Source Name)
	// Format: username:password@tcp(host:port)/dbname?param=value
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=%s&parseTime=%s&loc=%s",
		mariadbConfig.Connection.User,
		mariadbConfig.Connection.Password,
		mariadbConfig.Connection.Host,
		mariadbConfig.Connection.Port,
		mariadbConfig.Connection.DbName,
		charset,
		parseTime,
		loc,
	)

	// Add optional parameters
	if mariadbConfig.Connection.TLS != "" {
		dsn += "&tls=" + mariadbConfig.Connection.TLS
	}
	if mariadbConfig.Connection.Timeout != "" {
		dsn += "&timeout=" + mariadbConfig.Connection.Timeout
	}
	if mariadbConfig.Connection.ReadTimeout != "" {
		dsn += "&readTimeout=" + mariadbConfig.Connection.ReadTimeout
	}
	if mariadbConfig.Connection.WriteTimeout != "" {
		dsn += "&writeTimeout=" + mariadbConfig.Connection.WriteTimeout
	}

	database, err := gorm.Open(
		mysql.Open(dsn),
		&gorm.Config{
			TranslateError: true,
		})

	if err != nil {
		return nil, fmt.Errorf("failed to connect to MariaDB/MySQL database: %w", err)
	}

	databaseInstance, err := database.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get MariaDB/MySQL database instance: %w", err)
	}

	// Set connection pool parameters
	maxOpenConns := mariadbConfig.ConnectionDetails.MaxOpenConns
	if maxOpenConns <= 0 {
		maxOpenConns = 50
	}
	maxIdleConns := mariadbConfig.ConnectionDetails.MaxIdleConns
	if maxIdleConns <= 0 {
		maxIdleConns = 25
	}
	connMaxLifetime := mariadbConfig.ConnectionDetails.ConnMaxLifetime
	if connMaxLifetime <= 0 {
		connMaxLifetime = 1 * time.Minute
	}

	databaseInstance.SetMaxOpenConns(maxOpenConns)
	databaseInstance.SetMaxIdleConns(maxIdleConns)
	databaseInstance.SetConnMaxLifetime(connMaxLifetime)

	return database, nil
}

// RetryConnection continuously attempts to reconnect to the MariaDB database when notified
// of a connection failure. It operates as a goroutine that waits for signals on retryChanSignal
// before attempting reconnection. The function respects context cancellation and shutdown signals,
// ensuring graceful termination when requested.
//
// It implements two nested loops:
// - The outer loop waits for retry signals
// - The inner loop attempts reconnection until successful
func (m *MariaDB) RetryConnection(ctx context.Context) {
outerLoop:
	for {
		select {
		case <-m.shutdownSignal:
			m.logInfo(ctx, "Stopping RetryConnection loop due to shutdown signal", nil)
			return
		case <-ctx.Done():
			return
		case <-m.retryChanSignal:
		innerLoop:
			for {
				select {
				case <-m.shutdownSignal:
					return
				case <-ctx.Done():
					return
				default:
					newConn, err := connectToMariaDB(m.cfg)
					if err != nil {
						m.logError(ctx, "MariaDB reconnection failed", map[string]interface{}{
							"error": err.Error(),
						})
						time.Sleep(time.Second)
						continue innerLoop
					}
					m.client.Store(newConn)
					m.logInfo(ctx, "Successfully reconnected to MariaDB/MySQL database", nil)
					continue outerLoop
				}
			}
		}
	}
}

// MonitorConnection periodically checks the health of the database connection
// and triggers reconnection attempts when necessary. It runs as a goroutine that
// performs health checks at regular intervals (10 seconds) and signals the
// RetryConnection goroutine when a failure is detected.
//
// The function respects context cancellation and shutdown signals, ensuring
// proper resource cleanup and graceful termination when requested.
func (m *MariaDB) MonitorConnection(ctx context.Context) {
	defer m.closeRetryChanOnce.Do(func() {
		close(m.retryChanSignal)
	})

	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-m.shutdownSignal:
			m.logInfo(ctx, "Stopping MonitorConnection loop due to shutdown signal", nil)
			return
		case <-ticker.C:
			err := m.healthCheck()
			if err != nil {
				select {
				case m.retryChanSignal <- err:
				default:
				}
			}
		case <-ctx.Done():
			return
		}
	}
}

// healthCheck performs a health check on the MariaDB database connection.
// It snapshots the current *gorm.DB, then attempts to ping the database with a timeout
// of 5 seconds to verify connectivity.
//
// It returns nil if the database is healthy, or an error with details about the issue.
func (m *MariaDB) healthCheck() error {
	dbConn := m.DB()
	if dbConn == nil {
		return fmt.Errorf("database Client is not initialized")
	}

	db, err := dbConn.DB()
	if err != nil {
		return fmt.Errorf("failed to get database instance during health check: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		return fmt.Errorf("database ping failed during health check: %w", err)
	}

	return nil
}

// WithObserver attaches an observer to the MariaDB client for observability hooks.
// This method uses the builder pattern and returns the client for method chaining.
//
// The observer will be notified of all database operations, allowing
// external systems to track metrics, traces, or other observability data.
//
// Example:
//
//	client, err := mariadb.NewMariaDB(config)
//	if err != nil {
//	    return err
//	}
//	client = client.WithObserver(myObserver)
//	defer client.GracefulShutdown()
func (m *MariaDB) WithObserver(observer observability.Observer) *MariaDB {
	m.observer = observer
	return m
}

// WithLogger attaches a logger to the MariaDB client for internal logging.
// This method uses the builder pattern and returns the client for method chaining.
//
// The logger will be used for lifecycle events, connection monitoring, and background operations.
//
// Example:
//
//	client, err := mariadb.NewMariaDB(config)
//	if err != nil {
//	    return err
//	}
//	client = client.WithLogger(myLogger)
//	defer client.GracefulShutdown()
func (m *MariaDB) WithLogger(logger Logger) *MariaDB {
	m.logger = logger
	return m
}

// logInfo logs an informational message using the configured logger if available.
// This is used for lifecycle and background operation logging.
func (m *MariaDB) logInfo(ctx context.Context, msg string, fields map[string]interface{}) {
	if m.logger != nil {
		m.logger.InfoWithContext(ctx, msg, nil, fields)
	}
	// Silently skip if no logger configured
}

// logWarn logs a warning message using the configured logger if available.
// This is used for non-critical issues during connection monitoring.
func (m *MariaDB) logWarn(ctx context.Context, msg string, fields map[string]interface{}) {
	if m.logger != nil {
		m.logger.WarnWithContext(ctx, msg, nil, fields)
	}
	// Silently skip if no logger configured
}

// logError logs an error message using the configured logger if available.
// This is only used for errors in background goroutines that can't be returned to the caller.
func (m *MariaDB) logError(ctx context.Context, msg string, fields map[string]interface{}) {
	if m.logger != nil {
		m.logger.ErrorWithContext(ctx, msg, nil, fields)
	}
	// Silently skip if no logger configured
}
