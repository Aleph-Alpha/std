package metrics

// MetricsCollector provides an interface for collecting and exposing application metrics.
// It abstracts metric operations with support for counters, histograms, gauges, and summaries.
//
// This interface is implemented by the concrete *Metrics type and does not expose any
// Prometheus-specific types, allowing for potential alternative implementations or testing mocks.
//
// All metrics created through this interface are registered to the application metrics
// registry and exposed via the application metrics endpoint (default: :9091).
type MetricsCollector interface {
	// CreateCounter creates a new counter metric and registers it to the
	// application metrics registry.
	//
	// Counters are cumulative metrics that only increase over time (e.g., total requests).
	// Use WithLabelValues to set specific label values before incrementing.
	//
	// Example:
	//   counter := m.CreateCounter("http_requests_total", "Total HTTP requests", []string{"method", "status"})
	//   counter.WithLabelValues("GET", "200").Inc()
	CreateCounter(name, help string, labels []string) Counter

	// CreateHistogram creates a new histogram metric and registers it to the
	// application metrics registry.
	//
	// Histograms track distributions of values (e.g., request durations) and automatically
	// calculate quantiles and counts across configurable buckets.
	//
	// Example:
	//   hist := m.CreateHistogram("request_duration_seconds", "Request duration", []string{"endpoint"}, []float64{.01, .05, .1, .5, 1, 5})
	//   hist.WithLabelValues("/api/search").Observe(0.25)
	CreateHistogram(name, help string, labels []string, buckets []float64) Histogram

	// CreateGauge creates a new gauge metric and registers it to the
	// application metrics registry.
	//
	// Gauges represent values that can go up or down (e.g., active connections, temperature).
	// Use Set, Inc, Dec, Add, or Sub to modify gauge values.
	//
	// Example:
	//   gauge := m.CreateGauge("active_connections", "Number of active connections", []string{"pool"})
	//   gauge.WithLabelValues("postgres").Set(42)
	CreateGauge(name, help string, labels []string) Gauge

	// CreateSummary creates a new summary metric and registers it to the
	// application metrics registry.
	//
	// Summaries are similar to histograms but calculate streaming quantiles on the client side.
	// Objectives define the quantile ranks (e.g., 0.5 for median, 0.99 for 99th percentile).
	//
	// Example:
	//   summary := m.CreateSummary("api_latency_seconds", "API latency", []string{"endpoint"}, map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001})
	//   summary.WithLabelValues("/api/search").Observe(0.25)
	CreateSummary(name, help string, labels []string, objectives map[float64]float64) Summary
}
