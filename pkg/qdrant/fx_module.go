package qdrant

import (
	"context"
	"log"
	"sync"

	"go.uber.org/fx"
)

// FXModule defines the Fx module for the Qdrant client.
//
// This module integrates the Qdrant client into an Fx-based application by providing
// the client factory and registering its lifecycle hooks.
//
// The module:
//  1. Provides the NewQdrantClient factory function to the dependency injection container,
//     making the client available to other components.
//  2. Provides the NewEmbeddingsStore function, which wraps the client into a higher-level abstraction.
//  3. Invokes RegisterQdrantLifecycle to handle startup/shutdown of the client.
//
// Usage:
//
//	app := fx.New(
//	    qdrant.FXModule,
//	    // other modules...
//	)
//
// Dependencies required by this module:
// - A qdrant.Config instance must be available in the dependency injection container.
var FXModule = fx.Module("qdrant",
	fx.Provide(
		NewQdrantClient,
		NewEmbeddingsStore,
	),
	fx.Invoke(RegisterQdrantLifecycle),
)

// QdrantParams defines dependencies needed to construct the Qdrant client.
type QdrantParams struct {
	fx.In
	Config *Config
}

// RegisterQdrantLifecycle handles startup/shutdown of the Qdrant client.
// It ensures proper resource cleanup and logging.
func RegisterQdrantLifecycle(lc fx.Lifecycle, client *Client) {
	var once sync.Once

	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			log.Println("[Qdrant] client initialized successfully")
			return nil
		},
		OnStop: func(ctx context.Context) error {
			once.Do(func() {
				client.Close()
				log.Println("[Qdrant] client connection closed")
			})
			return nil
		},
	})
}
