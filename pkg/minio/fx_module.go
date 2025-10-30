package minio

import (
	"context"
	"sync"

	"go.uber.org/fx"
)

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
		NewMinioClientWithDI,
	),
	fx.Invoke(RegisterLifecycle),
)

type MinioParams struct {
	fx.In

	Config
	// Logger is optional - if not provided, fallback logger will be used
	Logger MinioLogger `optional:"true"`
}

func NewMinioClientWithDI(params MinioParams) (*Minio, error) {
	return NewClient(params.Config, params.Logger)
}

type MinioLifeCycleParams struct {
	fx.In

	Lifecycle fx.Lifecycle
	Minio     *Minio
}

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
		// Use fallback logger since we don't have a MinIO client
		fallbackLog := newFallbackLogger()
		fallbackLog.Fatal("MinIO client is nil, cannot register lifecycle hooks", nil, nil)
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

			params.Minio.logger.Info("closing minio client...", nil, nil)

			// Clean up resources before shutdown
			params.Minio.CleanupResources()
			params.Minio.GracefulShutdown()

			wg.Wait()
			return nil
		},
	})
}

// GracefulShutdown safely terminates all MinIO client operations and closes associated resources.
// This method ensures a clean shutdown process by using sync.Once to guarantee each channel
// is closed exactly once, preventing "close of closed channel" panics that could occur when
// multiple shutdown paths execute concurrently.
//
// The shutdown process:
// 1. Safely closes the shutdownSignal channel to notify all monitoring goroutines to stop
// 2. Safely closes the reconnectSignal channel to terminate any reconnection attempts
//
// This method is designed to be called from multiple potential places (such as application
// shutdown hooks, context cancellation handlers, or defer statements) without causing
// resource management issues. The use of sync.Once ensures that even if called multiple times,
// each cleanup operation happens exactly once.
//
// Example usage:
//
//	// In application shutdown code
//	func shutdownApp() {
//	    minioClient.GracefulShutdown()
//	    // other cleanup...
//	}
//
//	// Or with defer
//	func processFiles() {
//	    defer minioClient.GracefulShutdown()
//	    // processing logic...
//	}
func (m *Minio) GracefulShutdown() {
	m.closeShutdownOnce.Do(func() {
		close(m.shutdownSignal)
	})

	m.closeReconnectOnce.Do(func() {
		close(m.reconnectSignal)
	})
}
