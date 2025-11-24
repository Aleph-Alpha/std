// Package tracer provides distributed tracing functionality using OpenTelemetry.
//
// The tracer package offers a simplified interface for implementing distributed tracing
// in Go applications. It abstracts away the complexity of OpenTelemetry to provide
// a clean, easy-to-use API for creating and managing trace spans.
//
// Core Features:
//   - Simple span creation and management
//   - Error recording and status tracking
//   - Customizable span attributes
//   - Cross-service trace context propagation
//   - Integration with OpenTelemetry backends
//
// Basic Usage:
//
//	import (
//		"context"
//		"github.com/Aleph-Alpha/std/pkg/tracer"
//		"github.com/Aleph-Alpha/std/pkg/logger"
//	)
//
//	// Create a logger
//	log, _ := logger.NewLogger(logger.Config{Level: "info"})
//
//	// Create a tracer
//	tracerClient := tracer.NewClient(tracer.Config{
//		ServiceName:  "my-service",
//		AppEnv:       "development",
//		EnableExport: true,
//	}, log)
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
//		return nil, err
//	}
//
// Distributed Tracing Across Services:
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
// FX Module Integration:
//
// This package provides an fx module for easy integration:
//
//	app := fx.New(
//		logger.Module,
//		tracer.Module,
//		// ... other modules
//	)
//	app.Run()
//
// Best Practices:
//
//   - Create spans for significant operations in your code
//   - Always defer span.End() immediately after creating a span
//   - Use descriptive span names that identify the operation
//   - Add relevant attributes to provide context
//   - Record errors when operations fail
//   - Ensure trace context is properly propagated between services
//
// Thread Safety:
//
// All methods on the Tracer type and Span interface are safe for concurrent use
// by multiple goroutines.
package tracer
