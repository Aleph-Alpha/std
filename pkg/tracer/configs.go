package tracer

// Config defines the configuration for the OpenTelemetry tracer.
// It controls service identification, environment settings, and whether
// traces should be exported to an observability backend.
type Config struct {
	// ServiceName specifies the name of the service using this tracer.
	// This field is required and will appear in traces to identify the service
	// that generated the spans. It should be a descriptive, stable name that
	// uniquely identifies your service in your system architecture.
	//
	// Example values: "user-service", "payment-processor", "notification-worker"
	ServiceName string

	// AppEnv indicates the deployment environment where the service is running.
	// This helps separate traces from different environments in your observability system.
	// Common values include "development", "staging", "production".
	//
	// This field is used to set the "deployment.environment" and "environment"
	// resource attributes on all spans.
	AppEnv string

	// EnableExport controls whether traces are exported to an observability backend.
	// When set to true, the tracer will configure an OTLP HTTP exporter to send
	// traces to a collector. When false, traces are only kept in memory and not exported.
	//
	// In development environments, you might set this to false to avoid unnecessary
	// traffic to your observability system. In production, this should typically be
	// set to true to capture all telemetry data.
	//
	// Note that even when this is false, tracing is still functional for request context
	// propagation; spans just won't be sent to external systems.
	EnableExport bool
}
