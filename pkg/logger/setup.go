package logger

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"log"
	"os"
)

// Logger is a wrapper around Uber's Zap logger.
type Logger struct {
	Zap *zap.Logger
}

// NewLoggerClient initializes and returns a new instance of the logger based on configuration.
func NewLoggerClient(cfg Config) *Logger {

	encoderCfg := zap.NewProductionEncoderConfig()
	encoderCfg.TimeKey = "timestamp"
	encoderCfg.EncodeTime = zapcore.ISO8601TimeEncoder
	encoderCfg.EncodeLevel = zapcore.CapitalLevelEncoder

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
			"service": "pharia-data-api",
		},
	}

	logger, err := config.Build(zap.AddCaller(), zap.AddCallerSkip(1))

	if err != nil {
		log.Fatal(err)
	}

	return &Logger{Zap: logger}
}
