package minio

import (
	"context"
	"go.uber.org/fx"
	"sync"
)

type MinioLifeCycleParams struct {
	fx.In

	Lifecycle fx.Lifecycle
	Minio     *Minio
	Logger    Logger
}

// FXModule is an fx.Module that provides and configures the MinIO client.
// This module registers the MinIO client with the Fx dependency injection framework,
// making it available to other components in the application.
//
// The module:
// 1. Provides the MinIO client factory function
// 2. Invokes the lifecycle registration to manage the client's lifecycle
//
// Usage:
//
//	app := fx.New(
//	    minio.FXModule,
//	    // other modules...
//	)
var FXModule = fx.Module("minio",
	fx.Provide(
		NewClient,
	),
	fx.Invoke(RegisterLifecycle),
)

// RegisterLifecycle registers the MinIO client with the fx lifecycle system.
// This function sets up proper initialization and graceful shutdown of the MinIO client,
// including starting and stopping the connection monitoring goroutines.
//
// Parameters:
//   - lc: The fx lifecycle controller
//   - mi: The MinIO client instance
//   - logger: Logger for recording lifecycle events
//
// The function registers two background goroutines:
// 1. A connection monitor that checks for connection health
// 2. A retry mechanism that handles reconnection when needed
//
// On application shutdown, it ensures these goroutines are properly terminated
// and waits for their completion before allowing the application to exit.
func RegisterLifecycle(params MinioLifeCycleParams) {

	if params.Minio == nil {
		params.Logger.Fatal("MinIO client is nil, cannot register lifecycle hooks", nil, nil)
		return
	}

	wg := &sync.WaitGroup{}
	params.Lifecycle.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {

			wg.Add(1)
			go func() {
				defer wg.Done()
				params.Minio.monitorConnection(ctx)
			}()

			wg.Add(1)
			go func() {
				defer wg.Done()
				params.Minio.retryConnection(ctx)
			}()

			return nil
		},
		OnStop: func(ctx context.Context) error {

			params.Logger.Info("closing minio client...", nil, nil)
			close(params.Minio.shutdownSignal)

			wg.Wait()
			return nil
		},
	})
}
