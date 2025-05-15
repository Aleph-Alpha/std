package logger

import (
	"context"
	"go.uber.org/fx"
)

// FXModule defines the Fx module for the logger package.
var FXModule = fx.Module("logger",
	fx.Provide(
		NewLoggerClient,
	),
	fx.Invoke(RegisterLoggerLifecycle),
)

// RegisterLoggerLifecycle handles cleanup (sync) of the Zap logger.
func RegisterLoggerLifecycle(lc fx.Lifecycle, client *Logger) {
	lc.Append(fx.Hook{
		OnStop: func(ctx context.Context) error {
			return client.Zap.Sync() // flushes any buffered logs
		},
	})
}
