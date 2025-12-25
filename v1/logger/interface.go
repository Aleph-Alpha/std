package logger

import (
	"context"
)

// Logger provides a high-level interface for structured logging.
// It wraps Uber's Zap logger with a simplified API and optional tracing integration.
//
// This interface is implemented by the concrete *LoggerClient type.
type Logger interface {
	// Basic logging methods

	// Debug logs a debug-level message, useful for development and troubleshooting.
	Debug(msg string, err error, fields ...map[string]interface{})

	// Info logs an informational message about general application progress.
	Info(msg string, err error, fields ...map[string]interface{})

	// Warn logs a warning message, indicating potential issues.
	Warn(msg string, err error, fields ...map[string]interface{})

	// Error logs an error message with details of the error.
	Error(msg string, err error, fields ...map[string]interface{})

	// Fatal logs a critical error message and terminates the application.
	Fatal(msg string, err error, fields ...map[string]interface{})

	// Context-aware logging methods (automatically include trace/span IDs when tracing is enabled)

	// DebugWithContext logs a debug-level message with trace context.
	DebugWithContext(ctx context.Context, msg string, err error, fields ...map[string]interface{})

	// InfoWithContext logs an informational message with trace context.
	InfoWithContext(ctx context.Context, msg string, err error, fields ...map[string]interface{})

	// WarnWithContext logs a warning message with trace context.
	WarnWithContext(ctx context.Context, msg string, err error, fields ...map[string]interface{})

	// ErrorWithContext logs an error message with trace context.
	ErrorWithContext(ctx context.Context, msg string, err error, fields ...map[string]interface{})

	// FatalWithContext logs a critical error message with trace context and terminates the application.
	FatalWithContext(ctx context.Context, msg string, err error, fields ...map[string]interface{})
}
