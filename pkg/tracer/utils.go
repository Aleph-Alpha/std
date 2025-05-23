package tracer

import (
	"context"
	"fmt"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	traceSpan "go.opentelemetry.io/otel/trace"
)

func RecordErrorOnSpan(span traceSpan.Span, err error) {
	span.RecordError(err)
	span.SetStatus(codes.Error, err.Error())
}

func (t *Tracer) StartSpan(ctx context.Context, name string) (context.Context, traceSpan.Span) {
	tracer := t.tracer.Tracer("")
	ctx, span := tracer.Start(ctx, name)
	return ctx, span
}

// SetAttributes adds one or more attributes to a span with support for different data types.
// It accepts a map where keys are attribute names and values can be different types.
//
// Parameters:
//   - span: The span to add attributes to
//   - attrs: A map of attribute keys to values. Values can be strings, ints, int64s,
//     float64s, or booleans.
//
// Example:
//
//	SetAttributes(span, map[string]interface{}{
//	    "user.id": "12345",
//	    "request.size": 1024,
//	    "process.duration_ms": 123.45,
//	    "request.cached": true,
//	})
func SetAttributes(span traceSpan.Span, attrs map[string]interface{}) {
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

func GetCarrier(ctx context.Context) map[string]string {
	propgator := propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{})
	carrier := propagation.MapCarrier{}
	propgator.Inject(ctx, carrier)
	return carrier
}

// SetCarrierOnContext extracts trace information from a carrier map and injects it into a context.
// This is the complement to GetCarrier and is typically used when receiving requests or messages
// from other services that include trace headers.
//
// Parameters:
//   - ctx: The base context to inject trace information into
//   - carrier: A map containing trace headers (like those from HTTP requests or message headers)
//
// Returns:
//   - A new context with the trace information from the carrier injected into it
//
// Example:
//
//	headers := request.Headers
//	tracedCtx := SetCarrierOnContext(ctx, headers)
func SetCarrierOnContext(ctx context.Context, carrier map[string]string) context.Context {
	propagator := propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{})
	return propagator.Extract(ctx, propagation.MapCarrier(carrier))
}
