// Package tracer provides distributed tracing functionality using OpenTelemetry.
//
// The tracer package offers a simplified interface for implementing distributed tracing
// in Go applications. It abstracts away the complexity of OpenTelemetry to provide
// a clean, easy-to-use API for creating and managing trace spans.
//
// # Architecture
//
// The package follows the "accept interfaces, return structs" Go idiom:
//   - Tracer interface: Defines the contract for tracing operations
//   - TracerClient struct: Concrete implementation of the Tracer interface
//   - Span interface: Defines the contract for span operations
//   - Constructor returns *TracerClient (concrete type)
//   - FX module provides both *TracerClient and Tracer interface
//
// This design allows:
//   - Direct usage: Use *TracerClient for simple cases
//   - Interface usage: Depend on Tracer interface for testability and flexibility
//   - Zero adapters needed: Consumer code can use type aliases
//
// Core Features:
//   - Simple span creation and management
//   - Error recording and status tracking
//   - Customizable span attributes
//   - Cross-service trace context propagation
//   - Integration with OpenTelemetry backends
//
// # Basic Usage (Direct)
//
//	import (
//		"context"
//		"github.com/Aleph-Alpha/std/v1/tracer"
//	)
//
//	// Create a tracer (returns concrete *TracerClient)
//	tracerClient, err := tracer.NewClient(tracer.Config{
//		ServiceName:  "my-service",
//		AppEnv:       "development",
//		EnableExport: true,
//	})
//	if err != nil {
//		log.Fatal(err)
//	}
//
//	// Create a span
//	ctx, span := tracerClient.StartSpan(ctx, "process-request")
//	defer span.End()
//
//	// Add attributes to the span
//	span.SetAttributes(map[string]interface{}{
//		"user.id": "123",
//		"request.id": "abc-xyz",
//	})
//
//	// Record errors
//	if err != nil {
//		span.RecordError(err)
//		return err
//	}
//
// # FX Module Integration
//
// The package provides an FX module that injects both concrete and interface types:
//
//	import (
//		"github.com/Aleph-Alpha/std/v1/tracer"
//		"go.uber.org/fx"
//	)
//
//	app := fx.New(
//		tracer.FXModule,
//		fx.Provide(
//			func() tracer.Config {
//				return tracer.Config{
//					ServiceName:  "my-service",
//					AppEnv:       "production",
//					EnableExport: true,
//				}
//			},
//		),
//		fx.Invoke(func(t tracer.Tracer) {
//			// Use the Tracer interface
//			ctx, span := t.StartSpan(context.Background(), "app-startup")
//			defer span.End()
//		}),
//	)
//	app.Run()
//
// # Using Type Aliases (Recommended for Consumer Code)
//
// Consumer applications can use type aliases to avoid creating adapters:
//
//	// In your application's observability package
//	package observability
//
//	import stdTracer "github.com/Aleph-Alpha/std/v1/tracer"
//
//	// Type aliases reference std interfaces directly
//	type Tracer = stdTracer.Tracer
//	type Span = stdTracer.Span
//
// Then use these aliases throughout your application:
//
//	func MyService(tracer observability.Tracer) {
//		ctx, span := tracer.StartSpan(ctx, "my-operation")
//		defer span.End()
//	}
//
// # Distributed Tracing Across Services
//
//	// In the sending service
//	ctx, span := tracer.StartSpan(ctx, "send-request")
//	defer span.End()
//
//	// Extract trace context for an outgoing HTTP request
//	traceHeaders := tracer.GetCarrier(ctx)
//	for key, value := range traceHeaders {
//		req.Header.Set(key, value)
//	}
//
//	// In the receiving service
//	func httpHandler(w http.ResponseWriter, r *http.Request) {
//		// Extract headers from the request
//		headers := make(map[string]string)
//		for key, values := range r.Header {
//			if len(values) > 0 {
//				headers[key] = values[0]
//			}
//		}
//
//		// Create a context with the trace information
//		ctx := tracer.SetCarrierOnContext(r.Context(), headers)
//
//		// Create a child span in this service
//		ctx, span := tracer.StartSpan(ctx, "handle-request")
//		defer span.End()
//		// ...
//	}
//
// # Best Practices
//
//   - Create spans for significant operations in your code
//   - Always defer span.End() immediately after creating a span
//   - Use descriptive span names that identify the operation
//   - Add relevant attributes to provide context
//   - Record errors when operations fail
//   - Ensure trace context is properly propagated between services
//   - Prefer interface types (tracer.Tracer) in function signatures
//   - Use concrete types (*tracer.TracerClient) when you need specific implementation details
//
// # Thread Safety
//
// All methods on the TracerClient type and Span interface are safe for concurrent use
// by multiple goroutines.
package tracer
