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
func RegisterPostgresLifecycle(lifecycle fx.Lifecycle, postgres *Postgres, logger Logger) {
	wg := &sync.WaitGroup{}
	lifecycle.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			wg.Add(1)
			go func() {
				defer wg.Done()
				postgres.monitorConnection(ctx)
			}()

			wg.Add(1)
			go func() {
				defer wg.Done()
				postgres.retryConnection(ctx, logger, postgres.cfg)
			}()

			return nil
		},
		OnStop: func(ctx context.Context) error {
			close(postgres.shutdownSignal)
			wg.Wait()

			// Close the database connection
			sqlDB, err := postgres.DB().DB()
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
