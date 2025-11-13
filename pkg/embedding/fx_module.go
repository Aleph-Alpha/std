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
		NewConfig,            // -> *Config
		NewInferenceProvider, // -> Provider
		NewClient,            // -> *Client
	),

	fx.Invoke(RegisterEmbeddingLifecycle),
)

// -------------------------------------------------------
// Lifecycle hook
// -------------------------------------------------------

func RegisterEmbeddingLifecycle(lc fx.Lifecycle, p Provider) {
	lc.Append(fx.Hook{
		OnStop: func(ctx context.Context) error {
			// Cleanup: close idle HTTP connections, stop goroutines, etc.
			if c, ok := p.(interface{ Close() error }); ok {
				return c.Close()
			}
			return nil
		},
	})
}
