// Package logger provides structured logging functionality for Go applications.
//
// The logger package is designed to provide a standardized logging approach
// with features such as log levels, contextual logging, distributed tracing integration,
// and flexible output formatting. It integrates with the fx dependency injection framework
// for easy incorporation into applications.
//
// # Architecture
//
// This package follows the "accept interfaces, return structs" design pattern:
//   - Logger interface: Defines the contract for logging operations
//   - LoggerClient struct: Concrete implementation of the Logger interface
//   - NewLoggerClient constructor: Returns *LoggerClient (concrete type)
//   - FX module: Provides both *LoggerClient and Logger interface for dependency injection
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
// # Direct Usage (Without FX)
//
// For simple applications or tests, create a logger directly:
//
//	import "github.com/Aleph-Alpha/std/v1/logger"
//
//	// Create a new logger (returns concrete *LoggerClient)
//	log := logger.NewLoggerClient(logger.Config{
//		Level:         "info",
//		EnableTracing: true,
//	})
//
//	// Log with structured fields (without context)
//	log.Info("User logged in", nil, map[string]interface{}{
//		"user_id": "12345",
//		"ip":      "192.168.1.1",
//	})
//
//	// Log with trace context (automatically includes trace_id and span_id)
//	log.InfoWithContext(ctx, "Processing request", nil, map[string]interface{}{
//		"request_id": "abc-123",
//	})
//
// # FX Module Integration
//
// For production applications using Uber's fx, use the FXModule which provides
// both the concrete type and interface:
//
//	import (
//		"github.com/Aleph-Alpha/std/v1/logger"
//		"go.uber.org/fx"
//	)
//
//	app := fx.New(
//		logger.FXModule, // Provides *LoggerClient and logger.Logger interface
//		fx.Provide(func() logger.Config {
//			return logger.Config{
//				Level:         "info",
//				EnableTracing: true,
//				ServiceName:   "my-service",
//			}
//		}),
//		fx.Invoke(func(log *logger.LoggerClient) {
//			// Use concrete type directly
//			log.Info("Service started", nil, nil)
//		}),
//		// ... other modules
//	)
//	app.Run()
//
// # Type Aliases in Consumer Code
//
// To simplify your code and avoid tight coupling, use type aliases:
//
//	package myapp
//
//	import stdLogger "github.com/Aleph-Alpha/std/v1/logger"
//
//	// Use type alias to reference std's interface
//	type Logger = stdLogger.Logger
//
//	// Now use Logger throughout your codebase
//	func MyFunction(log Logger) {
//		log.Info("Processing", nil, nil)
//	}
//
// This eliminates the need for adapters and allows you to switch implementations
// by only changing the alias definition.
//
// # Logging Levels
//
//	// Log different levels
//	log.Debug("Debug message", nil, nil) // Only appears if level is Debug
//	log.Info("Info message", nil, nil)
//	log.Warn("Warning message", nil, nil)
//	log.Error("Error message", err, nil)
//
// # Context-Aware Logging
//
//	// Context-aware logging methods
//	log.DebugWithContext(ctx, "Debug with trace", nil, nil)
//	log.WarnWithContext(ctx, "Warning with trace", nil, nil)
//	log.ErrorWithContext(ctx, "Error with trace", err, nil)
//
// # Configuration
//
// The logger can be configured via environment variables:
//
//	ZAP_LOGGER_LEVEL=debug          # Log level (debug, info, warning, error)
//	LOGGER_ENABLE_TRACING=true      # Enable distributed tracing integration
//
// # Tracing Integration
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
// # Performance Considerations
//
// The logger is designed to be performant with minimal allocations. However,
// be mindful of excessive debug logging in production environments.
//
// # Thread Safety
//
// All methods on the Logger interface are safe for concurrent use by multiple
// goroutines.
package logger
