package metrics

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Metrics encapsulates two separate Prometheus registries and HTTP servers:
// 1. System metrics (Go runtime, process, build info) - exposed on SystemServer
// 2. Application metrics (user-defined custom metrics) - exposed on ApplicationServer
//
// This separation allows different scrape configurations and access controls
// for system-level vs application-level observability.
type Metrics struct {
	// SystemServer defines the HTTP server for the /metrics endpoint exposing
	// Go runtime, process, and build info metrics.
	// Endpoint: SystemMetricsAddress (default: :9090)
	SystemServer *http.Server

	// ApplicationServer defines the HTTP server for the /metrics endpoint exposing
	// user-defined application metrics.
	// Endpoint: ApplicationMetricsAddress (default: :9091)
	ApplicationServer *http.Server

	// SystemRegistry is the Prometheus registry for system-level metrics
	// (Go runtime, process collectors, build info).
	SystemRegistry *prometheus.Registry

	// ApplicationRegistry is the Prometheus registry for user-defined metrics.
	// All metrics created via CreateCounter, CreateGauge, CreateHistogram, CreateSummary
	// are registered here.
	ApplicationRegistry *prometheus.Registry

	// wrappedApplicationRegisterer is the service-label-wrapped registerer used internally
	// for registering application metrics with automatic service label.
	wrappedApplicationRegisterer prometheus.Registerer
}

// NewMetrics initializes and returns a new instance of the Metrics struct.
// It sets up two separate Prometheus registries and HTTP servers:
//
// 1. System Metrics Endpoint (default: :9090):
//   - Go runtime metrics (goroutines, GC stats, heap usage)
//   - Process metrics (CPU time, memory, file descriptors)
//   - Build info metrics
//
// 2. Application Metrics Endpoint (default: :9091):
//   - User-defined metrics created via CreateCounter, CreateGauge, etc.
//   - No default metrics - fully controlled by the application
//
// Parameters:
//   - cfg: Configuration for the metrics servers, including addresses and service name
//
// Returns:
//   - *Metrics: A configured Metrics instance ready for lifecycle management
//     and Fx module integration
//
// Both registries automatically wrap all metrics with a constant `service` label
// for easier aggregation and filtering in multi-service environments.
//
// Example:
//
//	cfg := metrics.Config{
//	    SystemMetricsAddress:      ":9090",
//	    ApplicationMetricsAddress: ":9091",
//	    ServiceName:               "document-index",
//	}
//	metricsInstance := metrics.NewMetrics(cfg)
//	go metricsInstance.SystemServer.ListenAndServe()
//	go metricsInstance.ApplicationServer.ListenAndServe()
//
// Access metrics at:
//   - System metrics: http://localhost:9090/metrics
//   - Application metrics: http://localhost:9091/metrics
func NewMetrics(cfg Config) *Metrics {
	m := &Metrics{}

	// Determine system metrics address
	var systemAddr string
	if cfg.SystemMetricsAddress != nil {
		systemAddr = *cfg.SystemMetricsAddress
	} else {
		systemAddr = DefaultSystemMetricsAddress
	}

	// Setup system metrics registry and server (if not explicitly disabled)
	if systemAddr != "" {
		systemRegistry := prometheus.NewRegistry()

		// Wrap with service label
		wrappedSystemRegistry := prometheus.WrapRegistererWith(
			prometheus.Labels{"service": cfg.ServiceName},
			systemRegistry,
		)

		// Register standard system collectors
		wrappedSystemRegistry.MustRegister(
			collectors.NewGoCollector(),
			collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
			collectors.NewBuildInfoCollector(),
		)

		m.SystemRegistry = systemRegistry
		m.SystemServer = &http.Server{
			Addr:    systemAddr,
			Handler: promhttp.HandlerFor(systemRegistry, promhttp.HandlerOpts{}),
		}
	}

	// Determine application metrics address
	var appAddr string
	if cfg.ApplicationMetricsAddress != nil {
		appAddr = *cfg.ApplicationMetricsAddress
	} else {
		appAddr = DefaultApplicationMetricsAddress
	}

	// Setup application metrics registry and server (if not explicitly disabled)
	if appAddr != "" {
		applicationRegistry := prometheus.NewRegistry()

		// Wrap with service label for auto-labeling
		wrappedApplicationRegisterer := prometheus.WrapRegistererWith(
			prometheus.Labels{"service": cfg.ServiceName},
			applicationRegistry,
		)

		m.ApplicationRegistry = applicationRegistry
		m.wrappedApplicationRegisterer = wrappedApplicationRegisterer
		m.ApplicationServer = &http.Server{
			Addr:    appAddr,
			Handler: promhttp.HandlerFor(applicationRegistry, promhttp.HandlerOpts{}),
		}
	}

	return m
}
