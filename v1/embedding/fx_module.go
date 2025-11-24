package embedding

import (
	"context"

	"go.uber.org/fx"
)

// FXModule wires the embedding system into Fx.
//
// It provides:
//   - Config                 (NewConfig)
//   - Provider               (via provider factory)
//   - *Client                (NewClient)
//   - Lifecycle hook         (RegisterEmbeddingLifecycle)
var FXModule = fx.Module(
	"embedding",

	fx.Provide(
		NewConfig, // -> *Config
		NewClient, // -> *Client
	),

	fx.Invoke(RegisterEmbeddingLifecycle),
)

// -------------------------------------------------------
// Lifecycle hook
// -------------------------------------------------------

// RegisterEmbeddingLifecycle ensures that the Client (and its provider)
// are properly cleaned up on application shutdown.
func RegisterEmbeddingLifecycle(lc fx.Lifecycle, client *Client) {
	lc.Append(fx.Hook{
		OnStop: func(ctx context.Context) error {
			return client.Close()
		},
	})
}
