package schema_registry

import (
	"context"
	"log"

	"go.uber.org/fx"
)

// FXModule is an fx.Module that provides and configures the Schema Registry client.
// This module registers the Schema Registry client with the Fx dependency injection framework,
// making it available to other components in the application.
//
// The module:
// 1. Provides the Schema Registry client factory function
// 2. Invokes the lifecycle registration to manage the client's lifecycle
//
// Usage:
//
//	app := fx.New(
//	    schema_registry.FXModule,
//	    fx.Provide(
//	        func() schema_registry.Config {
//	            return schema_registry.Config{
//	                URL:      "http://localhost:8081",
//	                Username: "user",
//	                Password: "pass",
//	            }
//	        },
//	    ),
//	)
var FXModule = fx.Module("schema_registry",
	fx.Provide(
		NewClientWithDI,
	),
	fx.Invoke(RegisterSchemaRegistryLifecycle),
)

// SchemaRegistryParams groups the dependencies needed to create a Schema Registry client
type SchemaRegistryParams struct {
	fx.In

	Config Config
}

// NewClientWithDI creates a new Schema Registry client using dependency injection.
// This function is designed to be used with Uber's fx dependency injection framework
// where dependencies are automatically provided via the SchemaRegistryParams struct.
//
// Parameters:
//   - params: A SchemaRegistryParams struct that contains the Config instance
//     required to initialize the Schema Registry client.
//     This struct embeds fx.In to enable automatic injection of these dependencies.
//
// Returns:
//   - Registry: A fully initialized Schema Registry client ready for use.
//
// Example usage with fx:
//
//	app := fx.New(
//	    schema_registry.FXModule,
//	    fx.Provide(
//	        func() schema_registry.Config {
//	            return schema_registry.Config{
//	                URL:      os.Getenv("SCHEMA_REGISTRY_URL"),
//	                Username: os.Getenv("SCHEMA_REGISTRY_USER"),
//	                Password: os.Getenv("SCHEMA_REGISTRY_PASSWORD"),
//	                Timeout:  30 * time.Second,
//	            }
//	        },
//	    ),
//	)
func NewClientWithDI(params SchemaRegistryParams) (Registry, error) {
	return NewClient(params.Config)
}

// SchemaRegistryLifecycleParams groups the dependencies needed for Schema Registry lifecycle management
type SchemaRegistryLifecycleParams struct {
	fx.In

	Lifecycle fx.Lifecycle
	Registry  Registry
}

// RegisterSchemaRegistryLifecycle registers the Schema Registry client with the fx lifecycle system.
// This function sets up proper initialization and graceful shutdown of the Schema Registry client.
//
// Parameters:
//   - params: The lifecycle parameters containing the Schema Registry client
//
// The function:
//  1. On application start: Logs that the registry client is ready
//  2. On application stop: Currently no cleanup needed (HTTP client is stateless)
//
// This ensures that the Schema Registry client remains available throughout the application's
// lifetime and any future cleanup logic can be added here.
func RegisterSchemaRegistryLifecycle(params SchemaRegistryLifecycleParams) {
	params.Lifecycle.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			log.Println("INFO: Schema Registry client initialized")
			return nil
		},
		OnStop: func(ctx context.Context) error {
			log.Println("INFO: Schema Registry client shutdown")
			// HTTP client cleanup is handled automatically by Go runtime
			return nil
		},
	})
}
