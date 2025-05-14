package minio

import (
	"context"
	"go.uber.org/fx"
	"sync"
)

var FXModule = fx.Module("minio",
	fx.Provide(
		NewClient,
	),
	fx.Invoke(RegisterLifecycle),
)

func RegisterLifecycle(lc fx.Lifecycle, mi *Minio, logger Logger) {

	if mi == nil {
		logger.Fatal("MinIO client is nil, cannot register lifecycle hooks", nil, nil)
		return
	}

	wg := &sync.WaitGroup{}
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {

			wg.Add(1)
			go func() {
				defer wg.Done()
				mi.monitorConnection(ctx)
			}()

			wg.Add(1)
			go func() {
				defer wg.Done()
				mi.retryConnection(ctx)
			}()

			return nil
		},
		OnStop: func(ctx context.Context) error {

			logger.Info("closing minio client...", nil, nil)
			close(mi.shutdownSignal)

			wg.Wait()
			return nil
		},
	})
}
