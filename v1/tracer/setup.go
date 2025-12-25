package tracer

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
)

// TracerClient provides a simplified API for distributed tracing with OpenTelemetry.
// It wraps the OpenTelemetry TracerProvider and provides convenient methods for
// creating spans, recording errors, and propagating trace context across service boundaries.
//
// TracerClient handles the complexity of trace context propagation, span creation, and attribute
// management, making it easier to implement distributed tracing in your applications.
//
// To use TracerClient effectively:
// 1. Create spans for significant operations in your code
// 2. Record errors when operations fail
// 3. Add attributes to spans to provide context
// 4. Extract and inject trace context when crossing service boundaries
//
// The TracerClient is designed to be thread-safe and can be shared across goroutines.
// It implements the Tracer interface.
type TracerClient struct {
	tracer *trace.TracerProvider
}

// NewClient creates and initializes a new TracerClient instance with OpenTelemetry.
// This function sets up the OpenTelemetry tracer provider with the provided configuration,
// configures trace exporters if enabled, and sets global OpenTelemetry settings.
//
// Parameters:
//   - cfg: Configuration for the tracer, including service name, environment, and export settings
//
// Returns:
//   - *TracerClient: A configured TracerClient instance ready for creating spans and managing trace context
//
// If trace export is enabled in the configuration, this function will set up an OTLP HTTP exporter
// that sends traces to the configured endpoint. If export fails to initialize, it will return an error.
//
// The function also configures resource attributes for the service, including:
//   - Service name
//   - Deployment environment
//   - Environment tag
//
// Example:
//
//	cfg := tracer.Config{
//	    ServiceName: "user-service",
//	    AppEnv: "production",
//	    EnableExport: true,
//	}
//
//	tracerClient, err := tracer.NewClient(cfg)
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	// Use the tracer in your application
//	ctx, span := tracerClient.StartSpan(context.Background(), "process-request")
//	defer span.End()
func NewClient(cfg Config) (*TracerClient, error) {
	var options []trace.TracerProviderOption

	if cfg.EnableExport {
		client := otlptracehttp.NewClient()
		exporter, err := otlptrace.New(context.Background(), client)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize OTLP exporter: %w", err)
		}
		options = append(options, trace.WithBatcher(exporter))
	}

	options = append(options, trace.WithResource(resource.NewWithAttributes(
		semconv.SchemaURL,
		semconv.ServiceName(cfg.ServiceName),
		semconv.DeploymentEnvironment(cfg.AppEnv),
		attribute.String("environment", cfg.AppEnv),
	)))

	tp := trace.NewTracerProvider(options...)

	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))

	return &TracerClient{tracer: tp}, nil
}
