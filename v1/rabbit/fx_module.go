package rabbit

import (
	"context"
	"sync"

	"github.com/Aleph-Alpha/std/v1/observability"
	amqp "github.com/rabbitmq/amqp091-go"
	"go.uber.org/fx"
)

// FXModule is an fx.Module that provides and configures the RabbitMQ client.
// This module registers the RabbitMQ client with the Fx dependency injection framework,
// making it available to other components in the application.
//
// The module provides:
// 1. *RabbitClient (concrete type) for direct use
// 2. Client interface for dependency injection
// 3. Lifecycle management for graceful startup and shutdown
//
// Usage:
//
//	app := fx.New(
//	    rabbit.FXModule,
//	    // other modules...
//	)
var FXModule = fx.Module("rabbit",
	fx.Provide(
		NewClientWithDI, // Provides *RabbitClient
		// Also provide the Client interface
		fx.Annotate(
			func(r *RabbitClient) Client { return r },
			fx.As(new(Client)),
		),
	),
	fx.Invoke(RegisterRabbitLifecycle),
)

// RabbitParams groups the dependencies needed to create a Rabbit client
type RabbitParams struct {
	fx.In

	Config   Config
	Logger   Logger                 `optional:"true"`
	Observer observability.Observer `optional:"true"`
}

// NewClientWithDI creates a new RabbitMQ client using dependency injection.
// This function is designed to be used with Uber's fx dependency injection framework
// where dependencies are automatically provided via the RabbitParams struct.
//
// Parameters:
//   - params: A RabbitParams struct that contains the Config instance
//     and optionally a Logger and Observer instances
//     required to initialize the RabbitMQ client.
//     This struct embeds fx.In to enable automatic injection of these dependencies.
//
// Returns:
//   - *RabbitClient: A fully initialized RabbitMQ client ready for use.
//
// Example usage with fx:
//
//	app := fx.New(
//	    rabbit.FXModule,
//	    logger.FXModule,  // Optional: provides logger
//	    fx.Provide(
//	        func() rabbit.Config {
//	            return loadRabbitConfig() // Your config loading function
//	        },
//	        func(metrics *prometheus.Metrics) observability.Observer {
//	            return &MyObserver{metrics: metrics}  // Optional observer
//	        },
//	    ),
//	)
//
// Under the hood, this function creates the client and injects the optional logger
// and observer before returning.
func NewClientWithDI(params RabbitParams) (*RabbitClient, error) {
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

// RabbitLifecycleParams groups the dependencies needed for RabbitMQ lifecycle management
type RabbitLifecycleParams struct {
	fx.In

	Lifecycle fx.Lifecycle
	Client    *RabbitClient
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
func (rb *RabbitClient) GracefulShutdown() {
	rb.closeShutdownOnce.Do(func() {
		close(rb.shutdownSignal)
	})
	rb.mu.Lock()

	rb.logInfo(context.Background(), "Shutting down RabbitMQ client", nil)

	if rb.Channel != nil {
		if err := rb.Channel.Close(); err != nil {
			rb.logWarn(context.Background(), "Failed to close rabbit channel", map[string]interface{}{
				"error": err.Error(),
			})
			return
		}
	}
	if rb.conn != nil && !rb.conn.IsClosed() {
		if err := rb.conn.Close(); err != nil {
			rb.logWarn(context.Background(), "Failed to close rabbit connection", map[string]interface{}{
				"error": err.Error(),
			})
			return
		}
	}
	rb.mu.Unlock()
}

// GetChannel returns the underlying AMQP channel for direct operations when needed.
// This allows advanced users to access RabbitMQ-specific functionality.
func (rb *RabbitClient) GetChannel() *amqp.Channel {
	rb.mu.RLock()
	defer rb.mu.RUnlock()
	return rb.Channel
}
