package embedding

import (
	"fmt"
	"os"
	"strconv"
)

// EMBEDDING_ENDPOINT → must be the root (no /v1/embeddings appended)
//
// DO NOT include suffix like `/v1/embeddings` or `/semantic_embed`.
// The provider appends paths automatically.

type Config struct {
	// Inference endpoint and auth
	Endpoint     string // Base URL of the Aleph Alpha inference API
	ServiceToken string // PHARIA internal service token
	HTTPTimeoutS int    // HTTP timeout seconds (default 30)
}

// NewConfig reads from environment variables.
func NewConfig() *Config {
	timeout := 30
	if v := os.Getenv("EMBEDDING_HTTP_TIMEOUT_SECONDS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			timeout = n
		}
	}

	return &Config{
		// Default should point to Aleph Alpha inference API
		Endpoint:     os.Getenv("EMBEDDING_ENDPOINT"),
		ServiceToken: os.Getenv("EMBEDDING_SERVICE_TOKEN"),
		HTTPTimeoutS: timeout,
	}
}

// Validate ensures required fields are present.
func (c *Config) Validate() error {
	if c.Endpoint == "" {
		return fmt.Errorf("embedding: missing EMBEDDING_ENDPOINT")
	}
	if c.ServiceToken == "" {
		return fmt.Errorf("embedding: missing EMBEDDING_SERVICE_TOKEN")
	}
	return nil
}
