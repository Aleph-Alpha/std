package metrics

import "github.com/prometheus/client_golang/prometheus"

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
