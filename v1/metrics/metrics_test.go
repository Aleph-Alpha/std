package metrics_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/Aleph-Alpha/std/v1/metrics"
)

// TestMetricsDualEndpoint verifies that the metrics package correctly
// sets up two separate registries and servers for system and application metrics.
func TestMetricsDualEndpoint(t *testing.T) {
	port := ":0"
	cfg := metrics.Config{
		SystemMetricsAddress:      &port, // Random port
		ApplicationMetricsAddress: &port, // Random port
		ServiceName:               "test-service",
	}

	m := metrics.NewMetrics(cfg)

	// Verify system registry exists and has collectors
	if m.SystemRegistry == nil {
		t.Fatal("SystemRegistry should not be nil")
	}
	if m.SystemServer == nil {
		t.Fatal("SystemServer should not be nil")
	}

	// Verify application registry exists
	if m.ApplicationRegistry == nil {
		t.Fatal("ApplicationRegistry should not be nil")
	}
	if m.ApplicationServer == nil {
		t.Fatal("ApplicationServer should not be nil")
	}
}

// TestMetricsCreation verifies that all metric types can be created
// and registered successfully.
func TestMetricsCreation(t *testing.T) {
	emptyStr := ""
	port := ":0"
	cfg := metrics.Config{
		SystemMetricsAddress:      &emptyStr, // Disable system metrics
		ApplicationMetricsAddress: &port,     // Random port
		ServiceName:               "test-service",
	}

	m := metrics.NewMetrics(cfg)

	// Test Counter creation
	counter := m.CreateCounter(
		"test_counter_total",
		"Test counter metric",
		[]string{"label1", "label2"},
	)
	if counter == nil {
		t.Fatal("CreateCounter returned nil")
	}
	counter.WithLabelValues("val1", "val2").Inc()
	counter.WithLabelValues("val1", "val2").Add(5)

	// Test Gauge creation
	gauge := m.CreateGauge(
		"test_gauge",
		"Test gauge metric",
		[]string{"pool"},
	)
	if gauge == nil {
		t.Fatal("CreateGauge returned nil")
	}
	gauge.WithLabelValues("pool1").Set(42)
	gauge.WithLabelValues("pool1").Inc()
	gauge.WithLabelValues("pool1").Dec()
	gauge.WithLabelValues("pool1").Add(10)
	gauge.WithLabelValues("pool1").Sub(5)

	// Test Histogram creation
	histogram := m.CreateHistogram(
		"test_duration_seconds",
		"Test histogram metric",
		[]string{"endpoint"},
		[]float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10},
	)
	if histogram == nil {
		t.Fatal("CreateHistogram returned nil")
	}
	histogram.WithLabelValues("/api/test").Observe(0.5)
	histogram.WithLabelValues("/api/test").Observe(1.2)

	// Test Summary creation
	summary := m.CreateSummary(
		"test_latency_seconds",
		"Test summary metric",
		[]string{"method"},
		map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001},
	)
	if summary == nil {
		t.Fatal("CreateSummary returned nil")
	}
	summary.WithLabelValues("GET").Observe(0.25)
	summary.WithLabelValues("POST").Observe(0.75)
}

// TestMetricsDisabledEndpoints verifies that endpoints can be disabled
// by setting their addresses to empty strings using Ptr("").
func TestMetricsDisabledEndpoints(t *testing.T) {
	port := ":0"
	emptyStr := ""

	tests := []struct {
		name                    string
		systemAddress           *string
		applicationAddress      *string
		expectSystemServer      bool
		expectApplicationServer bool
	}{
		{
			name:                    "Both enabled with defaults",
			systemAddress:           nil,
			applicationAddress:      nil,
			expectSystemServer:      true,
			expectApplicationServer: true,
		},
		{
			name:                    "Both enabled with explicit ports",
			systemAddress:           &port,
			applicationAddress:      &port,
			expectSystemServer:      true,
			expectApplicationServer: true,
		},
		{
			name:                    "Only system enabled",
			systemAddress:           &port,
			applicationAddress:      &emptyStr,
			expectSystemServer:      true,
			expectApplicationServer: false,
		},
		{
			name:                    "Only application enabled",
			systemAddress:           &emptyStr,
			applicationAddress:      &port,
			expectSystemServer:      false,
			expectApplicationServer: true,
		},
		{
			name:                    "Both disabled",
			systemAddress:           &emptyStr,
			applicationAddress:      &emptyStr,
			expectSystemServer:      false,
			expectApplicationServer: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := metrics.Config{
				SystemMetricsAddress:      tt.systemAddress,
				ApplicationMetricsAddress: tt.applicationAddress,
				ServiceName:               "test-service",
			}

			m := metrics.NewMetrics(cfg)

			if tt.expectSystemServer && m.SystemServer == nil {
				t.Error("Expected SystemServer to be non-nil")
			}
			if !tt.expectSystemServer && m.SystemServer != nil {
				t.Error("Expected SystemServer to be nil")
			}

			if tt.expectApplicationServer && m.ApplicationServer == nil {
				t.Error("Expected ApplicationServer to be non-nil")
			}
			if !tt.expectApplicationServer && m.ApplicationServer != nil {
				t.Error("Expected ApplicationServer to be nil")
			}
		})
	}
}

