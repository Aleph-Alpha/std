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

type Tracer struct {
	tracer *trace.TracerProvider
	logger Logger
}

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
