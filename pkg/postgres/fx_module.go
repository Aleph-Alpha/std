package postgres

import (
	"context"
	"go.uber.org/fx"
	"sync"
)

var FXModule = fx.Module("postgres",
	fx.Provide(
		NewPostgres,
	),
	fx.Invoke(RegisterPostgresLifecycle),
)

func RegisterPostgresLifecycle(lifecycle fx.Lifecycle, postgres *Postgres, logger logger) {
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
			return nil
		},
	})
}
