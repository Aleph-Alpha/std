package tracer

import (
	"context"
	"log"

	"go.uber.org/fx"
)

// FXModule provides a Uber FX module that configures distributed tracing for your application.
// This module registers the tracer client with the dependency injection system and
// sets up proper lifecycle management to ensure graceful startup and shutdown of the tracer.
//
// The module:
// 1. Provides the tracer client through the NewClient constructor
// 2. Registers shutdown hooks to cleanly close tracer resources on application termination
//
// Usage:
//
//	app := fx.New(
//	    tracer.FXModule,
//	    // other modules...
//	)
//	app.Run()
//
// This module should be included in your main application to enable distributed tracing
// throughout your dependency graph without manual wiring.
var FXModule = fx.Module("tracer",
	fx.Provide(
		NewClient,
	),
	fx.Invoke(RegisterTracerLifecycle),
)

// RegisterTracerLifecycle registers shutdown hooks for the tracer with the FX lifecycle.
// This function ensures that tracer resources are properly released when the application
// terminates, preventing resource leaks and ensuring traces are flushed to exporters.
//
// Parameters:
//   - lc: The FX lifecycle to register hooks with
//   - tracer: The tracer instance to manage lifecycle for
//
// The function registers an OnStop hook that:
// 1. Logs that the tracer is shutting down
// 2. Gracefully shuts down the tracer provider
// 3. Handles edge cases where the tracer might be nil
//
// This function is automatically invoked by the FXModule and normally doesn't need
// to be called directly.
//
// Example of how this works in the FX application lifecycle:
//
//	app := fx.New(
//	    tracer.FXModule,
//	    // When app.Stop() is called or the application receives a termination signal:
//	    // 1. The OnStop hook registered by RegisterTracerLifecycle is called
//	    // 2. The tracer is gracefully shut down, flushing any pending spans
//	)
func RegisterTracerLifecycle(lc fx.Lifecycle, tracer *Tracer) {
	lc.Append(fx.Hook{
		OnStop: func(ctx context.Context) error {
			log.Println("INFO: shutting down tracer...")
			if tracer.tracer == nil {
				log.Println("INFO: tracer is nil, skipping shutdown")
				return nil
			}
			return tracer.tracer.Shutdown(ctx)
		},
	})
}
