package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
)

// CreateCounter creates a new counter metric and registers it to the
// application metrics registry.
//
// Counters are cumulative metrics that only increase (e.g., total requests, errors).
//
// Example:
//
//	counter := m.CreateCounter("http_requests_total", "Total HTTP requests", []string{"method", "status"})
//	counter.WithLabelValues("GET", "200").Inc()
//	counter.WithLabelValues("POST", "500").Inc()
func (m *Metrics) CreateCounter(name, help string, labels []string) Counter {
	promCounter := createCounterVec(name, help, labels)
	m.wrappedApplicationRegisterer.MustRegister(promCounter)
	return &counterVec{vec: promCounter}
}

// CreateHistogram creates a new histogram metric and registers it to the
// application metrics registry.
//
// Histograms track distributions of values (e.g., request latencies) and
// automatically calculate quantiles and counts.
//
// Example:
//
//	hist := m.CreateHistogram(
//	    "request_duration_seconds",
//	    "Request duration in seconds",
//	    []string{"endpoint"},
//	    []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10},
//	)
//	hist.WithLabelValues("/api/search").Observe(0.25)
func (m *Metrics) CreateHistogram(name, help string, labels []string, buckets []float64) Histogram {
	promHistogram := createHistogramVec(name, help, labels, buckets)
	m.wrappedApplicationRegisterer.MustRegister(promHistogram)
	return &histogramVec{vec: promHistogram}
}

// CreateGauge creates a new gauge metric and registers it to the
// application metrics registry.
//
// Gauges represent values that can go up or down (e.g., active connections,
// temperature, queue depth).
//
// Example:
//
//	gauge := m.CreateGauge("active_connections", "Number of active connections", []string{"pool"})
//	gauge.WithLabelValues("postgres").Set(42)
//	gauge.WithLabelValues("postgres").Inc()
//	gauge.WithLabelValues("postgres").Dec()
func (m *Metrics) CreateGauge(name, help string, labels []string) Gauge {
	promGauge := createGaugeVec(name, help, labels)
	m.wrappedApplicationRegisterer.MustRegister(promGauge)
	return &gaugeVec{vec: promGauge}
}

// CreateSummary creates a new summary metric and registers it to the
// application metrics registry.
//
// Summaries calculate streaming quantiles on the client side. Objectives
// define the quantile ranks and their allowed error.
//
// Example:
//
//	summary := m.CreateSummary(
//	    "api_latency_seconds",
//	    "API request latency in seconds",
//	    []string{"endpoint"},
//	    map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001},
//	)
//	summary.WithLabelValues("/api/search").Observe(0.25)
func (m *Metrics) CreateSummary(name, help string, labels []string, objectives map[float64]float64) Summary {
	promSummary := createSummaryVec(name, help, labels, objectives)
	m.wrappedApplicationRegisterer.MustRegister(promSummary)
	return &summaryVec{vec: promSummary}
}

// createCounterVec defines a new CounterVec with standard options.
func createCounterVec(name, help string, labels []string) *prometheus.CounterVec {
	return prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: name,
			Help: help,
		},
		labels,
	)
}

// createHistogramVec defines a new HistogramVec with configurable buckets.
func createHistogramVec(name, help string, labels []string, buckets []float64) *prometheus.HistogramVec {
	return prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    name,
			Help:    help,
			Buckets: buckets,
		},
		labels,
	)
}

// createGaugeVec defines a new GaugeVec for resource monitoring.
func createGaugeVec(name, help string, labels []string) *prometheus.GaugeVec {
	return prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: name,
			Help: help,
		},
		labels,
	)
}

// createSummaryVec defines a new SummaryVec with configurable objectives.
func createSummaryVec(name, help string, labels []string, objectives map[float64]float64) *prometheus.SummaryVec {
	return prometheus.NewSummaryVec(
		prometheus.SummaryOpts{
			Name:       name,
			Help:       help,
			Objectives: objectives,
		},
		labels,
	)
}
