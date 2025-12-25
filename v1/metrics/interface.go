package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// MetricsCollector provides an interface for collecting and exposing application metrics.
// It abstracts Prometheus metric operations with support for counters, histograms, and gauges.
//
// This interface is implemented by the concrete *Metrics type.
type MetricsCollector interface {
	// Default metric methods

	// IncrementRequests increments the request counter with a given status label.
	IncrementRequests(status string)

	// RecordRequestDuration records the duration (in seconds) for a request endpoint.
	RecordRequestDuration(start time.Time, endpoint string)

	// ObserveCPUUsage sets the CPU usage gauge for a given core.
	ObserveCPUUsage(value float64, core string)

	// Dynamic metric factories

	// CreateCounter creates a new CounterVec metric and registers it.
	CreateCounter(name, help string, labels []string) *prometheus.CounterVec

	// CreateHistogram creates a new HistogramVec metric and registers it.
	CreateHistogram(name, help string, labels []string, buckets []float64) *prometheus.HistogramVec

	// CreateGauge creates a new GaugeVec metric and registers it.
	CreateGauge(name, help string, labels []string) *prometheus.GaugeVec
}
