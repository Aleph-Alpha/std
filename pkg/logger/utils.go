package logger

import "go.uber.org/zap"

// convertToZapFields converts error and additional field maps into Zap's structured logging fields.
// This internal helper method transforms the simplified field maps used by this logger wrapper
// into the zap.Field format required by the underlying Zap logger.
//
// Parameters:
//   - err: An error to include in the log entry, or nil if no error
//   - fields: Variable number of map[string]interface{} containing additional structured data
//
// Returns:
//   - []zap.Field: A slice of zap.Field objects ready to be passed to Zap logging methods
//
// The method handles both error objects and arbitrary key-value pairs from the fields maps.
// If multiple fields maps contain the same key, the later maps will override earlier ones.
func (l *Logger) convertToZapFields(err error, fields ...map[string]interface{}) []zap.Field {
	var zapFields []zap.Field
	if err != nil {
		zapFields = append(zapFields, zap.Error(err))
	}

	// Iterate through optional field maps and convert them into Zap fields.
	for _, fieldMap := range fields {
		for key, value := range fieldMap {
			zapFields = append(zapFields, zap.Any(key, value))
		}
	}
	return zapFields
}

// Info logs an informational message, along with an optional error and structured fields.
// Use Info for general application progress and successful operations.
//
// Parameters:
//   - msg: The log message
//   - err: An error to include in the log entry, or nil if no error
//   - fields: Variable number of map[string]interface{} containing additional structured data
//
// Example:
//
//	logger.Info("User logged in successfully", nil, map[string]interface{}{
//	    "user_id": 12345,
//	    "login_method": "oauth",
//	})
func (l *Logger) Info(msg string, err error, fields ...map[string]interface{}) {
	l.Zap.Info(msg, l.convertToZapFields(err, fields...)...)
}

// Debug logs a debug-level message, useful for development and troubleshooting.
// Debug logs are typically more verbose and include information primarily useful during
// development or when diagnosing issues.
//
// Parameters:
//   - msg: The log message
//   - err: An error to include in the log entry, or nil if no error
//   - fields: Variable number of map[string]interface{} containing additional structured data
//
// Example:
//
//	logger.Debug("Processing request", nil, map[string]interface{}{
//	    "request_id": "abc-123",
//	    "payload_size": 1024,
//	    "processing_time_ms": 15,
//	})
func (l *Logger) Debug(msg string, err error, fields ...map[string]interface{}) {
	l.Zap.Debug(msg, l.convertToZapFields(err, fields...)...)
}

// Warn logs a warning message, indicating potential issues that aren't necessarily errors.
// Warnings indicate situations that aren't failures but might need attention or
// could lead to problems if not addressed.
//
// Parameters:
//   - msg: The log message
//   - err: An error to include in the log entry, or nil if no error
//   - fields: Variable number of map[string]interface{} containing additional structured data
//
// Example:
//
//	logger.Warn("High resource usage detected", nil, map[string]interface{}{
//	    "cpu_usage": 85.5,
//	    "memory_usage_mb": 1024,
//	})
func (l *Logger) Warn(msg string, err error, fields ...map[string]interface{}) {
	l.Zap.Warn(msg, l.convertToZapFields(err, fields...)...)
}

// Error logs an error message, including details of the error and additional context fields.
// Use Error when something has gone wrong that affects the current operation but
// doesn't require immediate termination of the application.
//
// Parameters:
//   - msg: The log message
//   - err: An error to include in the log entry, or nil if no error
//   - fields: Variable number of map[string]interface{} containing additional structured data
//
// Example:
//
//	err := database.Connect()
//	if err != nil {
//	    logger.Error("Failed to connect to database", err, map[string]interface{}{
//	        "retry_count": 3,
//	        "database": "users",
//	    })
//	}
func (l *Logger) Error(msg string, err error, fields ...map[string]interface{}) {
	l.Zap.Error(msg, l.convertToZapFields(err, fields...)...)
}

// Fatal logs a critical error message and terminates the application.
// Use Fatal only for errors that make it impossible for the application to continue running.
// This method will call os.Exit(1) after logging the message.
//
// Parameters:
//   - msg: The log message
//   - err: An error to include in the log entry, or nil if no error
//   - fields: Variable number of map[string]interface{} containing additional structured data
//
// Example:
//
//	configErr := LoadConfiguration()
//	if configErr != nil {
//	    logger.Fatal("Cannot start application without configuration", configErr, map[string]interface{}{
//	        "config_path": "/etc/myapp/config.json",
//	    })
//	}
//
// Note: This function does not return as it terminates the application.
func (l *Logger) Fatal(msg string, err error, fields ...map[string]interface{}) {
	l.Zap.Fatal(msg, l.convertToZapFields(err, fields...)...)
}
