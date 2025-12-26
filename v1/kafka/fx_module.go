package kafka

import (
	"context"
	"log"

	"github.com/Aleph-Alpha/std/v1/observability"
	"go.uber.org/fx"
)

// FXModule is an fx.Module that provides and configures the Kafka client.
// This module registers the Kafka client with the Fx dependency injection framework,
// making it available to other components in the application.
//
// The module provides:
// 1. *KafkaClient (concrete type) for direct use
// 2. Client interface for dependency injection
// 3. Lifecycle management for graceful startup and shutdown
//
// Usage:
//
//	app := fx.New(
//	    kafka.FXModule,
//	    // other modules...
//	)
var FXModule = fx.Module("kafka",
	fx.Provide(
		NewClientWithDI, // Provides *KafkaClient
		// Also provide the Client interface
		fx.Annotate(
			func(k *KafkaClient) Client { return k },
			fx.As(new(Client)),
		),
	),
	fx.Invoke(RegisterKafkaLifecycle),
)

// KafkaParams groups the dependencies needed to create a Kafka client
type KafkaParams struct {
	fx.In

	Config       Config
	Logger       Logger                 `optional:"true"` // Optional logger from std/v1/logger
	Serializer   Serializer             `optional:"true"` // Optional serializer
	Deserializer Deserializer           `optional:"true"` // Optional deserializer
	Observer     observability.Observer `optional:"true"` // Optional observer for metrics/tracing
}

// NewClientWithDI creates a new Kafka client using dependency injection.
// This function is designed to be used with Uber's fx dependency injection framework
// where dependencies are automatically provided via the KafkaParams struct.
//
// Parameters:
//   - params: A KafkaParams struct that contains the Config instance
//     and optionally a Logger, Serializer, Deserializer, and Observer instances
//     required to initialize the Kafka client.
//     This struct embeds fx.In to enable automatic injection of these dependencies.
//
// Returns:
//   - *KafkaClient: A fully initialized Kafka client ready for use.
//
// Example usage with fx:
//
//	app := fx.New(
//	    kafka.FXModule,
//	    logger.FXModule,  // Optional: provides logger
//	    fx.Provide(
//	        func() kafka.Config {
//	            return loadKafkaConfig() // Your config loading function
//	        },
//	        func() kafka.Serializer {
//	            return &kafka.JSONSerializer{}  // Or ProtobufSerializer, etc.
//	        },
//	        func() kafka.Deserializer {
//	            return &kafka.JSONDeserializer{}
//	        },
//	        func(metrics *prometheus.Metrics) observability.Observer {
//	            return &MyObserver{metrics: metrics}  // Optional observer
//	        },
//	    ),
//	)
//
// Under the hood, this function injects the optional logger, serializer,
// deserializer, and observer before delegating to the standard NewClient function.
func NewClientWithDI(params KafkaParams) (*KafkaClient, error) {
	// Inject the logger into the config if provided
	if params.Logger != nil {
		params.Config.Logger = params.Logger
	}

	client, err := NewClient(params.Config)
	if err != nil {
		return nil, err
	}

	// Inject serializers directly if provided (overrides defaults)
	if params.Serializer != nil {
		client.SetSerializer(params.Serializer)
	}

	if params.Deserializer != nil {
		client.SetDeserializer(params.Deserializer)
	}

	// If no serializers were injected, ensure defaults are set
	// (already called in NewClient, but this is explicit)
	if params.Serializer == nil && params.Deserializer == nil {
		client.SetDefaultSerializers()
	}

	// Inject observer if provided
	if params.Observer != nil {
		client.observer = params.Observer
	}

	return client, nil
}

// KafkaLifecycleParams groups the dependencies needed for Kafka lifecycle management
type KafkaLifecycleParams struct {
	fx.In

	Lifecycle fx.Lifecycle
	Client    *KafkaClient
}

// RegisterKafkaLifecycle registers the Kafka client with the fx lifecycle system.
// This function sets up proper initialization and graceful shutdown of the Kafka client.
//
// Parameters:
//   - params: The lifecycle parameters containing the Kafka client
//
// The function:
//  1. On application start: Ensures the client is ready for use
//  2. On application stop: Triggers a graceful shutdown of the Kafka client,
//     closing producers and consumers cleanly.
//
// This ensures that the Kafka client remains available throughout the application's
// lifetime and is properly cleaned up during shutdown.
func RegisterKafkaLifecycle(params KafkaLifecycleParams) {
	params.Lifecycle.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			log.Println("INFO: Kafka client started")
			return nil
		},
		OnStop: func(ctx context.Context) error {
			log.Println("INFO: Shutting down Kafka client")
			params.Client.GracefulShutdown()
			return nil
		},
	})
}

// GracefulShutdown closes the Kafka client's connections cleanly.
// This method ensures that all resources are properly released when the application
// is shutting down.
//
// The shutdown process:
// 1. Signals all goroutines to stop by closing the shutdownSignal channel
// 2. Closes the writer if it exists
// 3. Closes the reader if it exists
//
// Any errors during shutdown are logged but not propagated, as they typically
// cannot be handled at this stage of application shutdown.
func (k *KafkaClient) GracefulShutdown() {
	k.closeShutdownOnce.Do(func() {
		close(k.shutdownSignal)
	})

	k.mu.Lock()
	defer k.mu.Unlock()

	log.Println("INFO: Closing Kafka client")

	if k.writer != nil {
		if err := k.writer.Close(); err != nil {
			log.Printf("WARN: Failed to close Kafka writer: %v", err)
		}
	}

	if k.reader != nil {
		if err := k.reader.Close(); err != nil {
			log.Printf("WARN: Failed to close Kafka reader: %v", err)
		}
	}
}
