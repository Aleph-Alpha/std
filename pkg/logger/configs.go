package logger

const (
	Debug   = "debug"
	Info    = "info"
	Warning = "warning"
	Error   = "error"
)

type Config struct {
	// 1. production -> INFO
	// 2. development -> DEBUG
	// else -> INFO
	Level string `yaml:"level" envconfig:"ZAP_LOGGER_LEVEL"`
}
