package metrics

import (
	"context"
	"net/http"

	"go.uber.org/fx"

	"github.com/Aleph-Alpha/std/v1/logger"
)

// FXModule defines the Fx module for the metrics package.
// This module integrates two separate Prometheus metrics servers into an Fx-based application
// by providing the Metrics factory and registering lifecycle hooks for both servers.
//
// The module provides:
// 1. *Metrics (concrete type) for direct use
// 2. MetricsCollector interface for dependency injection
// 3. Lifecycle management for both system and application metrics HTTP servers
//
// System Metrics Endpoint (default: :9090):
//   - Go runtime metrics (goroutines, memory, GC)
//   - Process metrics (CPU, file descriptors)
//   - Build info metrics
//
// Application Metrics Endpoint (default: :9091):
//   - User-defined custom metrics created via CreateCounter, CreateGauge, etc.
//
// Usage:
//
//	app := fx.New(
//	    metrics.FXModule,
//	    fx.Provide(func() metrics.Config {
//	        return metrics.Config{
//	            SystemMetricsAddress:      ":9090",
//	            ApplicationMetricsAddress: ":9091",
//	            ServiceName:               "search-store",
//	        }
//	    }),
//	    fx.Invoke(func(m metrics.MetricsCollector) {
//	        // Create custom metrics
//	        counter := m.CreateCounter("requests_total", "Total requests", []string{"endpoint"})
//	        counter.WithLabelValues("/api/search").Inc()
//	    }),
//	)
//
// Dependencies required by this module:
// - A metrics.Config instance must be available in the dependency injection container
// - A logger.LoggerClient instance is optional but recommended for startup/shutdown logs
var FXModule = fx.Module("metrics",
	fx.Provide(
		NewMetrics, // Provides *Metrics
		// Also provide the MetricsCollector interface
		fx.Annotate(
			func(m *Metrics) MetricsCollector { return m },
			fx.As(new(MetricsCollector)),
		),
	),
	fx.Invoke(RegisterMetricsLifecycle), // Registers the lifecycle hooks
)

// RegisterMetricsLifecycle manages the startup and shutdown lifecycle
// of both Prometheus metrics HTTP servers (system and application).
//
// Parameters:
//   - lc: The Fx lifecycle controller
//   - m: The Metrics instance containing both HTTP servers
//   - log: The logger instance for structured lifecycle logging (optional)
//
// The lifecycle hook:
//   - OnStart: Launches both metrics servers in background goroutines
//   - OnStop: Gracefully shuts down both servers
//
// This ensures that both metrics endpoints are available for scraping during
// the application's lifetime and shut down cleanly when the application stops.
//
// Note: This function is automatically invoked by the FXModule and does not need
// to be called directly in application code.
func RegisterMetricsLifecycle(lc fx.Lifecycle, m *Metrics, log *logger.LoggerClient) {
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			// Start system metrics server if configured
			if m.SystemServer != nil {
				go func() {
					log.Info("Starting system metrics server", nil, map[string]interface{}{
						"address": m.SystemServer.Addr,
					})

					if err := m.SystemServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
						log.Error("Error starting system metrics server", err, nil)
					}
				}()
			}

			// Start application metrics server if configured
			if m.ApplicationServer != nil {
				go func() {
					log.Info("Starting application metrics server", nil, map[string]interface{}{
						"address": m.ApplicationServer.Addr,
					})

					if err := m.ApplicationServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
						log.Error("Error starting application metrics server", err, nil)
					}
				}()
			}

			return nil
		},
		OnStop: func(ctx context.Context) error {
			// Shutdown system metrics server
			if m.SystemServer != nil {
				log.Info("Shutting down system metrics server", nil, nil)
				if err := m.SystemServer.Shutdown(ctx); err != nil {
					log.Error("Error shutting down system metrics server", err, nil)
				}
			}

			// Shutdown application metrics server
			if m.ApplicationServer != nil {
				log.Info("Shutting down application metrics server", nil, nil)
				if err := m.ApplicationServer.Shutdown(ctx); err != nil {
					log.Error("Error shutting down application metrics server", err, nil)
				}
			}

			return nil
		},
	})
}
