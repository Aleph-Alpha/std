package logger

import (
	"context"

	"go.uber.org/fx"
)

// FXModule defines the Fx module for the logger package.
// This module integrates the logger into an Fx-based application by providing
// the logger factory and registering its lifecycle hooks.
//
// The module provides:
// 1. *LoggerClient (concrete type) for direct use
// 2. Logger interface for dependency injection
// 3. Lifecycle management for proper cleanup
//
// Usage:
//
//	app := fx.New(
//	    logger.FXModule,
//	    // other modules...
//	)
//
// Dependencies required by this module:
// - A logger.Config instance must be available in the dependency injection container
var FXModule = fx.Module("logger",
	fx.Provide(
		NewLoggerClient, // Provides *LoggerClient
		// Also provide the Logger interface
		fx.Annotate(
			func(l *LoggerClient) Logger { return l },
			fx.As(new(Logger)),
		),
	),
	fx.Invoke(RegisterLoggerLifecycle),
)

// RegisterLoggerLifecycle handles cleanup (sync) of the Zap logger.
// This function registers a shutdown hook with the Fx lifecycle system that
// ensures any buffered log entries are flushed when the application terminates.
//
// Parameters:
//   - lc: The Fx lifecycle controller
//   - client: The logger instance to be managed
//
// The lifecycle hook:
//   - OnStop: Calls Sync() on the underlying Zap logger to flush any buffered logs
//     to their output destinations before the application terminates
//
// This ensures that no log entries are lost if the application shuts down while
// logs are still buffered in memory.
//
// Note: This function is automatically invoked by the FXModule and does not need
// to be called directly in application code.
func RegisterLoggerLifecycle(lc fx.Lifecycle, client *LoggerClient) {
	lc.Append(fx.Hook{
		OnStop: func(ctx context.Context) error {
			return client.Zap.Sync() // flushes any buffered logs
		},
	})
}
