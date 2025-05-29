package postgres

import (
	"context"
	"go.uber.org/fx"
	"sync"
)

// FXModule is an fx module that provides the Postgres database component.
// It registers the Postgres constructor for dependency injection
// and sets up lifecycle hooks to properly initialize and shut down
// the database connection.
var FXModule = fx.Module("postgres",
	fx.Provide(
		NewMinioClientWithDI,
	),
	fx.Invoke(RegisterPostgresLifecycle),
)

// PostgresParams groups the dependencies needed to create a Postgres Client via dependency injection.
// This struct is designed to work with Uber's fx dependency injection framework and provides
// the necessary parameters for initializing a Postgres database connection.
//
// The embedded fx.In marker enables automatic injection of the struct fields from the
// dependency container when this struct is used as a parameter in provider functions.
type PostgresParams struct {
	fx.In

	Config Config
	Logger Logger
}

// NewMinioClientWithDI creates a new Postgres Client using dependency injection.
// This function is designed to be used with Uber's fx dependency injection framework
// where the Config and Logger dependencies are automatically provided via the PostgresParams struct.
//
// Parameters:
//   - params: A PostgresParams struct containing the Config and Logger instances
//     required to initialize the Postgres Client. This struct embeds fx.In to enable
//     automatic injection of these dependencies.
//
// Returns:
//   - *Postgres: A fully initialized Postgres Client ready for use.
//
// Example usage with fx:
//
//	app := fx.New(
//	    postgres.FXModule,
//	    fx.Provide(
//	        func() postgres.Config {
//	            return loadPostgresConfig() // Your config loading function
//	        },
//	        func() postgres.Logger {
//	            return initLogger() // Your logger initialization
//	        },
//	    ),
//	)
//
// This function delegates to the standard NewPostgres function, maintaining the same
// initialization logic while enabling seamless integration with dependency injection.
func NewMinioClientWithDI(params PostgresParams) *Postgres {
	return NewPostgres(params.Config, params.Logger)
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
	Logger    Logger
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
				params.Postgres.RetryConnection(ctx, params.Logger)
			}()

			return nil
		},
		OnStop: func(ctx context.Context) error {
			close(params.Postgres.shutdownSignal)
			wg.Wait()

			params.Postgres.closeOnce.Do(func() {
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
	close(p.shutdownSignal)
	p.closeOnce.Do(func() {
		close(p.retryChanSignal)
	})
	p.mu.Lock()
	// Close the database connection
	sqlDB, err := p.DB().DB()
	if err == nil {
		err := sqlDB.Close()
		if err != nil {
			return err
		}
	}
	p.mu.Unlock()
	return nil
}
