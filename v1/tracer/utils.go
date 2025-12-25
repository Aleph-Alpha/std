package tracer

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	traceSpan "go.opentelemetry.io/otel/trace"
)

// spanImpl is an internal implementation of the Span interface
// that wraps an OpenTelemetry span. This type handles the conversion
// between the simplified API and the underlying OpenTelemetry functionality.
type spanImpl struct {
	span traceSpan.Span
}

// End implements the Span interface by ending the underlying OpenTelemetry span.
// This method should be called when the operation being traced is complete,
// typically using a deferred call immediately after span creation.
//
// When End is called:
// - The span's end time is recorded
// - The span is marked as complete
// - The span data is submitted to configured exporters
// - Any resources associated with the span are released
//
// After calling End, no further operations should be performed on the span,
// as its data has already been finalized and submitted for processing.
//
// Example usage:
//
//	ctx, span := tracer.StartSpan(ctx, "operation-name")
//	defer span.End() // Ensures the span is properly ended when the function returns
func (s *spanImpl) End() {
	s.span.End()
}

// SetAttributes implements the Span interface by adding attributes to the span.
// Attributes provide additional context about the operation being traced,
// making it easier to filter, analyze, and understand trace data.
//
// This method accepts a map of key-value pairs and automatically handles
// type conversion for common Go types:
// - string: Stored as string attributes
// - int/int64: Stored as integer attributes
// - float64: Stored as floating-point attributes
// - bool: Stored as boolean attributes
// - Other types: Converted to strings using fmt.Sprint
//
// If the attributes map is empty, the method returns immediately without
// making any changes to the span.
//
// The method converts the Go values to OpenTelemetry attribute values
// internally, handling all the necessary type conversions.
//
// Example usage:
//
//	span.SetAttributes(map[string]interface{}{
//	    "user.id": "usr_12345",
//	    "request.size_bytes": 1024,
//	    "items.count": 5,
//	    "checkout.total": 29.99,
//	    "premium.customer": true,
//	    "cart.items": cartItems, // Complex type automatically converted to string
//	})
func (s *spanImpl) SetAttributes(attrs map[string]interface{}) {
	if len(attrs) == 0 {
		return
	}

	attributes := make([]attribute.KeyValue, 0, len(attrs))

	for k, v := range attrs {
		switch val := v.(type) {
		case string:
			attributes = append(attributes, attribute.String(k, val))
		case int:
			attributes = append(attributes, attribute.Int(k, val))
		case int64:
			attributes = append(attributes, attribute.Int64(k, val))
		case float64:
			attributes = append(attributes, attribute.Float64(k, val))
		case bool:
			attributes = append(attributes, attribute.Bool(k, val))
		default:
			// For unsupported types, convert to string
			attributes = append(attributes, attribute.String(k, fmt.Sprint(val)))
		}
	}

	s.span.SetAttributes(attributes...)
}

// RecordError implements the Span interface by recording an error on the span.
// This method performs two important actions:
// 1. Records the error event on the span with its details
// 2. Sets the span's status to Error with the error message as the description
//
// Recording errors on spans provides crucial context for debugging and
// monitoring, making it clear which operations failed and why. This
// information appears in trace visualizations and can be used for
// filtering and alerting.
//
// The error's message is used as the status description, providing immediate
// visibility into what went wrong without needing to examine the error events.
//
// This method should be called whenever an error occurs during the operation
// represented by the span, typically just before returning the error to the caller.
//
// Example usage:
//
//	result, err := database.Query(ctx, queryString)
//	if err != nil {
//	    span.RecordError(err) // Record the error before returning it
//	    return nil, fmt.Errorf("database query failed: %w", err)
//	}
//
// In trace visualizations, spans with recorded errors are typically highlighted,
// making it easy to identify problematic operations at a glance.
func (s *spanImpl) RecordError(err error) {
	s.span.RecordError(err)
	s.span.SetStatus(codes.Error, err.Error())
}

