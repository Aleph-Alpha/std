package redis

import (
	"context"
	"log"

	"go.uber.org/fx"
)

// FXModule is an fx.Module that provides and configures the Redis client.
// This module registers the Redis client with the Fx dependency injection framework,
// making it available to other components in the application.
//
// The module:
// 1. Provides the Redis client factory function
// 2. Invokes the lifecycle registration to manage the client's lifecycle
//
// Usage:
//
//	app := fx.New(
//	    redis.FXModule,
//	    // other modules...
//	)
var FXModule = fx.Module("redis",
	fx.Provide(
		NewClientWithDI,
	),
	fx.Invoke(RegisterRedisLifecycle),
)

// RedisParams groups the dependencies needed to create a Redis client
type RedisParams struct {
	fx.In

	Config Config
	Logger Logger `optional:"true"` // Optional logger from std/v1/logger
}

// NewClientWithDI creates a new Redis client using dependency injection.
// This function is designed to be used with Uber's fx dependency injection framework
// where dependencies are automatically provided via the RedisParams struct.
//
// Parameters:
//   - params: A RedisParams struct that contains the Config instance
//     and optionally a Logger instance required to initialize the Redis client.
//     This struct embeds fx.In to enable automatic injection of these dependencies.
//
// Returns:
//   - *Redis: A fully initialized Redis client ready for use.
//
// Example usage with fx:
//
//	app := fx.New(
//	    redis.FXModule,
//	    logger.FXModule, // Optional: provides logger
//	    fx.Provide(
//	        func() redis.Config {
//	            return loadRedisConfig() // Your config loading function
//	        },
//	    ),
//	)
//
// Under the hood, this function injects the optional logger before delegating
// to the standard NewClient function.
func NewClientWithDI(params RedisParams) (*RedisClient, error) {
	// Inject the logger into the config if provided
	if params.Logger != nil {
		params.Config.Logger = params.Logger
	}

	return NewClient(params.Config)
}

// RedisLifecycleParams groups the dependencies needed for Redis lifecycle management
type RedisLifecycleParams struct {
	fx.In

	Lifecycle fx.Lifecycle
	Client    *RedisClient
}

// RegisterRedisLifecycle registers the Redis client with the fx lifecycle system.
// This function sets up proper initialization and graceful shutdown of the Redis client.
//
// Parameters:
//   - params: The lifecycle parameters containing the Redis client
//
// The function:
//  1. On application start: Pings Redis to ensure the connection is healthy
//  2. On application stop: Triggers a graceful shutdown of the Redis client,
//     closing connections cleanly.
//
// This ensures that the Redis client remains available throughout the application's
// lifetime and is properly cleaned up during shutdown.
func RegisterRedisLifecycle(params RedisLifecycleParams) {
	params.Lifecycle.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			// Ping Redis to ensure connection is healthy
			if err := params.Client.Ping(ctx); err != nil {
				log.Printf("WARN: Failed to ping Redis on startup: %v", err)
				return err
			}
			log.Println("INFO: Redis client started and healthy")
			return nil
		},
		OnStop: func(ctx context.Context) error {
			log.Println("INFO: Shutting down Redis client")
			return params.Client.Close()
		},
	})
}

// ClusterFXModule is an fx.Module for Redis Cluster configuration.
var ClusterFXModule = fx.Module("redis-cluster",
	fx.Provide(
		NewClusterClientWithDI,
	),
	fx.Invoke(RegisterRedisLifecycle),
)

// ClusterRedisParams groups the dependencies needed to create a Redis Cluster client
type ClusterRedisParams struct {
	fx.In

	Config ClusterConfig
	Logger Logger `optional:"true"`
}

// NewClusterClientWithDI creates a new Redis Cluster client using dependency injection.
func NewClusterClientWithDI(params ClusterRedisParams) (*RedisClient, error) {
	// Inject the logger into the config if provided
	if params.Logger != nil {
		params.Config.Logger = params.Logger
	}

	return NewClusterClient(params.Config)
}

// FailoverFXModule is an fx.Module for Redis Sentinel (failover) configuration.
var FailoverFXModule = fx.Module("redis-failover",
	fx.Provide(
		NewFailoverClientWithDI,
	),
	fx.Invoke(RegisterRedisLifecycle),
)

// FailoverRedisParams groups the dependencies needed to create a Redis Sentinel client
type FailoverRedisParams struct {
	fx.In

	Config FailoverConfig
	Logger Logger `optional:"true"`
}

// NewFailoverClientWithDI creates a new Redis Sentinel client using dependency injection.
func NewFailoverClientWithDI(params FailoverRedisParams) (*RedisClient, error) {
	// Inject the logger into the config if provided
	if params.Logger != nil {
		params.Config.Logger = params.Logger
	}

	return NewFailoverClient(params.Config)
}
