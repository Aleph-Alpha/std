package tracer

import (
	"context"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
)

// Logger defines the interface for logging operations in the rabbit package.
// This interface allows the package to use any logging implementation that
// conforms to these methods.
//
//go:generate mockgen -source=setup.go -destination=mock_logger.go -package=tracer
type Logger interface {
	Info(msg string, err error, fields ...map[string]interface{})
	Debug(msg string, err error, fields ...map[string]interface{})
	Warn(msg string, err error, fields ...map[string]interface{})
	Error(msg string, err error, fields ...map[string]interface{})
	Fatal(msg string, err error, fields ...map[string]interface{})
}

// Tracer provides a simplified API for distributed tracing with OpenTelemetry.
// It wraps the OpenTelemetry TracerProvider and provides convenient methods for
// creating spans, recording errors, and propagating trace context across service boundaries.
//
// Tracer handles the complexity of trace context propagation, span creation, and attribute
// management, making it easier to implement distributed tracing in your applications.
//
// To use Tracer effectively:
// 1. Create spans for significant operations in your code
// 2. Record errors when operations fail
// 3. Add attributes to spans to provide context
// 4. Extract and inject trace context when crossing service boundaries
//
// The Tracer is designed to be thread-safe and can be shared across goroutines.
type Tracer struct {
	tracer *trace.TracerProvider
	logger Logger
}

// NewClient creates and initializes a new Tracer instance with OpenTelemetry.
// This function sets up the OpenTelemetry tracer provider with the provided configuration,
// configures trace exporters if enabled, and sets global OpenTelemetry settings.
//
// Parameters:
//   - cfg: Configuration for the tracer, including service name, environment, and export settings
//   - logger: Logger for recording initialization events and errors
//
// Returns:
//   - *Tracer: A configured Tracer instance ready for creating spans and managing trace context
//
// If trace export is enabled in the configuration, this function will set up an OTLP HTTP exporter
// that sends traces to the configured endpoint. If export fails to initialize, it will log a fatal error.
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
//	tracerClient := tracer.NewClient(cfg, logger)
//
//	// Use the tracer in your application
//	ctx, span := tracerClient.StartSpan(context.Background(), "process-request")
//	defer span.End()
func NewClient(cfg Config, logger Logger) *Tracer {
	var options []trace.TracerProviderOption

	if cfg.EnableExport {
		client := otlptracehttp.NewClient()
		exporter, err := otlptrace.New(context.Background(), client)
		if err != nil {
			logger.Fatal("cannot initiate tracer", err, nil)
			return nil
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

	return &Tracer{tracer: tp, logger: logger}
}
