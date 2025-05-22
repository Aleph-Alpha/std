// Package logger provides structured logging functionality for Go applications.
//
// The logger package is designed to provide a standardized logging approach
// with features such as log levels, contextual logging, and flexible output formatting.
// It integrates with the fx dependency injection framework for easy incorporation
// into applications.
//
// Core Features:
//   - Structured logging with key-value pairs
//   - Support for multiple log levels (Debug, Info, Warn, Error, etc.)
//   - Context-aware logging for request tracing
//   - Configurable output formats (JSON, text)
//   - Integration with common log collection systems
//
// Basic Usage:
//
//	import "gitlab.aleph-alpha.de/engineering/pharia-data-search/data-go-packages/pkg/logger"
//
//	// Create a new logger using factory
//	log, err := logger.NewLogger(logger.Config{
//		Level:  "info",
//		Format: "json",
//	})
//	if err != nil {
//		panic(err)
//	}
//
//	// Log with structured fields
//	log.Info("User logged in", err, map[string]interface{}{
//		"user_id": "12345",
//		"ip":      "192.168.1.1",
//	})
//
//	// Log different levels
//	log.Debug("Debug message", nil, nil) // Only appears if level is Debug
//	log.Info("Info message", nil, nil)
//	log.Warn("Warning message", nil, nil)
//	log.Error("Error message", err, nil)
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
//	LOG_LEVEL=debug
//	LOG_FORMAT=json
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
