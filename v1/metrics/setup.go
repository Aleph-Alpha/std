package metrics

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Metrics encapsulates the Prometheus registry and HTTP server responsible
// for exposing application metrics.
//
// This structure provides the components needed to register metrics collectors
// and serve them via the /metrics HTTP endpoint for Prometheus scraping.
type Metrics struct {
	// Server defines the HTTP server used to expose the /metrics endpoint.
	Server *http.Server

	// Registry is the Prometheus registry where all metrics are registered.
	// Each service maintains its own isolated registry to prevent metric name collisions.
	Registry *prometheus.Registry

	// Core built-in metrics
	requestsTotal   *prometheus.CounterVec
	requestDuration *prometheus.HistogramVec
	cpuUsageGauge   *prometheus.GaugeVec
}

// NewMetrics initializes and returns a new instance of the Metrics struct.
// It sets up a dedicated Prometheus registry, registers default system collectors,
// wraps all metrics with a constant `service` label, and creates an HTTP server
// exposing the /metrics endpoint.
//
// Parameters:
//   - cfg: Configuration for the metrics server, including listening address,
//     service name, and whether to enable default collectors.
//
// Returns:
//   - *Metrics: A configured Metrics instance ready for lifecycle management
//     and Fx module integration.
//
// The setup includes:
//   - A dedicated Prometheus registry for the service
//   - Automatic registration of Go, process, and build info collectors
//   - A global "service" label applied to all metrics for easier aggregation
//   - An HTTP server exposing the metrics endpoint
//
// Example:
//
//	cfg := metrics.Config{
//	    Address:              ":9090",
//	    ServiceName:           "document-index",
//	    EnableDefaultCollectors: true,
//	}
//	metricsInstance := metrics.NewMetrics(cfg)
//	go metricsInstance.Server.ListenAndServe()
//
// Access metrics at: http://localhost:9090/metrics
func NewMetrics(cfg Config) *Metrics {
	// Create a new isolated Prometheus registry for this service.
	// This avoids metric collisions when multiple services run in the same process.
	registry := prometheus.NewRegistry()

	// Wrap the registry with a constant label for consistent observability.
	// All metrics emitted by this service will automatically include the label:
	//   service="<cfg.ServiceName>"
	wrappedRegistry := prometheus.WrapRegistererWith(
		prometheus.Labels{"service": cfg.ServiceName},
		registry,
	)

	// Initialize the metrics struct
	m := &Metrics{
		Registry: registry,
	}

	// Define default metrics using helpers
	m.requestsTotal = createCounterVec("requests_total", "Total number of processed requests", []string{"status"})
	m.requestDuration = createHistogramVec("request_duration_seconds", "Duration of HTTP requests in seconds", []string{"endpoint"}, prometheus.DefBuckets)
	m.cpuUsageGauge = createGaugeVec("cpu_usage_percent", "Current CPU usage percentage per core", []string{"core"})

	// Register the metrics
	wrappedRegistry.MustRegister(
		m.requestsTotal,
		m.requestDuration,
		m.cpuUsageGauge,
	)

	// Register standard collectors if enabled.
	// These provide essential runtime metrics for Go processes:
	//   - GoCollector: Memory usage, goroutines, GC stats
	//   - ProcessCollector: CPU, file descriptors, memory stats
	//   - BuildInfoCollector: Binary version/build info
	if cfg.EnableDefaultCollectors {
		wrappedRegistry.MustRegister(
			collectors.NewGoCollector(),
			collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
			collectors.NewBuildInfoCollector(),
		)
	}

	// Create an HTTP handler that serves metrics from the registry.
	// The handler exposes metrics at /metrics for Prometheus scraping.
	handler := promhttp.HandlerFor(registry, promhttp.HandlerOpts{})

	// Configure the HTTP server for exposing metrics.
	server := &http.Server{
		Addr:    cfg.Address,
		Handler: handler,
	}

	// Return the fully configured metrics instance.
	m.Server = server
	return m
}