// TestMetricsServerLifecycle verifies that both servers can be started
// and shut down properly.
func TestMetricsServerLifecycle(t *testing.T) {
	sysAddr := "127.0.0.1:0"
	appAddr := "127.0.0.1:0"
	cfg := metrics.Config{
		SystemMetricsAddress:      &sysAddr,
		ApplicationMetricsAddress: &appAddr,
		ServiceName:               "test-service",
	}

	m := metrics.NewMetrics(cfg)

	// Start system server
	go func() {
		if err := m.SystemServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			t.Errorf("SystemServer.ListenAndServe() error: %v", err)
		}
	}()

	// Start application server
	go func() {
		if err := m.ApplicationServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			t.Errorf("ApplicationServer.ListenAndServe() error: %v", err)
		}
	}()

	// Give servers time to start
	time.Sleep(100 * time.Millisecond)

	// Shutdown both servers
	if err := m.SystemServer.Close(); err != nil {
		t.Errorf("SystemServer.Close() error: %v", err)
	}
	if err := m.ApplicationServer.Close(); err != nil {
		t.Errorf("ApplicationServer.Close() error: %v", err)
	}
}

// TestMetricsInterfaceImplementation verifies that *Metrics implements
// the MetricsCollector interface.
func TestMetricsInterfaceImplementation(t *testing.T) {
	emptyStr := ""
	port := ":0"
	cfg := metrics.Config{
		SystemMetricsAddress:      &emptyStr,
		ApplicationMetricsAddress: &port,
		ServiceName:               "test-service",
	}

	m := metrics.NewMetrics(cfg)

	// This should compile if *Metrics implements MetricsCollector
	var _ metrics.MetricsCollector = m

	// Test interface methods return our custom interfaces (not prometheus types)
	counter := m.CreateCounter("test", "test", []string{"label"})
	if counter == nil {
		t.Fatal("CreateCounter via interface returned nil")
	}

	gauge := m.CreateGauge("test_gauge", "test", []string{"label"})
	if gauge == nil {
		t.Fatal("CreateGauge via interface returned nil")
	}

	histogram := m.CreateHistogram("test_hist", "test", []string{"label"}, []float64{.01, .05, .1, .5, 1, 5})
	if histogram == nil {
		t.Fatal("CreateHistogram via interface returned nil")
	}

	summary := m.CreateSummary("test_summary", "test", []string{"label"}, map[float64]float64{0.5: 0.05})
	if summary == nil {
		t.Fatal("CreateSummary via interface returned nil")
	}
}

// TestPtrHelper verifies the Ptr helper function for disabling endpoints.
func TestPtrHelper(t *testing.T) {
	cfg := metrics.Config{
		SystemMetricsAddress:      metrics.Ptr(""),      // Disable using helper
		ApplicationMetricsAddress: metrics.Ptr(":9091"), // Enable with helper
		ServiceName:               "test-service",
	}

	m := metrics.NewMetrics(cfg)

	if m.SystemServer != nil {
		t.Error("Expected SystemServer to be nil when using Ptr(\"\")")
	}

	if m.ApplicationServer == nil {
		t.Error("Expected ApplicationServer to be non-nil when using Ptr(\":9091\")")
	}
}

// TestMetricTypesImplementInterfaces verifies that our metric types
// correctly implement their interfaces without exposing Prometheus types.
func TestMetricTypesImplementInterfaces(t *testing.T) {
	emptyStr := ""
	port := ":0"
	cfg := metrics.Config{
		SystemMetricsAddress:      &emptyStr,
		ApplicationMetricsAddress: &port,
		ServiceName:               "test-service",
	}

	m := metrics.NewMetrics(cfg)

	// Test that Counter interface works
	var _ metrics.Counter = m.CreateCounter("test_counter", "test", []string{"label"})

	// Test that Gauge interface works
	var _ metrics.Gauge = m.CreateGauge("test_gauge", "test", []string{"label"})

	// Test that Histogram interface works
	var _ metrics.Histogram = m.CreateHistogram("test_hist", "test", []string{"label"}, []float64{1, 5, 10})

	// Test that Summary interface works
	var _ metrics.Summary = m.CreateSummary("test_summary", "test", []string{"label"}, map[float64]float64{0.5: 0.05})
}
