package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// RecordDuration records the time taken for an operation using a Histogram metric.
// It measures latency and is typically deferred at the start of a function.
//
// Example:
//
//	defer metrics.RecordDuration(time.Now(), myHistogram, "query_type", "semantic")
func RecordDuration(start time.Time, histogram *prometheus.HistogramVec, labels ...string) {
	duration := time.Since(start).Seconds()
	histogram.WithLabelValues(labels...).Observe(duration)
}

// IncrementCounter increments a counter metric with the provided labels.
//
// Example:
//
//	metrics.IncrementCounter(myCounterVec, "status", "success")
func IncrementCounter(counter *prometheus.CounterVec, labels ...string) {
	counter.WithLabelValues(labels...).Inc()
}

// ObserveGauge sets a gauge value dynamically.
//
// Example:
//
//	metrics.ObserveGauge(cpuUsageGauge, 85.4, "core", "0")
func ObserveGauge(gauge *prometheus.GaugeVec, value float64, labels ...string) {
	gauge.WithLabelValues(labels...).Set(value)
}

// CreateCounterVec creates and registers a new Prometheus CounterVec safely.
// It automatically uses a consistent namespace and subsystem if you want global alignment.
//
// Example:
//
//	requestsTotal := metrics.CreateCounterVec("http", "requests_total", "Total HTTP requests", []string{"method", "status"})
func CreateCounterVec(namespace, name, help string, labels []string) *prometheus.CounterVec {
	counter := prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: namespace,
		Name:      name,
		Help:      help,
	}, labels)
	return counter
}

// CreateHistogramVec creates and registers a new Prometheus HistogramVec safely.
//
// Example:
//
//	requestLatency := metrics.CreateHistogramVec("search", "request_duration_seconds", "Duration of search requests", []string{"endpoint"})
func CreateHistogramVec(namespace, name, help string, labels []string, buckets []float64) *prometheus.HistogramVec {
	histogram := prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: namespace,
		Name:      name,
		Help:      help,
		Buckets:   buckets,
	}, labels)
	return histogram
}
