package logger

import "go.uber.org/zap"

// convertToZapFields converts error and additional field maps into Zap's structured logging fields.
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
func (l *Logger) Info(msg string, err error, fields ...map[string]interface{}) {
	l.Zap.Info(msg, l.convertToZapFields(err, fields...)...)
}

// Debug logs a debug-level message, useful for development and troubleshooting.
func (l *Logger) Debug(msg string, err error, fields ...map[string]interface{}) {
	l.Zap.Debug(msg, l.convertToZapFields(err, fields...)...)
}

// Warn logs a warning message, indicating potential issues that aren't necessarily errors.
func (l *Logger) Warn(msg string, err error, fields ...map[string]interface{}) {
	l.Zap.Warn(msg, l.convertToZapFields(err, fields...)...)
}

// Error logs an error message, including details of the error and additional context fields.
func (l *Logger) Error(msg string, err error, fields ...map[string]interface{}) {
	l.Zap.Error(msg, l.convertToZapFields(err, fields...)...)
}

// Fatal logs a critical error message and terminates the application.
func (l *Logger) Fatal(msg string, err error, fields ...map[string]interface{}) {
	l.Zap.Fatal(msg, l.convertToZapFields(err, fields...)...)
}
