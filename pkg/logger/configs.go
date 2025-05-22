package logger

// Log level constants that define the available logging levels.
// These string constants are used in configuration to set the desired log level.
const (
	// Debug represents the most verbose logging level, intended for development and troubleshooting.
	// When the logger is set to Debug level, all log messages (Debug, Info, Warning, Error) will be output.
	Debug = "debug"

	// Info represents the standard logging level for general operational information.
	// When the logger is set to Info level, Info, Warning, and Error messages will be output, but Debug messages will be suppressed.
	Info = "info"

	// Warning represents the logging level for potential issues that aren't errors.
	// When the logger is set to Warning level, only Warning and Error messages will be output.
	Warning = "warning"

	// Error represents the logging level for error conditions.
	// When the logger is set to Error level, only Error messages will be output.
	Error = "error"
)

// Config defines the configuration structure for the logger.
// It contains settings that control the behavior of the logging system.
type Config struct {
	// Level determines the minimum log level that will be output.
	// Valid values are:
	//   - "debug": Most verbose, shows all log messages
	//   - "info": Shows info, warning, and error messages
	//   - "warning": Shows only warning and error messages
	//   - "error": Shows only error messages
	//
	// The default behavior is:
	//   - In production environments: defaults to "info"
	//   - In development environments: defaults to "debug"
	//   - In other/unspecified environments: defaults to "info"
	//
	// This setting can be configured via:
	//   - YAML configuration with the "level" key
	//   - Environment variable ZAP_LOGGER_LEVEL
	Level string `yaml:"level" envconfig:"ZAP_LOGGER_LEVEL"`
}