// StartSpan creates a new span with the given name and returns an updated context
// containing the span, along with a Span interface. This is the primary method for
// creating spans to trace operations in your application.
//
// The created span becomes a child of any span that exists in the provided context.
// If no span exists in the context, a new root span is created. This automatically
// builds a hierarchy of spans that reflects the structure of your application's operations.
//
// Parameters:
//   - ctx: The parent context, which may contain a parent span
//   - name: A descriptive name for the operation being traced (e.g., "http-request", "db-query")
//
// Returns:
//   - context.Context: A new context containing the created span
//   - Span: An interface to interact with the span, which must be ended when the operation completes
//
// Best practices:
//   - Use descriptive, consistent naming conventions for spans
//   - Create spans for operations that are significant for performance or debugging
//   - For operations with sub-operations, create a parent span for the overall operation
//     and child spans for each sub-operation
//   - Always defer span.End() immediately after creating the span
//
// Example:
//
//	func processRequest(ctx context.Context, req Request) (Response, error) {
//	    // Create a span for this operation
//	    ctx, span := tracer.StartSpan(ctx, "process-request")
//	    // Ensure the span is ended when the function returns
//	    defer span.End()
//
//	    // Add context information to the span
//	    span.SetAttributes(map[string]interface{}{
//	        "request.id": req.ID,
//	        "request.type": req.Type,
//	        "user.id": req.UserID,
//	    })
//
//	    // Perform the operation, using the context with the span
//	    result, err := performWork(ctx, req)
//	    if err != nil {
//	        // Record the error on the span
//	        span.RecordError(err)
//	        return Response{}, err
//	    }
//
//	    return result, nil
//	}
func (t *TracerClient) StartSpan(ctx context.Context, name string) (context.Context, Span) {
	tracer := t.tracer.Tracer("")
	ctx, otSpan := tracer.Start(ctx, name)

	span := &spanImpl{
		span: otSpan,
	}

	return ctx, span
}

// GetCarrier extracts the current trace context from a context object and returns it as
// a map that can be transmitted across service boundaries. This is essential for distributed
// tracing to maintain trace continuity across different services.
//
// This method extracts W3C Trace Context headers, which follow the standard format for
// distributed tracing propagation. Using standardized headers ensures compatibility with
// other services and observability tools in your infrastructure that support the W3C
// Trace Context specification.
//
// When a trace spans multiple services (e.g., a web service calling a database service),
// the trace context must be passed between these services to maintain a unified trace.
// This method facilitates this by extracting that context into a format that can be
// added to HTTP headers, message queue properties, or other inter-service communication.
//
// Parameters:
//   - ctx: The context containing the current trace information
//
// Returns:
//   - map[string]string: A map containing the trace context headers
//
// The returned map typically includes:
//   - "traceparent": Contains trace ID, span ID, and trace flags
//   - "tracestate": Contains vendor-specific trace information (if present)
//
// Example:
//
//	// Extract trace context for an outgoing HTTP request
//	func makeHttpRequest(ctx context.Context, url string) (*http.Response, error) {
//	    req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
//	    if err != nil {
//	        return nil, err
//	    }
//
//	    // Get trace headers from context
//	    traceHeaders := tracer.GetCarrier(ctx)
//
//	    // Add trace headers to the outgoing request
//	    for key, value := range traceHeaders {
//	        req.Header.Set(key, value)
//	    }
//
//	    // Make the request with trace context included
//	    return http.DefaultClient.Do(req)
//	}
func (t *TracerClient) GetCarrier(ctx context.Context) map[string]string {
	propagator := propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{})
	carrier := propagation.MapCarrier{}
	propagator.Inject(ctx, carrier)
	return carrier
}

// SetCarrierOnContext extracts trace information from a carrier map and injects it into a context.
// This is the complement to GetCarrier and is typically used when receiving requests or messages
// from other services that include trace headers.
//
// This method is crucial for maintaining distributed trace continuity across service boundaries.
// When a service receives a request from another service that includes trace context headers,
// this method allows the receiving service to continue the existing trace rather than starting
// a new one. This creates a complete end-to-end view of the request's journey through your
// distributed system.
//
// The method works with standard W3C Trace Context headers, ensuring compatibility across
// different services and observability systems. It handles extracting trace IDs, span IDs,
// and other trace context information from the carrier map and properly initializing the
// context for continuation of the trace.
//
// Parameters:
//   - ctx: The base context to inject trace information into
//   - carrier: A map containing trace headers (like those from HTTP requests or message headers)
//
// Returns:
//   - context.Context: A new context with the trace information from the carrier injected into it
//
// The carrier map typically contains the following trace context headers:
//   - "traceparent": Contains trace ID, span ID, and trace flags
//   - "tracestate": Contains vendor-specific trace information (if present)
//
// Example:
//
//	// Extract trace context from an incoming HTTP request
//	func httpHandler(w http.ResponseWriter, r *http.Request) {
//	    // Extract headers from the request
//	    headers := make(map[string]string)
//	    for key, values := range r.Header {
//	        if len(values) > 0 {
//	            headers[key] = values[0]
//	        }
//	    }
//
//	    // Create a context with the trace information
//	    ctx := tracer.SetCarrierOnContext(r.Context(), headers)
//
//	    // Use this traced context for processing the request
//	    // Any spans created with this context will be properly connected to the trace
//	    result, err := processRequest(ctx, r)
//	    // ...
//	}
func (t *TracerClient) SetCarrierOnContext(ctx context.Context, carrier map[string]string) context.Context {
	propagator := propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{})
	return propagator.Extract(ctx, propagation.MapCarrier(carrier))
}
