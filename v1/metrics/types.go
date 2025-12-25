package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
)

// Counter represents a cumulative metric that only increases.
// It is used to track totals such as request counts, errors, or bytes processed.
//
// This interface abstracts the underlying Prometheus CounterVec implementation.
type Counter interface {
	// WithLabelValues returns the Counter for the given label values.
	// The number of label values must match the number of labels defined
	// when the counter was created.
	WithLabelValues(lvs ...string) Counter

	// Inc increments the counter by 1.
	Inc()

	// Add adds the given value to the counter. The value must be >= 0.
	Add(val float64)
}

// Gauge represents a metric that can arbitrarily go up and down.
// It is used for values like active connections, temperature, or queue depth.
//
// This interface abstracts the underlying Prometheus GaugeVec implementation.
type Gauge interface {
	// WithLabelValues returns the Gauge for the given label values.
	// The number of label values must match the number of labels defined
	// when the gauge was created.
	WithLabelValues(lvs ...string) Gauge

	// Set sets the gauge to an arbitrary value.
	Set(val float64)

	// Inc increments the gauge by 1.
	Inc()

	// Dec decrements the gauge by 1.
	Dec()

	// Add adds the given value to the gauge. The value can be negative.
	Add(val float64)

	// Sub subtracts the given value from the gauge. The value can be negative.
	Sub(val float64)

	// SetToCurrentTime sets the gauge to the current Unix timestamp in seconds.
	SetToCurrentTime()
}

// Histogram tracks the distribution of observations (e.g., request durations or response sizes).
// Histograms calculate quantiles, counts, and sums on the server side.
//
// This interface abstracts the underlying Prometheus HistogramVec implementation.
type Histogram interface {
	// WithLabelValues returns the Histogram (Observer) for the given label values.
	// The number of label values must match the number of labels defined
	// when the histogram was created.
	WithLabelValues(lvs ...string) Observer

	// Observe adds a single observation to the histogram.
	Observe(val float64)
}

// Summary calculates streaming quantiles of observed values on the client side.
// Unlike histograms, summaries cannot be aggregated across multiple instances.
//
// This interface abstracts the underlying Prometheus SummaryVec implementation.
type Summary interface {
	// WithLabelValues returns the Summary (Observer) for the given label values.
	// The number of label values must match the number of labels defined
	// when the summary was created.
	WithLabelValues(lvs ...string) Observer

	// Observe adds a single observation to the summary.
	Observe(val float64)
}

// Observer is a common interface for metrics that observe values (Histogram and Summary).
type Observer interface {
	// Observe adds a single observation to the metric.
	Observe(val float64)
}

// counterVec wraps prometheus.CounterVec to implement the Counter interface.
type counterVec struct {
	vec *prometheus.CounterVec
}

func (c *counterVec) WithLabelValues(lvs ...string) Counter {
	return &counter{metric: c.vec.WithLabelValues(lvs...)}
}

func (c *counterVec) Inc() {
	c.vec.WithLabelValues().Inc()
}

func (c *counterVec) Add(val float64) {
	c.vec.WithLabelValues().Add(val)
}

// counter wraps prometheus.Counter.
type counter struct {
	metric prometheus.Counter
}

func (c *counter) WithLabelValues(lvs ...string) Counter {
	// This shouldn't be called on an already-labeled counter, but we return self for interface compliance
	return c
}

func (c *counter) Inc() {
	c.metric.Inc()
}

func (c *counter) Add(val float64) {
	c.metric.Add(val)
}

// gaugeVec wraps prometheus.GaugeVec to implement the Gauge interface.
type gaugeVec struct {
	vec *prometheus.GaugeVec
}

func (g *gaugeVec) WithLabelValues(lvs ...string) Gauge {
	return &gauge{metric: g.vec.WithLabelValues(lvs...)}
}

func (g *gaugeVec) Set(val float64) {
	g.vec.WithLabelValues().Set(val)
}

func (g *gaugeVec) Inc() {
	g.vec.WithLabelValues().Inc()
}

func (g *gaugeVec) Dec() {
	g.vec.WithLabelValues().Dec()
}

func (g *gaugeVec) Add(val float64) {
	g.vec.WithLabelValues().Add(val)
}

func (g *gaugeVec) Sub(val float64) {
	g.vec.WithLabelValues().Sub(val)
}

func (g *gaugeVec) SetToCurrentTime() {
	g.vec.WithLabelValues().SetToCurrentTime()
}

// gauge wraps prometheus.Gauge.
type gauge struct {
	metric prometheus.Gauge
}

func (g *gauge) WithLabelValues(lvs ...string) Gauge {
	// This shouldn't be called on an already-labeled gauge, but we return self for interface compliance
	return g
}

func (g *gauge) Set(val float64) {
	g.metric.Set(val)
}

func (g *gauge) Inc() {
	g.metric.Inc()
}

func (g *gauge) Dec() {
	g.metric.Dec()
}

func (g *gauge) Add(val float64) {
	g.metric.Add(val)
}

func (g *gauge) Sub(val float64) {
	g.metric.Sub(val)
}

func (g *gauge) SetToCurrentTime() {
	g.metric.SetToCurrentTime()
}

// histogramVec wraps prometheus.HistogramVec to implement the Histogram interface.
type histogramVec struct {
	vec *prometheus.HistogramVec
}

func (h *histogramVec) WithLabelValues(lvs ...string) Observer {
	return &observer{metric: h.vec.WithLabelValues(lvs...)}
}

func (h *histogramVec) Observe(val float64) {
	h.vec.WithLabelValues().Observe(val)
}

// summaryVec wraps prometheus.SummaryVec to implement the Summary interface.
type summaryVec struct {
	vec *prometheus.SummaryVec
}

func (s *summaryVec) WithLabelValues(lvs ...string) Observer {
	return &observer{metric: s.vec.WithLabelValues(lvs...)}
}

func (s *summaryVec) Observe(val float64) {
	s.vec.WithLabelValues().Observe(val)
}

// observer wraps prometheus.Observer.
type observer struct {
	metric prometheus.Observer
}

func (o *observer) Observe(val float64) {
	o.metric.Observe(val)
}
