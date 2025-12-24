package postgres

import (
	"context"
	"sync"

	"go.uber.org/fx"
)

// FXModule is an fx module that provides the Postgres database component.
// It registers the Postgres constructor for dependency injection
// and sets up lifecycle hooks to properly initialize and shut down
// the database connection.
//
// This module provides:
//   - *Postgres (concrete type) - for direct use and lifecycle management
//   - Client (interface) - for consumers who want database abstraction
//
// Consumers can inject either:
//   - *Postgres for full access to all methods
//   - Client for interface-based programming
var FXModule = fx.Module("postgres",
	fx.Provide(
		NewPostgresClientWithDI, // Returns *Postgres for internal lifecycle
		fx.Annotate(
			ProvideClient,      // Returns Client interface
			fx.As(new(Client)), // Expose as Client interface
		),
	),
	fx.Invoke(RegisterPostgresLifecycle),
)

// ProvideClient wraps the concrete *Postgres and returns it as Client interface.
// This enables applications to depend on the interface rather than concrete type.
func ProvideClient(pg *Postgres) Client {
	return pg
}

// PostgresParams groups the dependencies needed to create a Postgres Client via dependency injection.
// This struct is designed to work with Uber's fx dependency injection framework and provides
// the necessary parameters for initializing a Postgres database connection.
//
// The embedded fx.In marker enables automatic injection of the struct fields from the
// dependency container when this struct is used as a parameter in provider functions.
type PostgresParams struct {
	fx.In

	Config Config
}

// NewPostgresClientWithDI creates a new Postgres Client using dependency injection.
// This function is designed to be used with Uber's fx dependency injection framework
// where the Config and Logger dependencies are automatically provided via the PostgresParams struct.
//
// Parameters:
//   - params: A PostgresParams struct containing the Config and Logger instances
//     required to initialize the Postgres Client. This struct embeds fx.In to enable
//     automatic injection of these dependencies.
//
// Returns:
//   - *Postgres: A fully initialized Postgres Client (concrete type).
//     The FX module also provides this as Client interface for consumers who want abstraction.
//
// Example usage with fx:
//
//	app := fx.New(
//	    postgres.FXModule,
//	    fx.Provide(
//	        func() postgres.Config {
//	            return loadPostgresConfig() // Your config loading function
//	        },
//	    ),
//	)
//
// This function delegates to the standard NewPostgres function, maintaining the same
// initialization logic while enabling seamless integration with dependency injection.
func NewPostgresClientWithDI(params PostgresParams) (*Postgres, error) {
	return NewPostgres(params.Config)
}

// PostgresLifeCycleParams groups the dependencies needed for Postgres lifecycle management.
// This struct combines all the components required to properly manage the lifecycle
// of a Postgres Client within an fx application, including startup, monitoring,
// and graceful shutdown.
//
// The embedded fx.In marker enables automatic injection of the struct fields from the
// dependency container when this struct is used as a parameter in lifecycle registration functions.
type PostgresLifeCycleParams struct {
	fx.In

	Lifecycle fx.Lifecycle
	Postgres  *Postgres
}

// RegisterPostgresLifecycle registers lifecycle hooks for the Postgres database component.
// It sets up:
// 1. Connection monitoring on the application starts
// 2. Automatic reconnection mechanism on application start
// 3. Graceful shutdown of database connections on application stop
//
// The function uses a WaitGroup to ensure that all goroutines complete
// before the application terminates.
func RegisterPostgresLifecycle(params PostgresLifeCycleParams) {
	wg := &sync.WaitGroup{}
	params.Lifecycle.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			wg.Add(1)
			go func() {
				defer wg.Done()
				params.Postgres.MonitorConnection(ctx)
			}()

			wg.Add(1)
			go func() {
				defer wg.Done()
				params.Postgres.RetryConnection(ctx)
			}()

			return nil
		},
		OnStop: func(ctx context.Context) error {
			params.Postgres.closeShutdownOnce.Do(func() {
				close(params.Postgres.shutdownSignal)
			})

			wg.Wait()

			params.Postgres.closeRetryChanOnce.Do(func() {
				close(params.Postgres.retryChanSignal)
			})

			// Close the database connection
			sqlDB, err := params.Postgres.DB().DB()
			if err == nil {
				err := sqlDB.Close()
				if err != nil {
					return err
				}
			}

			return nil
		},
	})
}

func (p *Postgres) GracefulShutdown() error {
	if p.shutdownSignal != nil {
		p.closeShutdownOnce.Do(func() {
			close(p.shutdownSignal)
		})
	}

	if p.retryChanSignal != nil {
		p.closeRetryChanOnce.Do(func() {
			close(p.retryChanSignal)
		})
	}

	// Snapshot the connection; do not hold any package-level lock while closing.
	dbConn := p.DB()
	if dbConn == nil {
		return nil
	}

	sqlDB, err := dbConn.DB()
	if err != nil {
		return nil
	}
	if err := sqlDB.Close(); err != nil {
		return err
	}
	return nil
}
