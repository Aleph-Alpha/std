package logger

import (
	"log"
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Logger is a wrapper around Uber's Zap logger.
// It provides a simplified interface to the underlying Zap logger,
// with additional functionality specific to the application's needs.
type Logger struct {
	// Zap is the underlying zap.Logger instance
	// This is exposed to allow direct access to Zap-specific functionality
	// when needed, but most logging should go through the wrapper methods.
	Zap *zap.Logger

	// tracingEnabled indicates whether tracing integration is enabled
	// When true, logging methods will automatically extract trace context
	// and include trace/span IDs in log entries
	tracingEnabled bool
}

// NewLoggerClient initializes and returns a new instance of the logger based on configuration.
// This function creates a configured Zap logger with appropriate encoding, log levels,
// and output destinations.
//
// Parameters:
//   - cfg: Configuration for the logger, including log level
//
// Returns:
//   - *Logger: A configured logger instance ready for use
//
// The logger is configured with:
//   - JSON encoding for structured logging
//   - ISO8601 timestamp format
//   - Capital letter level encoding (e.g., "INFO", "ERROR")
//   - Process ID and service name as default fields
//   - Caller information (file and line) included in log entries
//   - Output directed to stderr
//
// If initialization fails, the function will call log.Fatal to terminate the application.
//
// Example:
//
//	loggerConfig := logger.Config{
//	    Level: logger.Info,
//	}
//	log := logger.NewLoggerClient(loggerConfig)
//	log.Info("Application started", nil, nil)
func NewLoggerClient(cfg Config) *Logger {

	encoderCfg := zap.NewProductionEncoderConfig()
	encoderCfg.TimeKey = "timestamp"
	encoderCfg.EncodeTime = zapcore.ISO8601TimeEncoder
	encoderCfg.EncodeLevel = zapcore.CapitalColorLevelEncoder
	encoderCfg.EncodeCaller = zapcore.FullCallerEncoder
	encoderCfg.EncodeDuration = zapcore.MillisDurationEncoder

	logLevel := zap.InfoLevel

	switch cfg.Level {
	case Debug:
		logLevel = zap.DebugLevel
	case Info:
		logLevel = zap.InfoLevel
	case Warning:
		logLevel = zap.WarnLevel
	case Error:
		logLevel = zap.ErrorLevel
	}

	config := zap.Config{
		Level:             zap.NewAtomicLevelAt(logLevel),
		Development:       false,
		DisableCaller:     false,
		DisableStacktrace: false,
		Sampling:          nil,
		Encoding:          "json",
		EncoderConfig:     encoderCfg,
		OutputPaths: []string{
			"stderr",
		},
		ErrorOutputPaths: []string{
			"stderr",
		},
		InitialFields: map[string]interface{}{
			"pid":     os.Getpid(),
			"service": cfg.ServiceName,
		},
	}

	logger, err := config.Build(zap.AddCaller(), zap.AddCallerSkip(1))

	if err != nil {
		log.Fatal(err)
	}

	return &Logger{
		Zap:            logger,
		tracingEnabled: cfg.EnableTracing,
	}
}
