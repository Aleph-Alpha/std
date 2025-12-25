package metrics

import (
	"context"
	"net/http"

	"go.uber.org/fx"

	"github.com/Aleph-Alpha/std/v1/logger"
)

// FXModule defines the Fx module for the metrics package.
// This module integrates the Prometheus metrics server into an Fx-based application
// by providing the Metrics factory and registering its lifecycle hooks.
//
// The module:
//  1. Provides the NewMetrics factory function to the dependency injection container,
//     making the Metrics instance available to other components.
//  2. Invokes RegisterMetricsLifecycle to manage startup and graceful shutdown
//     of the Prometheus HTTP server.
//
// Usage:
//
//	app := fx.New(
//	    metrics.FXModule,
//	    fx.Provide(func() metrics.Config {
//	        return metrics.Config{
//	            Address:                ":9090",
//	            EnableDefaultCollectors: true,
//	            ServiceName:             "search-store",
//	        }
//	    }),
//	    // other modules...
//	)
//
// Dependencies required by this module:
// - A metrics.Config instance must be available in the dependency injection container
// - A logger.Logger instance is optional but recommended for startup/shutdown logs
var FXModule = fx.Module("metrics",
	fx.Provide(NewMetrics),              // Creates a *Metrics instance§
	fx.Invoke(RegisterMetricsLifecycle), // Registers the lifecycle hooks
)

// RegisterMetricsLifecycle manages the startup and shutdown lifecycle
// of the Prometheus metrics HTTP server.
//
// Parameters:
//   - lc: The Fx lifecycle controller
//   - m: The Metrics instance containing the HTTP server
//   - log: The logger instance for structured lifecycle logging (optional)
//
// The lifecycle hook:
//   - OnStart: Launches the Prometheus HTTP server in a background goroutine.
//   - OnStop: Gracefully shuts down the metrics server.
//
// This ensures that metrics are available for scraping during the application's lifetime
// and that the server shuts down cleanly when the application stops.
//
// Note: This function is automatically invoked by the FXModule and does not need
// to be called directly in application code.
func RegisterMetricsLifecycle(lc fx.Lifecycle, m *Metrics, log *logger.LoggerClient) {
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			go func() {
				log.Info("Starting Prometheus metrics server", nil, map[string]interface{}{
					"address": m.Server.Addr,
				})

				if err := m.Server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
					log.Error("Error starting Prometheus metrics server", err, nil)
				}
			}()
			return nil
		},
		OnStop: func(ctx context.Context) error {
			log.Info("Shutting down Prometheus metrics server", nil, nil)
			return m.Server.Shutdown(ctx)
		},
	})
}
