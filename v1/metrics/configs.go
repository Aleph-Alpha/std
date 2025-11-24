package metrics

// Default port for metrics server if none is specified.
const DefaultMetricsAddress = ":9090"

// Config defines the configuration structure for the Prometheus metrics server.
// It contains settings that control how metrics are exposed and collected.
type Config struct {
	// Address determines the network address where the Prometheus
	// metrics HTTP server listens.
	//
	// Example values:
	//   - ":9090"   → Listen on all interfaces, port 9090
	//   - "127.0.0.1:9100" → Listen only on localhost, port 9100
	//
	// This setting can be configured via:
	//   - YAML configuration with the "address" key
	//   - Environment variable METRICS_ADDRESS
	//
	// Default: ":9090"
	Address string `yaml:"address" envconfig:"METRICS_ADDRESS"`

	// EnableDefaultCollectors controls whether the built-in Go runtime
	// and process metrics are automatically registered.
	//
	// When true, metrics such as goroutine count, GC stats, and CPU usage
	// will be included automatically. Disable only if you want full
	// manual control over registered collectors.
	//
	// This setting can be configured via:
	//   - YAML configuration with the "enable_default_collectors" key
	//   - Environment variable METRICS_ENABLE_DEFAULT_COLLECTORS
	//
	// Default: true
	EnableDefaultCollectors bool `yaml:"enable_default_collectors" envconfig:"METRICS_ENABLE_DEFAULT_COLLECTORS"`

	// Namespace sets a global prefix for all metrics registered by this service.
	// Useful when running multiple services in the same Prometheus cluster.
	//
	// Example:
	//   Namespace: "pharia_data"
	//   → Metric name becomes "pharia_data_http_requests_total"
	//
	// This setting can be configured via:
	//   - YAML configuration with the "namespace" key
	//   - Environment variable METRICS_NAMESPACE
	Namespace string `yaml:"namespace" envconfig:"METRICS_NAMESPACE"`

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
