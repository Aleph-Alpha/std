package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// IncrementRequests increments the request counter with a given status label.
// Example: metrics.IncrementRequests("success")
func (m *Metrics) IncrementRequests(status string) {
	m.requestsTotal.WithLabelValues(status).Inc()
}

// RecordRequestDuration records the duration (in seconds) for a request endpoint.
// Example: defer metrics.RecordRequestDuration(time.Now(), "/search")
func (m *Metrics) RecordRequestDuration(start time.Time, endpoint string) {
	duration := time.Since(start).Seconds()
	m.requestDuration.WithLabelValues(endpoint).Observe(duration)
}

// ObserveCPUUsage sets the CPU usage gauge for a given core.
// Example: metrics.ObserveCPUUsage(87.2, "core_0")
func (m *Metrics) ObserveCPUUsage(value float64, core string) {
	m.cpuUsageGauge.WithLabelValues(core).Set(value)
}

// CreateCounter creates a new CounterVec metric and registers it.
func (m *Metrics) CreateCounter(name, help string, labels []string) *prometheus.CounterVec {
	counter := createCounterVec(name, help, labels)
	m.Registry.MustRegister(counter)
	return counter
}

// CreateHistogram creates a new HistogramVec metric and registers it.
func (m *Metrics) CreateHistogram(name, help string, labels []string, buckets []float64) *prometheus.HistogramVec {
	hist := createHistogramVec(name, help, labels, buckets)
	m.Registry.MustRegister(hist)
	return hist
}

// CreateGauge creates a new GaugeVec metric and registers it.
func (m *Metrics) CreateGauge(name, help string, labels []string) *prometheus.GaugeVec {
	gauge := createGaugeVec(name, help, labels)
	m.Registry.MustRegister(gauge)
	return gauge
}

// createCounterVec defines a new CounterVec with standard options.
// Used internally by NewMetrics to maintain consistency.
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
// Used internally by NewMetrics for latency tracking.
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

// createGaugeVec defines a new GaugeVec safely for resource monitoring.
// Used internally by NewMetrics to track resource utilization.
func createGaugeVec(name, help string, labels []string) *prometheus.GaugeVec {
	return prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: name,
			Help: help,
		},
		labels,
	)
}
