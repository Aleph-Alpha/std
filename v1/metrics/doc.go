// Package metrics provides Prometheus-based monitoring and metrics collection
// functionality for Go applications.
//
// The metrics package is designed to provide a standardized observability
// approach with dual HTTP endpoints for system-level and application-level metrics,
// full control over metric definitions, and integration with the Fx dependency
// injection framework for easy incorporation into Aleph Alpha services.
//
// # Architecture
//
// This package follows the "accept interfaces, return structs" design pattern:
//   - MetricsCollector interface: Defines the contract for metrics operations
//   - Metrics struct: Concrete implementation of the MetricsCollector interface
//   - NewMetrics constructor: Returns *Metrics (concrete type)
//   - FX module: Provides both *Metrics and MetricsCollector interface for dependency injection
//
// # Dual Endpoint Design
//
// The package provides two separate Prometheus endpoints:
//
// 1. System Metrics Endpoint (default: :9090)
//   - Go runtime metrics (goroutines, memory, GC stats)
//   - Process metrics (CPU, file descriptors, memory)
//   - Build info metrics
//   - Automatically registered, no user action required
//
// 2. Application Metrics Endpoint (default: :9091)
//   - User-defined custom metrics only
//   - Full control over metric names, types, and labels
//   - No default metrics - clean slate for application observability
//
// This separation allows:
//   - Different scrape configurations (e.g., system metrics every 15s, app metrics every 5s)
//   - Different access controls (e.g., system metrics internal-only)
//   - Cleaner organization and cardinality management
//
// # Core Features
//
//   - Two configurable /metrics endpoints for Prometheus scraping
//   - Integration with go.uber.org/fx for automatic lifecycle management
//   - Support for all Prometheus metric types (Counter, Gauge, Histogram, Summary)
//   - User-defined metrics with custom labels
//   - Automatic service label wrapping for multi-service observability
//   - Graceful startup and shutdown via Fx lifecycle hooks
//
// # Direct Usage (Without FX)
//
// For simple applications or tests, create metrics directly:
//
//	import "github.com/Aleph-Alpha/std/v1/metrics"
//
//	// Create metrics servers (returns concrete *Metrics)
//	cfg := metrics.Config{
//		SystemMetricsAddress:      ":9090",
//		ApplicationMetricsAddress: ":9091",
//		ServiceName:               "search-store",
//	}
//
//	m := metrics.NewMetrics(cfg)
//
//	// Start both servers
//	go m.SystemServer.ListenAndServe()
//	go m.ApplicationServer.ListenAndServe()
//
//	// Create custom application metrics
//	requestCounter := m.CreateCounter(
//		"http_requests_total",
//		"Total HTTP requests",
//		[]string{"method", "status"},
//	)
//	requestCounter.WithLabelValues("GET", "200").Inc()
//
//	// Access metrics:
//	// - System: http://localhost:9090/metrics
//	// - Application: http://localhost:9091/metrics
//
// # FX Module Integration
//
// For production applications using Uber's fx, use the FXModule which provides
// both the concrete type and interface:
//
//	import (
//		"go.uber.org/fx"
//		"github.com/Aleph-Alpha/std/v1/metrics"
//		"github.com/Aleph-Alpha/std/v1/logger"
//	)
//
//	app := fx.New(
//		logger.FXModule, // Optional: provides std logger
//		metrics.FXModule, // Provides *Metrics and MetricsCollector interface
//		fx.Provide(func() metrics.Config {
//			return metrics.Config{
//				SystemMetricsAddress:      ":9090",
//				ApplicationMetricsAddress: ":9091",
//				ServiceName:               "search-store",
//			}
//		}),
//		fx.Invoke(func(m metrics.MetricsCollector) {
//			// Define application metrics
//			counter := m.CreateCounter(
//				"requests_processed",
//				"Total processed requests",
//				[]string{"status"},
//			)
//			counter.WithLabelValues("success").Inc()
//		}),
//	)
//	app.Run()
//
// # Type Aliases in Consumer Code
//
// To simplify your code and make it metrics-agnostic, use type aliases:
//
//	package myapp
//
//	import stdMetrics "github.com/Aleph-Alpha/std/v1/metrics"
//
//	// Use type alias to reference std's interface
//	type MetricsCollector = stdMetrics.MetricsCollector
//
//	// Now use MetricsCollector throughout your codebase
//	func MyFunction(metrics MetricsCollector) {
//		counter := metrics.CreateCounter("my_counter", "My counter", []string{"label"})
//		counter.WithLabelValues("value").Inc()
//	}
//
// This eliminates the need for adapters and allows you to switch implementations
// by only changing the alias definition.
//
// # Configuration
//
// The metrics servers can be configured via environment variables:
//
//	METRICS_SYSTEM_ADDRESS=:9090          # System metrics endpoint address
//	METRICS_APPLICATION_ADDRESS=:9091     # Application metrics endpoint address
//	METRICS_SERVICE_NAME=search-store     # Adds service label to all metrics
//
// Set an address to empty string ("") to disable that endpoint:
//
//	cfg := metrics.Config{
//		SystemMetricsAddress:      "",      // Disable system metrics
//		ApplicationMetricsAddress: ":9091", // Only application metrics
//		ServiceName:               "search-store",
//	}
//
// # Metric Types and Usage Examples
//
// ## 1. Counter - Cumulative metrics that only increase
//
// Use counters for tracking totals (requests, errors, bytes processed):
//
//	requestCounter := m.CreateCounter(
//		"http_requests_total",
//		"Total number of HTTP requests",
//		[]string{"method", "status", "endpoint"},
//	)
//
//	// In your HTTP handler:
//	requestCounter.WithLabelValues("GET", "200", "/api/search").Inc()
//	requestCounter.WithLabelValues("POST", "500", "/api/index").Inc()
//
// ## 2. Gauge - Values that can go up or down
//
// Use gauges for current state (active connections, queue depth, temperature):
//
//	activeConnections := m.CreateGauge(
//		"active_database_connections",
//		"Number of active database connections",
//		[]string{"pool"},
//	)
//
//	// Track connection pool size
//	activeConnections.WithLabelValues("postgres").Set(25)
//	activeConnections.WithLabelValues("postgres").Inc()  // Add 1
//	activeConnections.WithLabelValues("postgres").Dec()  // Subtract 1
//	activeConnections.WithLabelValues("postgres").Add(5) // Add 5
//
// ## 3. Histogram - Distribution tracking with quantiles
//
// Use histograms for latency, request sizes, or any value distribution:
//
//	requestDuration := m.CreateHistogram(
//		"http_request_duration_seconds",
//		"HTTP request duration in seconds",
//		[]string{"method", "endpoint"},
//		[]float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10}, // buckets
//	)
//
//	// In your HTTP handler:
//	start := time.Now()
//	// ... handle request ...
//	duration := time.Since(start).Seconds()
//	requestDuration.WithLabelValues("GET", "/api/search").Observe(duration)
//
// Prometheus will automatically calculate:
//   - Quantiles (p50, p95, p99)
//   - Count of observations
//   - Sum of all observed values
//
// ## 4. Summary - Client-side streaming quantiles
//
// Use summaries when you need precise quantile calculations on the client side:
//
//	apiLatency := m.CreateSummary(
//		"api_request_latency_seconds",
//		"API request latency in seconds",
//		[]string{"endpoint"},
//		map[float64]float64{
//			0.5:  0.05,  // 50th percentile (median) with 5% error
//			0.9:  0.01,  // 90th percentile with 1% error
//			0.99: 0.001, // 99th percentile with 0.1% error
//		},
//	)
//
//	// In your API handler:
//	start := time.Now()
//	// ... process request ...
//	apiLatency.WithLabelValues("/api/search").Observe(time.Since(start).Seconds())
//
// # Complete HTTP Middleware Example
//
// Here's a complete example of HTTP request instrumentation:
//
//	package main
//
//	import (
//		"net/http"
//		"time"
//
//		"go.uber.org/fx"
//		"github.com/Aleph-Alpha/std/v1/metrics"
//		"github.com/Aleph-Alpha/std/v1/logger"
//	)
//
//	type HTTPMetrics struct {
//		RequestsTotal   metrics.Counter
//		RequestDuration metrics.Histogram
//		RequestSize     metrics.Histogram
//		ResponseSize    metrics.Histogram
//		ActiveRequests  metrics.Gauge
//	}
//
//	func NewHTTPMetrics(m metrics.MetricsCollector) *HTTPMetrics {
//		return &HTTPMetrics{
//			RequestsTotal: m.CreateCounter(
//				"http_requests_total",
//				"Total number of HTTP requests",
//				[]string{"method", "endpoint", "status"},
//			),
//			RequestDuration: m.CreateHistogram(
//				"http_request_duration_seconds",
//				"HTTP request duration in seconds",
//				[]string{"method", "endpoint"},
//				[]float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10},
//			),
//			RequestSize: m.CreateHistogram(
//				"http_request_size_bytes",
//				"HTTP request size in bytes",
//				[]string{"method", "endpoint"},
//				exponentialBuckets(100, 10, 8), // 100, 1000, 10000, ...
//			),
//			ResponseSize: m.CreateHistogram(
//				"http_response_size_bytes",
//				"HTTP response size in bytes",
//				[]string{"method", "endpoint"},
//				exponentialBuckets(100, 10, 8),
//			),
//			ActiveRequests: m.CreateGauge(
//				"http_requests_active",
//				"Number of active HTTP requests",
//				[]string{"method", "endpoint"},
//			),
//		}
//	}
//
//	func (hm *HTTPMetrics) Middleware(next http.Handler) http.Handler {
//		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
//			start := time.Now()
//			endpoint := r.URL.Path
//			method := r.Method
//
//			// Track active requests
//			hm.ActiveRequests.WithLabelValues(method, endpoint).Inc()
//			defer hm.ActiveRequests.WithLabelValues(method, endpoint).Dec()
//
//			// Track request size
//			if r.ContentLength > 0 {
//				hm.RequestSize.WithLabelValues(method, endpoint).Observe(float64(r.ContentLength))
//			}
//
//			// Capture response
//			wrw := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
//			next.ServeHTTP(wrw, r)
//
//			// Record metrics
//			duration := time.Since(start).Seconds()
//			status := fmt.Sprintf("%d", wrw.statusCode)
//
//			hm.RequestsTotal.WithLabelValues(method, endpoint, status).Inc()
//			hm.RequestDuration.WithLabelValues(method, endpoint).Observe(duration)
//			hm.ResponseSize.WithLabelValues(method, endpoint).Observe(float64(wrw.bytesWritten))
//		})
//	}
//
//	type responseWriter struct {
//		http.ResponseWriter
//		statusCode   int
//		bytesWritten int
//	}
//
//	func (rw *responseWriter) WriteHeader(code int) {
//		rw.statusCode = code
//		rw.ResponseWriter.WriteHeader(code)
//	}
//
//	func (rw *responseWriter) Write(b []byte) (int, error) {
//		n, err := rw.ResponseWriter.Write(b)
//		rw.bytesWritten += n
//		return n, err
//	}
//
//	func main() {
//		app := fx.New(
//			logger.FXModule,
//			metrics.FXModule,
//			fx.Provide(
//				func() metrics.Config {
//					return metrics.Config{
//						SystemMetricsAddress:      ":9090",
//						ApplicationMetricsAddress: ":9091",
//						ServiceName:               "api-gateway",
//					}
//				},
//				NewHTTPMetrics,
//			),
//			fx.Invoke(func(hm *HTTPMetrics) {
//				// Your HTTP server setup with middleware
//				mux := http.NewServeMux()
//				mux.HandleFunc("/api/search", handleSearch)
//
//				server := &http.Server{
//					Addr:    ":8080",
//					Handler: hm.Middleware(mux),
//				}
//
//				go server.ListenAndServe()
//			}),
//		)
//		app.Run()
//	}
//
// # Database Metrics Example
//
// Track database operations with detailed labels:
//
//	type DBMetrics struct {
//		QueriesTotal    metrics.Counter
//		QueryDuration   metrics.Histogram
//		ActiveConns     metrics.Gauge
//		QueryErrors     metrics.Counter
//	}
//
//	func NewDBMetrics(m metrics.MetricsCollector) *DBMetrics {
//		return &DBMetrics{
//			QueriesTotal: m.CreateCounter(
//				"db_queries_total",
//				"Total number of database queries",
//				[]string{"operation", "table"},
//			),
//			QueryDuration: m.CreateHistogram(
//				"db_query_duration_seconds",
//				"Database query duration in seconds",
//				[]string{"operation", "table"},
//				[]float64{.001, .005, .01, .025, .05, .1, .25, .5, 1, 2.5},
//			),
//			ActiveConns: m.CreateGauge(
//				"db_connections_active",
//				"Number of active database connections",
//				[]string{"pool"},
//			),
//			QueryErrors: m.CreateCounter(
//				"db_query_errors_total",
//				"Total number of database query errors",
//				[]string{"operation", "table", "error_type"},
//			),
//		}
//	}
//
//	// Usage in database layer:
//	func (db *Database) Query(ctx context.Context, query string) error {
//		start := time.Now()
//		defer func() {
//			duration := time.Since(start).Seconds()
//			db.metrics.QueryDuration.WithLabelValues("select", "documents").Observe(duration)
//		}()
//
//		db.metrics.QueriesTotal.WithLabelValues("select", "documents").Inc()
//
//		err := db.conn.QueryContext(ctx, query)
//		if err != nil {
//			db.metrics.QueryErrors.WithLabelValues("select", "documents", "timeout").Inc()
//			return err
//		}
//
//		return nil
//	}
//
// # Business Metrics Example
//
// Track application-specific business metrics:
//
//	type BusinessMetrics struct {
//		DocumentsIndexed    metrics.Counter
//		SearchesPerformed   metrics.Counter
//		ActiveUsers         metrics.Gauge
//		DocumentProcessTime metrics.Histogram
//		CacheHits           metrics.Counter
//	}
//
//	func NewBusinessMetrics(m metrics.MetricsCollector) *BusinessMetrics {
//		return &BusinessMetrics{
//			DocumentsIndexed: m.CreateCounter(
//				"documents_indexed_total",
//				"Total number of documents indexed",
//				[]string{"type", "status"},
//			),
//			SearchesPerformed: m.CreateCounter(
//				"searches_performed_total",
//				"Total number of search queries performed",
//				[]string{"type", "result_count_bucket"},
//			),
//			ActiveUsers: m.CreateGauge(
//				"active_users",
//				"Number of currently active users",
//				[]string{"tier"},
//			),
//			DocumentProcessTime: m.CreateHistogram(
//				"document_processing_seconds",
//				"Time to process a document",
//				[]string{"type", "stage"},
//				[]float64{.1, .5, 1, 2, 5, 10, 30, 60},
//			),
//			CacheHits: m.CreateCounter(
//				"cache_operations_total",
//				"Total cache operations",
//				[]string{"cache", "operation", "result"},
//			),
//		}
//	}
//
// # Performance Considerations
//
// 1. Label Cardinality:
//   - Keep label values bounded (avoid user IDs, request IDs, timestamps)
//   - High cardinality can cause memory issues
//   - Good: []string{"method", "status"} with ~10 combinations
//   - Bad: []string{"user_id"} with millions of users
//
// 2. Metric Updates:
//   - All Prometheus metric operations are thread-safe
//   - Prefer gauges over counters when values can decrease
//   - Use histograms for latency tracking (more efficient than summaries)
//
// 3. Histogram vs Summary:
//   - Histograms: Server-side quantile calculation, aggregatable across instances
//   - Summaries: Client-side quantile calculation, NOT aggregatable
//   - Prefer histograms unless you need precise quantiles per instance
//
// # Thread Safety
//
// All methods on the Metrics struct and all Prometheus collectors are safe for
// concurrent use by multiple goroutines. No additional synchronization is needed.
//
// # Observability
//
// Exposed metrics can be scraped by Prometheus and visualized in Grafana or any
// compatible monitoring system. Example Prometheus scrape config:
//
//	scrape_configs:
//	  - job_name: 'system-metrics'
//	    static_configs:
//	      - targets: ['localhost:9090']
//	    scrape_interval: 15s
//
//	  - job_name: 'application-metrics'
//	    static_configs:
//	      - targets: ['localhost:9091']
//	    scrape_interval: 5s
//
// # Testing
//
// For unit tests, you can create a metrics instance without starting the servers:
//
//	func TestMyFunction(t *testing.T) {
//		cfg := metrics.Config{
//			SystemMetricsAddress:      "",      // Disable
//			ApplicationMetricsAddress: ":0",    // Random port
//			ServiceName:               "test",
//		}
//		m := metrics.NewMetrics(cfg)
//
//		counter := m.CreateCounter("test_counter", "Test counter", []string{"label"})
//		counter.WithLabelValues("test").Inc()
//
//		// Verify metrics using prometheus/testutil
//		// ...
//	}
package metrics
