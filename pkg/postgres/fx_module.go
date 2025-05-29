package postgres

import (
	"context"
	"go.uber.org/fx"
	"sync"
)

type PostgresLifeCycleParams struct {
	fx.In

	Lifecycle fx.Lifecycle
	Postgres  *Postgres
	Logger    Logger
}

// FXModule is an fx module that provides the Postgres database component.
// It registers the Postgres constructor for dependency injection
// and sets up lifecycle hooks to properly initialize and shut down
// the database connection.
var FXModule = fx.Module("postgres",
	fx.Provide(
		NewPostgres,
	),
	fx.Invoke(RegisterPostgresLifecycle),
)

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
				params.Postgres.monitorConnection(ctx)
			}()

			wg.Add(1)
			go func() {
				defer wg.Done()
				params.Postgres.retryConnection(ctx, params.Logger)
			}()

			return nil
		},
		OnStop: func(ctx context.Context) error {
			close(params.Postgres.shutdownSignal)
			wg.Wait()

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
