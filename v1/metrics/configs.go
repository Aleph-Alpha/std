package metrics

// Default addresses for metrics servers if none is specified.
const (
	DefaultSystemMetricsAddress      = ":9090"
	DefaultApplicationMetricsAddress = ":9091"
)

// Config defines the configuration structure for the Prometheus metrics servers.
// It contains settings that control how metrics are exposed and collected.
//
// The package provides two separate metrics endpoints:
// 1. System Metrics Endpoint (default: :9090): Exposes Go runtime, process, and build info metrics
// 2. Application Metrics Endpoint (default: :9091): Exposes user-defined application-specific metrics
type Config struct {
	// SystemMetricsAddress determines the network address where the system
	// metrics HTTP server listens. This endpoint exposes Go runtime,
	// process, and build info metrics.
	//
	// Example values:
	//   - ":9090"   → Listen on all interfaces, port 9090
	//   - "127.0.0.1:9090" → Listen only on localhost, port 9090
	//   - nil (or omitted) → Use default ":9090"
	//
	// To disable the system metrics endpoint, use an empty string pointer:
	//   SystemMetricsAddress: ptr(""),
	//
	// This setting can be configured via:
	//   - YAML configuration with the "system_metrics_address" key
	//   - Environment variable METRICS_SYSTEM_ADDRESS
	//
	// Default: ":9090"
	SystemMetricsAddress *string `yaml:"system_metrics_address" envconfig:"METRICS_SYSTEM_ADDRESS"`

	// ApplicationMetricsAddress determines the network address where the
	// application metrics HTTP server listens. This endpoint exposes
	// user-defined custom metrics created via CreateCounter, CreateGauge, etc.
	//
	// Example values:
	//   - ":9091"   → Listen on all interfaces, port 9091
	//   - "127.0.0.1:9091" → Listen only on localhost, port 9091
	//   - nil (or omitted) → Use default ":9091"
	//
	// To disable the application metrics endpoint, use an empty string pointer:
	//   ApplicationMetricsAddress: ptr(""),
	//
	// This setting can be configured via:
	//   - YAML configuration with the "application_metrics_address" key
	//   - Environment variable METRICS_APPLICATION_ADDRESS
	//
	// Default: ":9091"
	ApplicationMetricsAddress *string `yaml:"application_metrics_address" envconfig:"METRICS_APPLICATION_ADDRESS"`

	// ServiceName identifies the service exposing metrics.
	// This is used as a common label in all metrics to help
	// distinguish metrics between services in multi-tenant deployments.
	//
	// Example:
	//   ServiceName: "document-index"
	//   → metrics include label service="document-index"
	//
	// This setting can be configured via:
	//   - YAML configuration with the "service_name" key
	//   - Environment variable METRICS_SERVICE_NAME
	ServiceName string `yaml:"service_name" envconfig:"METRICS_SERVICE_NAME"`
}

// Ptr returns a pointer to the given string value.
// Helper function for disabling endpoints in configuration.
//
// Example:
//
//	cfg := metrics.Config{
//	    SystemMetricsAddress: metrics.Ptr(""),     // Explicitly disable
//	    ApplicationMetricsAddress: nil,            // Use default
//	    ServiceName: "my-service",
//	}
func Ptr(s string) *string {
	return &s
}
