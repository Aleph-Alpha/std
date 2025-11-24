package rabbit

import (
	"context"
	"log"
	"sync"

	"go.uber.org/fx"
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
		NewClientWithDI,
	),
	fx.Invoke(RegisterRabbitLifecycle),
)

// RabbitParams groups the dependencies needed to create a Rabbit client
type RabbitParams struct {
	fx.In

	Config Config
}

// NewClientWithDI creates a new RabbitMQ client using dependency injection.
// This function is designed to be used with Uber's fx dependency injection framework
// where dependencies are automatically provided via the RabbitParams struct.
//
// Parameters:
//   - params: A RabbitParams struct that contains the Config and Logger instances
//     required to initialize the RabbitMQ client. This struct embeds fx.In to enable
//     automatic injection of these dependencies.
//
// Returns:
//   - *Rabbit: A fully initialized RabbitMQ client ready for use.
//
// Example usage with fx:
//
//	app := fx.New(
//	    rabbit.FXModule,
//	    fx.Provide(
//	        func() rabbit.Config {
//	            return loadRabbitConfig() // Your config loading function
//	        },
//	        func() rabbit.Logger {
//	            return initLogger() // Your logger initialization
//	        },
//	    ),
//	)
//
// Under the hood, this function simply delegates to the standard NewClient function,
// making it easier to integrate with dependency injection frameworks while maintaining
// the same initialization logic.
func NewClientWithDI(params RabbitParams) (*Rabbit, error) {
	return NewClient(params.Config)
}

// RabbitLifecycleParams groups the dependencies needed for RabbitMQ lifecycle management
type RabbitLifecycleParams struct {
	fx.In

	Lifecycle fx.Lifecycle
	Client    *Rabbit
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

			go func(cfg Config) {
				defer wg.Done()
				params.Client.RetryConnection(cfg)
			}(params.Config)

			return nil
		},
		OnStop: func(ctx context.Context) error {

			params.Client.GracefulShutdown()

			wg.Wait()
			return nil
		},
	})
}

// GracefulShutdown closes the RabbitMQ client's connections and channels cleanly.
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
func (rb *Rabbit) GracefulShutdown() {
	rb.closeShutdownOnce.Do(func() {
		close(rb.shutdownSignal)
	})
	rb.mu.Lock()

	log.Println("INFO: Shutting down RabbitMQ client")

	if rb.Channel != nil {
		if err := rb.Channel.Close(); err != nil {
			log.Println("WARN: Failed to close rabbit channel")
			return
		}
	}
	if rb.conn != nil && !rb.conn.IsClosed() {
		if err := rb.conn.Close(); err != nil {
			log.Println("WARN: Failed to close rabbit connection")
			return
		}
	}
	rb.mu.Unlock()
}
