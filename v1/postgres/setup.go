package postgres

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Aleph-Alpha/std/v1/observability"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// Postgres is a wrapper around gorm.DB that provides connection monitoring,
// automatic reconnection, and standardized database operations.
//
// Concurrency: the active `*gorm.DB` pointer is stored in an atomic pointer and can be
// swapped during reconnection without blocking readers.
type Postgres struct {
	cfg             Config
	client          atomic.Pointer[gorm.DB]
	observer        observability.Observer
	logger          Logger
	shutdownSignal  chan struct{}
	retryChanSignal chan error

	closeRetryChanOnce sync.Once
	closeShutdownOnce  sync.Once
}

// NewPostgres creates a new Postgres instance with the provided configuration and Logger.
// It establishes the initial database connection and sets up the internal state
// for connection monitoring and recovery. If the initial connection fails,
// it logs a fatal error and terminates.
//
// Returns *Postgres concrete type (following Go best practice: "accept interfaces, return structs").
func NewPostgres(cfg Config) (*Postgres, error) {
	conn, err := connectToPostgres(cfg)
	if err != nil {
		return nil, fmt.Errorf("error in connecting to postgres after all retries: %w", err)
	}

	pg := &Postgres{
		cfg:             cfg,
		observer:        nil, // No observer by default
		logger:          nil, // No logger by default
		shutdownSignal:  make(chan struct{}),
		retryChanSignal: make(chan error, 1),
	}
	pg.client.Store(conn)
	return pg, nil
}

// connectToPostgres establishes a connection to the PostgresSQL database using the provided
// configuration. It sets up the connection string, opens the connection with GORM,
// and configures the connection pool with appropriate parameters for performance.
// Returns the initialized GORM DB instance or an error if the connection fails.
func connectToPostgres(postgresConfig Config) (*gorm.DB, error) {
	pgConnStr := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		postgresConfig.Connection.Host,
		postgresConfig.Connection.Port,
		postgresConfig.Connection.User,
		postgresConfig.Connection.Password,
		postgresConfig.Connection.DbName,
		postgresConfig.Connection.SSLMode)

	database, err := gorm.Open(
		postgres.Open(pgConnStr),
		&gorm.Config{
			TranslateError: true,
		})

	if err != nil {
		return nil, fmt.Errorf("failed to connect to PostgresSQL database: %w", err)
	}

	databaseInstance, err := database.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get PostgresSQL database instance: %w", err)
	}

	// Set connection pool parameters.
	// If config fields are not set (zero), apply package defaults to preserve prior behavior.
	maxOpen := postgresConfig.ConnectionDetails.MaxOpenConns
	if maxOpen == 0 {
		maxOpen = 50
	}
	maxIdle := postgresConfig.ConnectionDetails.MaxIdleConns
	if maxIdle == 0 {
		maxIdle = 25
	}
	maxLifetime := postgresConfig.ConnectionDetails.ConnMaxLifetime
	if maxLifetime == 0 {
		maxLifetime = 1 * time.Minute
	}

	databaseInstance.SetMaxOpenConns(maxOpen)
	databaseInstance.SetMaxIdleConns(maxIdle)
	databaseInstance.SetConnMaxLifetime(maxLifetime)

	return database, nil
}

// RetryConnection continuously attempts to reconnect to the PostgresSQL database when notified
// of a connection failure. It operates as a goroutine that waits for signals on retryChanSignal
// before attempting reconnection. The function respects context cancellation and shutdown signals,
// ensuring graceful termination when requested.
//
// It implements two nested loops:
// - The outer loop waits for retry signals
// - The inner loop attempts reconnection until successful
func (p *Postgres) RetryConnection(ctx context.Context) {
outerLoop:
	for {
		select {
		case <-p.shutdownSignal:
			p.logInfo(ctx, "Stopping RetryConnection loop due to shutdown signal", nil)
			return
		case <-ctx.Done():
			return
		case <-p.retryChanSignal:
		innerLoop:
			for {
				select {
				case <-p.shutdownSignal:
					return
				case <-ctx.Done():
					return
				default:
					newConn, err := connectToPostgres(p.cfg)
					if err != nil {
						p.logError(ctx, "PostgreSQL reconnection failed", map[string]interface{}{
							"error": err.Error(),
						})
						time.Sleep(time.Second)
						continue innerLoop
					}
					p.client.Store(newConn)
					p.logInfo(ctx, "Successfully reconnected to PostgreSQL database", nil)
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
func (p *Postgres) MonitorConnection(ctx context.Context) {
	defer p.closeRetryChanOnce.Do(func() {
		close(p.retryChanSignal)
	})

	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-p.shutdownSignal:
			p.logInfo(ctx, "Stopping MonitorConnection loop due to shutdown signal", nil)
			return
		case <-ticker.C:
			err := p.healthCheck()
			if err != nil {
				select {
				case p.retryChanSignal <- err:
				default:
				}
			}
		case <-ctx.Done():
			return
		}
	}
}

// healthCheck performs a health check on the Postgres database connection.
// It snapshots the current *gorm.DB, then attempts to ping the database with a
// timeout of 5 seconds to verify connectivity.
//
// It returns nil if the database is healthy, or an error with details about the issue.
func (p *Postgres) healthCheck() error {
	// Snapshot the current connection; do not hold any package-level lock while pinging.
	dbConn := p.DB()
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

// WithObserver attaches an observer to the Postgres client for observability hooks.
// This method uses the builder pattern and returns the client for method chaining.
//
// The observer will be notified of all database operations, allowing
// external systems to track metrics, traces, or other observability data.
//
// Example:
//
//	client, err := postgres.NewPostgres(config)
//	if err != nil {
//	    return err
//	}
//	client = client.WithObserver(myObserver)
//	defer client.GracefulShutdown()
func (p *Postgres) WithObserver(observer observability.Observer) *Postgres {
	p.observer = observer
	return p
}

// WithLogger attaches a logger to the Postgres client for internal logging.
// This method uses the builder pattern and returns the client for method chaining.
//
// The logger will be used for lifecycle events, connection monitoring, and background operations.
//
// Example:
//
//	client, err := postgres.NewPostgres(config)
//	if err != nil {
//	    return err
//	}
//	client = client.WithLogger(myLogger)
//	defer client.GracefulShutdown()
func (p *Postgres) WithLogger(logger Logger) *Postgres {
	p.logger = logger
	return p
}

// logInfo logs an informational message using the configured logger if available.
// This is used for lifecycle and background operation logging.
func (p *Postgres) logInfo(ctx context.Context, msg string, fields map[string]interface{}) {
	if p.logger != nil {
		p.logger.InfoWithContext(ctx, msg, nil, fields)
	}
	// Silently skip if no logger configured
}

// logWarn logs a warning message using the configured logger if available.
// This is used for non-critical issues during connection monitoring.
func (p *Postgres) logWarn(ctx context.Context, msg string, fields map[string]interface{}) {
	if p.logger != nil {
		p.logger.WarnWithContext(ctx, msg, nil, fields)
	}
	// Silently skip if no logger configured
}

// logError logs an error message using the configured logger if available.
// This is only used for errors in background goroutines that can't be returned to the caller.
func (p *Postgres) logError(ctx context.Context, msg string, fields map[string]interface{}) {
	if p.logger != nil {
		p.logger.ErrorWithContext(ctx, msg, nil, fields)
	}
	// Silently skip if no logger configured
}
