// Package logger provides structured logging functionality for Go applications.
//
// The logger package is designed to provide a standardized logging approach
// with features such as log levels, contextual logging, distributed tracing integration,
// and flexible output formatting. It integrates with the fx dependency injection framework
// for easy incorporation into applications.
//
// Core Features:
//   - Structured logging with key-value pairs
//   - Support for multiple log levels (Debug, Info, Warn, Error, etc.)
//   - Context-aware logging for request tracing
//   - Distributed tracing integration with OpenTelemetry
//   - Automatic trace and span ID extraction from context
//   - Configurable output formats (JSON, text)
//   - Integration with common log collection systems
//
// Basic Usage:
//
//	import "github.com/Aleph-Alpha/data-go-packages/pkg/logger"
//
//	// Create a new logger using factory
//	log := logger.NewLoggerClient(logger.Config{
//		Level:         "info",
//		EnableTracing: true,
//	})
//
//	// Log with structured fields (without context)
//	log.Info("User logged in", err, map[string]interface{}{
//		"user_id": "12345",
//		"ip":      "192.168.1.1",
//	})
//
//	// Log with trace context (automatically includes trace_id and span_id)
//	log.InfoWithContext(ctx, "Processing request", nil, map[string]interface{}{
//		"request_id": "abc-123",
//		"user_id":    "12345",
//	})
//
//	// Log different levels
//	log.Debug("Debug message", nil, nil) // Only appears if level is Debug
//	log.Info("Info message", nil, nil)
//	log.Warn("Warning message", nil, nil)
//	log.Error("Error message", err, nil)
//
//	// Context-aware logging methods
//	log.DebugWithContext(ctx, "Debug with trace", nil, nil)
//	log.WarnWithContext(ctx, "Warning with trace", nil, nil)
//	log.ErrorWithContext(ctx, "Error with trace", err, nil)
//
// FX Module Integration:
//
// This package provides an fx module for easy integration with applications
// using the fx dependency injection framework:
//
//	app := fx.New(
//		logger.Module,
//		// ... other modules
//	)
//	app.Run()
//
// Configuration:
//
// The logger can be configured via environment variables:
//
//	ZAP_LOGGER_LEVEL=debug          # Log level (debug, info, warning, error)
//	LOGGER_ENABLE_TRACING=true      # Enable distributed tracing integration
//
// Tracing Integration:
//
// When tracing is enabled (EnableTracing: true), the logger will automatically
// extract trace and span IDs from the context and include them in log entries.
// This provides correlation between logs and distributed traces in your observability system.
//
// The following fields are automatically added to log entries when tracing is enabled:
//   - trace_id: The OpenTelemetry trace ID
//   - span_id: The OpenTelemetry span ID
//
// To use tracing, ensure your application has OpenTelemetry configured and pass
// context with active spans to the *WithContext logging methods.
//
// Performance Considerations:
//
// The logger is designed to be performant with minimal allocations. However,
// be mindful of excessive debug logging in production environments.
//
// Thread Safety:
//
// All methods on the Logger interface are safe for concurrent use by multiple
// goroutines.
package logger
