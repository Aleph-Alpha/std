// Package metrics provides Prometheus-based monitoring and metrics collection
// functionality for Go applications.
//
// The metrics package is designed to provide a standardized observability
// approach with features such as configurable HTTP endpoints for metrics exposure,
// automatic runtime instrumentation, and integration with the Fx dependency
// injection framework for easy incorporation into Aleph Alpha services.
//
// Core Features:
//   - Exposes a configurable /metrics endpoint for Prometheus scraping
//   - Integration with go.uber.org/fx for automatic lifecycle management
//   - Automatic registration of Go runtime and process-level metrics
//   - Support for custom metric registration (counters, gauges, histograms)
//   - Optional namespace and service name labelling for multi-service observability
//   - Graceful startup and shutdown via Fx lifecycle hooks
//
// Basic Usage:
//
//	import "github.com/Aleph-Alpha/std/v1/metrics"
//
//	// Create a new metrics server manually
//	cfg := metrics.Config{
//		Address:                ":9090",
//		EnableDefaultCollectors: true,
//		ServiceName:             "search-store",
//		Namespace:               "pharia_data",
//	}
//
//	m := metrics.NewMetrics(cfg)
//	go m.Server.ListenAndServe()
//
//	// Register custom counters
//	counter := prometheus.NewCounterVec(
//	    prometheus.CounterOpts{
//	        Name: "http_requests_total",
//	        Help: "Total number of HTTP requests processed.",
//	    },
//	    []string{"method", "endpoint"},
//	)
//	m.Registry.MustRegister(counter)
//
// FX Module Integration:
//
// This package provides an Fx module for easy integration with applications
// using the Fx dependency injection framework:
//
//	app := fx.New(
//		logger.FXModule,
//		metrics.FXModule,
//		fx.Provide(func() metrics.Config {
//			return metrics.Config{
//				Address:                ":9090",
//				EnableDefaultCollectors: true,
//				ServiceName:             "search-store",
//			}
//		}),
//	)
//	app.Run()
//
// Configuration:
//
// The metrics server can be configured via environment variables:
//
//	METRICS_ADDRESS=:9090                      # Port and address for /metrics endpoint
//	METRICS_ENABLE_DEFAULT_COLLECTORS=true     # Enable runtime and process metrics
//	METRICS_NAMESPACE=pharia_data              # Optional prefix for all metric names
//	METRICS_SERVICE_NAME=search-store          # Adds service label to all metrics
//
// Default Collectors:
//
// When EnableDefaultCollectors is true, the package automatically registers
// the following collectors:
//   - Go runtime metrics (goroutines, GC stats, heap usage)
//   - Process metrics (CPU time, memory, file descriptors)
//
// These metrics provide deep visibility into service performance and stability.
//
// Custom Metrics:
//
// Applications can register additional Prometheus metrics using the exposed
// Registry. For example:
//
//	requestDuration := prometheus.NewHistogramVec(
//	    prometheus.HistogramOpts{
//	        Name:    "http_request_duration_seconds",
//	        Help:    "Histogram of request latencies.",
//	        Buckets: prometheus.DefBuckets,
//	    },
//	    []string{"method", "route"},
//	)
//	m.Registry.MustRegister(requestDuration)
//
// Performance Considerations:
//
// The metrics server runs in a separate HTTP handler and is lightweight.
// Default collectors use minimal resources, but avoid unnecessary high-cardinality
// metrics or unbounded label values to maintain good performance.
//
// Thread Safety:
//
// All methods on the Metrics struct and Prometheus collectors are safe for
// concurrent use by multiple goroutines.
//
// Observability:
//
// Exposed metrics can be visualized in Prometheus, Grafana, or any compatible
// monitoring system to provide insights into service health, latency, and
// resource utilization.
package metrics
