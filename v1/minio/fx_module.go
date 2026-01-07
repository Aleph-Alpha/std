package minio

import (
	"context"
	"sync"

	"github.com/Aleph-Alpha/std/v1/observability"
	"go.uber.org/fx"
)

// FXModule is an fx.Module that provides and configures the MinIO client.
// This module registers the MinIO client with the Fx dependency injection framework,
// making it available to other components in the application.
//
// The module provides:
// 1. *MinioClient (concrete type) for direct use
// 2. Client interface for dependency injection
// 3. Lifecycle management for graceful startup and shutdown
//
// Note: The client supports multi-bucket operations. Bucket names must be
// specified explicitly in each operation call.
//
// Usage:
//
//	app := fx.New(
//	    minio.FXModule,
//	    // other modules...
//	)
var FXModule = fx.Module("minio",
	fx.Provide(
		NewMinioClientWithDI, // Provides *MinioClient
		// Also provide the Client interface
		fx.Annotate(
			func(m *MinioClient) Client { return m },
			fx.As(new(Client)),
		),
	),
	fx.Invoke(RegisterLifecycle),
)

type MinioParams struct {
	fx.In

	Config   Config
	Logger   Logger                 `optional:"true"`
	Observer observability.Observer `optional:"true"`
}

func NewMinioClientWithDI(params MinioParams) (*MinioClient, error) {
	// Create client with config
	client, err := NewClient(params.Config)
	if err != nil {
		return nil, err
	}

	// Inject logger if provided
	if params.Logger != nil {
		client.logger = params.Logger
	}

	// Inject observer if provided
	if params.Observer != nil {
		client.observer = params.Observer
	}

	return client, nil
}

type MinioLifeCycleParams struct {
	fx.In

	Lifecycle fx.Lifecycle
	Minio     *MinioClient
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
		// Cannot register lifecycle hooks without a client
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

			params.Minio.logInfo(ctx, "closing minio client", nil)

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
func (m *MinioClient) GracefulShutdown() {
	m.closeShutdownOnce.Do(func() {
		close(m.shutdownSignal)
	})
}
