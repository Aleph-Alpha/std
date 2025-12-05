package qdrant

import (
	"time"
)

// Config holds connection and behavior settings for the Qdrant client.
//
// It is intentionally minimal, readable, and easy to override from environment
// variables, YAML, or programmatically via helper methods.
//
// Example (programmatic):
//
//	cfg := qdrant.DefaultConfig()
//	cfg.Endpoint = "http://localhost:6334"
//	cfg.ApiKey = os.Getenv("QDRANT_API_KEY")
//	cfg.Timeout = 10 * time.Second
//
// Example (builder style):
//
//	cfg := qdrant.FromEndpoint("http://localhost:6334").
//	    WithApiKey(os.Getenv("QDRANT_API_KEY")).
//	    WithTimeout(10 * time.Second)
type Config struct {
	// Hostname of the Qdrant server, e.g. "localhost".
	Endpoint string `yaml:"endpoint" env:"QDRANT_ENDPOINT"`

	// gRPC port of the Qdrant server. Defaults to 6334.
	Port int `yaml:"port" env:"QDRANT_PORT"`

	// Optional authentication token for secured deployments.
	ApiKey string `yaml:"api_key" env:"QDRANT_API_KEY"`

	// Default collection name this client operates on.
	DefaultCollection string `yaml:"default_collection" env:"QDRANT_DEFAULT_COLLECTION"`

	// Maximum request duration before timing out.
	Timeout time.Duration `yaml:"timeout" env:"QDRANT_TIMEOUT"`

	// Connection establishment timeout.
	ConnectTimeout time.Duration `yaml:"connect_timeout" env:"QDRANT_CONNECT_TIMEOUT"`

	// Whether to keep idle connections open for reuse.
	KeepAlive bool `yaml:"keep_alive" env:"QDRANT_KEEP_ALIVE"`

	// Enable gzip compression for requests.
	Compression bool `yaml:"compression" env:"QDRANT_COMPRESSION"`

	// Whether to perform version compatibility checks between client and server.
	CheckCompatibility bool `yaml:"check_compatibility" env:"QDRANT_CHECK_COMPATIBILITY"`
}

// DefaultConfig provides sensible defaults for most use cases.
func DefaultConfig() *Config {
	return &Config{
		Endpoint:           "localhost",
		Port:               6334,
		Timeout:            5 * time.Second,
		ConnectTimeout:     5 * time.Second,
		KeepAlive:          true,
		Compression:        false,
		CheckCompatibility: true,
	}
}

// FromEndpoint returns a default config pre-filled with a specific endpoint.
func FromEndpoint(url string) *Config {
	cfg := DefaultConfig()
	cfg.Endpoint = url
	return cfg
}

// Builder-style helpers (optional, ergonomic)
func (c *Config) WithApiKey(key string) *Config {
	c.ApiKey = key
	return c
}

func (c *Config) WithTimeout(d time.Duration) *Config {
	c.Timeout = d
	return c
}

func (c *Config) WithConnectTimeout(d time.Duration) *Config {
	c.ConnectTimeout = d
	return c
}

func (c *Config) WithCompression(enabled bool) *Config {
	c.Compression = enabled
	return c
}

func (c *Config) WithKeepAlive(enabled bool) *Config {
	c.KeepAlive = enabled
	return c
}

func (c *Config) WithCompatibilityCheck(enabled bool) *Config {
	c.CheckCompatibility = enabled
	return c
}
