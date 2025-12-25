package logger

import (
	"log"
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// LoggerClient is a wrapper around Uber's Zap logger.
// It provides a simplified interface to the underlying Zap logger,
// with additional functionality specific to the application's needs.
//
// LoggerClient implements the Logger interface.
type LoggerClient struct {
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
//   - cfg: Configuration for the logger, including log level, caller skip, and tracing options
//
// Returns:
//   - *LoggerClient: A configured logger instance ready for use
//
// The logger is configured with:
//   - JSON encoding for structured logging
//   - ISO8601 timestamp format
//   - Capital letter level encoding (e.g., "INFO", "ERROR") without color codes
//   - Process ID and service name as default fields
//   - Caller information (file and line) included in log entries
//   - Configurable caller skip depth for wrapper scenarios
//   - Output directed to stderr
//
// If initialization fails, the function will call log.Fatal to terminate the application.
//
// Example (direct usage):
//
//	loggerConfig := logger.Config{
//	    Level:       logger.Info,
//	    ServiceName: "my-service",
//	    CallerSkip:  1, // default, can be omitted
//	}
//	log := logger.NewLoggerClient(loggerConfig)
//	log.Info("Application started", nil, nil)
//
// Example (with service wrapper):
//
//	loggerConfig := logger.Config{
//	    Level:       logger.Info,
//	    ServiceName: "my-service",
//	    CallerSkip:  2, // skip service wrapper + std wrapper
//	}
//	log := logger.NewLoggerClient(loggerConfig)
func NewLoggerClient(cfg Config) *LoggerClient {

	encoderCfg := zap.NewProductionEncoderConfig()
	encoderCfg.TimeKey = "timestamp"
	encoderCfg.EncodeTime = zapcore.ISO8601TimeEncoder
	encoderCfg.EncodeLevel = zapcore.CapitalLevelEncoder
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

	// Default to 1 if not set, which works for direct usage of std logger
	callerSkip := cfg.CallerSkip
	if callerSkip <= 0 {
		callerSkip = 1
	}

	logger, err := config.Build(zap.AddCaller(), zap.AddCallerSkip(callerSkip))

	if err != nil {
		log.Fatal(err)
	}

	return &LoggerClient{
		Zap:            logger,
		tracingEnabled: cfg.EnableTracing,
	}
}
