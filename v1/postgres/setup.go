package postgres

import (
	"context"
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"

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

	log.Println("INFO: Successfully connected to PostgresSQL database")

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
			log.Println("INFO: Stopping RetryConnection loop due to shutdown signal")
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
						log.Printf("ERROR: PostgresSQL reconnection failed: %v", err)
						time.Sleep(time.Second)
						continue innerLoop
					}
					p.client.Store(newConn)
					log.Println("INFO: Successfully reconnected to PostgresSQL database")
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
			log.Println("INFO: Stopping MonitorConnection loop due to shutdown signal")
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
