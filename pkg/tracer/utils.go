package tracer

import (
	"context"
	"fmt"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	traceSpan "go.opentelemetry.io/otel/trace"
)

// RecordErrorOnSpan records an error on a span and sets its status to error.
// This method is used to indicate that a span represents a failed operation,
// which helps with error tracing and monitoring in observability systems.
//
// Parameters:
//   - span: The span on which to record the error
//   - err: The error to record on the span
//
// Example:
//
//	ctx, span := tracer.StartSpan(ctx, "fetch-user-data")
//	defer span.End()
//
//	data, err := fetchUserData(ctx, userID)
//	if err != nil {
//	    // Record the error on the span to track it in traces
//	    tracer.RecordErrorOnSpan(span, err)
//	    return nil, err
//	}
//
//	return data, nil
func (t *Tracer) RecordErrorOnSpan(span traceSpan.Span, err error) {
	span.RecordError(err)
	span.SetStatus(codes.Error, err.Error())
}

// StartSpan creates a new span with the given name and returns an updated context
// containing the span, along with the span itself. This is the primary method for
// creating spans to trace operations in your application.
//
// The created span becomes a child of any span that exists in the provided context.
// If no span exists in the context, a new root span is created.
//
// Parameters:
//   - ctx: The parent context, which may contain a parent span
//   - name: A descriptive name for the operation being traced
//
// Returns:
//   - context.Context: A new context containing the created span
//   - traceSpan.Span: The created span, which must be ended when the operation completes
//
// Example:
//
//	func processRequest(ctx context.Context, req Request) (Response, error) {
//	    // Create a span for this operation
//	    ctx, span := tracer.StartSpan(ctx, "process-request")
//	    // Ensure the span is ended when the function returns
//	    defer span.End()
//
//	    // Perform the operation, using the context with the span
//	    result, err := performWork(ctx, req)
//	    if err != nil {
//	        tracer.RecordErrorOnSpan(span, err)
//	        return Response{}, err
//	    }
//
//	    return result, nil
//	}
func (t *Tracer) StartSpan(ctx context.Context, name string) (context.Context, traceSpan.Span) {
	tracer := t.tracer.Tracer("")
	ctx, span := tracer.Start(ctx, name)
	return ctx, span
}

// SetAttributes adds one or more attributes to a span with support for different data types.
// Attributes provide additional context and metadata for spans, making traces more informative
// for debugging and analysis.
//
// Parameters:
//   - span: The span to add attributes to
//   - attrs: A map of attribute keys to values. Values can be strings, ints, int64s,
//     float64s, or booleans. Other types are converted to strings.
//
// Supported value types:
//   - string: Stored as string attributes
//   - int/int64: Stored as integer attributes
//   - float64: Stored as floating-point attributes
//   - bool: Stored as boolean attributes
//   - other types: Converted to strings using fmt.Sprint
//
// Example:
//
//	ctx, span := tracer.StartSpan(ctx, "process-payment")
//	defer span.End()
//
//	// Add attributes to provide context about the operation
//	tracer.SetAttributes(span, map[string]interface{}{
//	    "user.id": userID,
//	    "payment.amount": amount,
//	    "payment.currency": "USD",
//	    "payment.method": "credit_card",
//	    "request.id": requestID,
//	})
//
//	// Perform the payment processing...
func (t *Tracer) SetAttributes(span traceSpan.Span, attrs map[string]interface{}) {
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

	span.SetAttributes(attributes...)
}

// GetCarrier extracts the current trace context from a context object and returns it as
// a map that can be transmitted across service boundaries. This is essential for distributed
// tracing to maintain trace continuity across different services.
//
// The returned map contains W3C Trace Context headers that follow the standard format for
// distributed tracing, making it compatible with other services that support the W3C
// Trace Context specification.
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
func (t *Tracer) GetCarrier(ctx context.Context) map[string]string {
	propagator := propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{})
	carrier := propagation.MapCarrier{}
	propagator.Inject(ctx, carrier)
	return carrier
}

// SetCarrierOnContext extracts trace information from a carrier map and injects it into a context.
// This is the complement to GetCarrier and is typically used when receiving requests or messages
// from other services that include trace headers.
//
// This method is crucial for maintaining distributed trace continuity across service boundaries
// by ensuring that spans created in this service are properly connected to spans from upstream services.
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
//	    result, err := processRequest(ctx, r)
//	    // ...
//	}
func (t *Tracer) SetCarrierOnContext(ctx context.Context, carrier map[string]string) context.Context {
	propagator := propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{})
	return propagator.Extract(ctx, propagation.MapCarrier(carrier))
}
