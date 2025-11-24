package postgres

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// Postgres is a thread-safe wrapper around gorm.DB that provides connection monitoring,
// automatic reconnection, and standardized database operations.
// It guards all database operations with a mutex to ensure thread safety
// and includes mechanisms for graceful shutdown and connection health monitoring.
type Postgres struct {
	Client          *gorm.DB
	cfg             Config
	mu              *sync.RWMutex
	shutdownSignal  chan struct{}
	retryChanSignal chan error

	closeRetryChanOnce sync.Once
	closeShutdownOnce  sync.Once
}

// NewPostgres creates a new Postgres instance with the provided configuration and Logger.
// It establishes the initial database connection and sets up the internal state
// for connection monitoring and recovery. If the initial connection fails,
// it logs a fatal error and terminates.
func NewPostgres(cfg Config) (*Postgres, error) {
	conn, err := connectToPostgres(cfg)
	if err != nil {
		return nil, fmt.Errorf("error in connecting to postgres after all retries: %w", err)
	}

	return &Postgres{
		Client:          conn,
		cfg:             cfg,
		mu:              &sync.RWMutex{},
		shutdownSignal:  make(chan struct{}),
		retryChanSignal: make(chan error, 1),
	}, nil
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

	// Set connection pool parameters
	databaseInstance.SetMaxOpenConns(50)
	databaseInstance.SetMaxIdleConns(25)
	databaseInstance.SetConnMaxLifetime(1 * time.Minute)

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
					p.mu.Lock()
					p.Client = newConn
					p.mu.Unlock()
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
// It acquires a read lock to safely access the Client, then attempts to ping
// the database with a timeout of 5 seconds to verify connectivity.
//
// It returns nil if the database is healthy, or an error with details about the issue.
func (p *Postgres) healthCheck() error {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if p.Client == nil {
		return fmt.Errorf("database Client is not initialized")
	}

	db, err := p.Client.DB()
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
