package postgres

import (
	"context"
	"fmt"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"sync"
	"time"
)

type logger interface {
	Info(msg string, err error, fields ...map[string]interface{})
	Debug(msg string, err error, fields ...map[string]interface{})
	Warn(msg string, err error, fields ...map[string]interface{})
	Error(msg string, err error, fields ...map[string]interface{})
	Fatal(msg string, err error, fields ...map[string]interface{})
}

type Postgres struct {
	client          *gorm.DB
	cfg             Config
	logger          logger
	mu              sync.RWMutex
	shutdownSignal  chan struct{}
	retryChanSignal chan error
}

func NewPostgres(cfg Config, logger logger) *Postgres {
	conn, err := connectToPostgres(logger, cfg)
	if err != nil {
		logger.Fatal("error in connecting to postgres after all retries", nil, nil)
	}

	return &Postgres{
		client:          conn,
		cfg:             cfg,
		logger:          logger,
		shutdownSignal:  make(chan struct{}),
		retryChanSignal: make(chan error),
	}
}

func connectToPostgres(logger logger, postgresConfig Config) (*gorm.DB, error) {
	pgConnStr := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s", postgresConfig.Connection.Host, postgresConfig.Connection.Port, postgresConfig.Connection.User, postgresConfig.Connection.Password, postgresConfig.Connection.DbName, postgresConfig.Connection.SSLMode)

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

	logger.Info("Successfully connected to PostgreSQL database", nil, nil)

	return database, nil
}

func (p *Postgres) retryConnection(ctx context.Context, logger logger, cfg Config) {
outerLoop:
	for {
		select {
		case <-p.shutdownSignal:
			logger.Info("Stopping retryConnection loop due to shutdown signal", nil, nil)
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
					newConn, err := connectToPostgres(logger, cfg)
					if err != nil {
						logger.Error("Reconnection failed", err, nil)
						time.Sleep(time.Second)
						continue innerLoop
					}
					p.mu.Lock()
					p.client = newConn
					p.mu.Unlock()
					logger.Info("Reconnected to PostgresSQL database", nil, nil)
					continue outerLoop
				}
			}
		}
	}
}

func (p *Postgres) monitorConnection(ctx context.Context) {
	defer close(p.retryChanSignal)
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-p.shutdownSignal:
			p.logger.Info("Stopping monitorConnection loop due to shutdown signal", nil, nil)
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
// It returns nil if the database is healthy, or an error with details about the issue.
func (p *Postgres) healthCheck() error {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if p.client == nil {
		return fmt.Errorf("database client is not initialized")
	}

	db, err := p.client.DB()
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
