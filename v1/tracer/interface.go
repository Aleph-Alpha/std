package tracer

import (
	"context"
)

// Tracer provides distributed tracing capabilities for applications.
// It wraps OpenTelemetry functionality with a simplified interface
// for creating spans, recording errors, and propagating trace context.
//
// This interface is implemented by the concrete *Tracer type.
type Tracer interface {
	// StartSpan creates a new span with the given name.
	// The span is automatically attached to the parent span in the context (if any).
	// Returns a new context with the span and the span itself.
	// Always call span.End() when the operation completes (typically via defer).
	StartSpan(ctx context.Context, name string) (context.Context, Span)

	// GetCarrier extracts trace context from the given context as a map of headers.
	// Use this when making outbound HTTP requests or sending messages to other services
	// to propagate the trace context.
	GetCarrier(ctx context.Context) map[string]string

	// SetCarrierOnContext injects trace context from headers into the given context.
	// Use this when receiving HTTP requests or messages from other services
	// to continue the trace.
	SetCarrierOnContext(ctx context.Context, carrier map[string]string) context.Context
}

// Span represents a trace span for tracking operations in distributed systems.
// It provides methods for ending the span, recording errors, and setting attributes.
//
// The Span interface abstracts the underlying OpenTelemetry implementation details,
// providing a clean API for application code to interact with spans without
// direct dependencies on the tracing library.
//
// Spans represent a single operation or unit of work in your application. They form
// a hierarchy where a parent span can have multiple child spans, creating a trace
// that shows the flow of operations and their relationships.
//
// To use a span effectively:
// 1. Always call End() when the operation completes (typically with defer)
// 2. Add attributes that provide context about the operation
// 3. Record any errors that occur during the operation
//
// Spans created with StartSpan() automatically inherit the parent span from the context
// if one exists, creating a proper span hierarchy.
type Span interface {
	// End completes the span and sends it to configured exporters.
	// End should be called when the operation being traced is complete.
	// It's recommended to defer this call immediately after obtaining the span.
	//
	// Example:
	//   ctx, span := tracer.StartSpan(ctx, "operation-name")
	//   defer span.End()
	End()

	// SetAttributes adds key-value pairs of attributes to the span.
	// These attributes provide additional context about the operation.
	// The method supports various data types including strings, integers,
	// floating-point numbers, and booleans.
	//
	// Example:
	//   span.SetAttributes(map[string]interface{}{
	//     "user.id": userID,
	//     "request.size": size,
	//     "request.type": "background",
	//     "priority": 3,
	//     "retry.enabled": true,
	//   })
	SetAttributes(attrs map[string]interface{})

	// RecordError marks the span as having encountered an error and
	// records the error information within the span. This helps with
	// error tracing and debugging by clearly identifying which spans
	// represent failed operations.
	//
	// Example:
	//   result, err := database.Query(ctx, query)
	//   if err != nil {
	//     span.RecordError(err)
	//     return nil, err
	//   }
	RecordError(err error)
}
