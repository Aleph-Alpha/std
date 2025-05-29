package rabbit

import (
	"context"
	"go.uber.org/fx"
	"sync"
)

// FXModule is an fx.Module that provides and configures the RabbitMQ client.
// This module registers the RabbitMQ client with the Fx dependency injection framework,
// making it available to other components in the application.
//
// The module:
// 1. Provides the RabbitMQ client factory function
// 2. Invokes the lifecycle registration to manage the client's lifecycle
//
// Usage:
//
//	app := fx.New(
//	    rabbit.FXModule,
//	    // other modules...
//	)
var FXModule = fx.Module("rabbit",
	fx.Provide(
		NewClient,
	),
	fx.Invoke(RegisterRabbitLifecycle),
)

// RabbitLifecycleParams groups the dependencies needed for RabbitMQ lifecycle management
type RabbitLifecycleParams struct {
	fx.In

	Lifecycle fx.Lifecycle
	Client    *Rabbit
	Logger    Logger
	Config    Config
}

// RegisterRabbitLifecycle registers the RabbitMQ client with the fx lifecycle system.
// This function sets up proper initialization and graceful shutdown of the RabbitMQ client,
// including starting the connection monitoring goroutine.
//
// Parameters:
//   - lc: The fx lifecycle controller
//   - client: The RabbitMQ client instance
//   - logger: Logger for recording lifecycle events
//   - cfg: Configuration for the RabbitMQ client
//
// The function:
//  1. On application start: Launches a background goroutine that monitors and maintains
//     the RabbitMQ connection, automatically reconnecting if it fails.
//  2. On application stop: Triggers a graceful shutdown of the RabbitMQ client,
//     closing channels and connections cleanly.
//
// This ensures that the RabbitMQ client remains available throughout the application's
// lifetime and is properly cleaned up during shutdown.
func RegisterRabbitLifecycle(params RabbitLifecycleParams) {
	wg := &sync.WaitGroup{}

	params.Lifecycle.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			wg.Add(1)

			go func(logger Logger, cfg Config) {
				defer wg.Done()
				params.Client.retryConnection(logger, cfg)
			}(params.Logger, params.Config)

			return nil
		},
		OnStop: func(ctx context.Context) error {

			params.Client.gracefulShutdown()

			wg.Wait()
			return nil
		},
	})
}

// gracefulShutdown closes the RabbitMQ client's connections and channels cleanly.
// This method ensures that all resources are properly released when the application
// is shutting down.
//
// The shutdown process:
// 1. Signals all goroutines to stop by closing the shutdownSignal channel
// 2. Acquires a lock to prevent concurrent access during shutdown
// 3. Closes the AMQP channel if it exists
// 4. Closes the AMQP connection if it exists and is not already closed
//
// Any errors during shutdown are logged but not propagated, as they typically
// cannot be handled at this stage of application shutdown.
func (rb *Rabbit) gracefulShutdown() {
	close(rb.shutdownSignal)
	rb.mu.Lock()

	rb.logger.Info("closing rabbit channel...", nil, nil)

	if rb.Channel != nil {
		if err := rb.Channel.Close(); err != nil {
			rb.logger.Error("error in closing rabbit channel", err, nil)
			return
		}
	}
	if rb.conn != nil && !rb.conn.IsClosed() {
		if err := rb.conn.Close(); err != nil {
			rb.logger.Error("error in closing rabbit connection", err, nil)
			return
		}
	}
	rb.mu.Unlock()
}
