package embedding

import (
	"fmt"
	"os"
	"strconv"
)

type Config struct {
	// Which provider to use: "openai" or "inference"
	Provider string

	// Shared/API config
	APIKey   string // For OpenAI or any provider that requires API key
	Endpoint string // For OpenAI (embeddings endpoint) or inference base URL

	// Inference API specific (used by providers/inference.go)
	ServiceToken string // e.g., PHARIA internal service token
	HTTPTimeoutS int    // http timeout seconds (default 30)
}

// NewConfig reads from environment (no extra deps).
func NewConfig() *Config {
	timeout := 30
	if v := os.Getenv("EMBEDDING_HTTP_TIMEOUT_SECONDS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			timeout = n
		}
	}
	return &Config{
		Provider:     getenvDefault("EMBEDDING_PROVIDER", "openai"), // "openai" | "inference"
		APIKey:       os.Getenv("EMBEDDING_API_KEY"),
		Endpoint:     getenvDefault("EMBEDDING_ENDPOINT", "https://api.openai.com/v1/embeddings"),
		ServiceToken: os.Getenv("EMBEDDING_SERVICE_TOKEN"),
		HTTPTimeoutS: timeout,
	}
}

func getenvDefault(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func (c *Config) Validate() error {
	if c.Provider == "openai" && c.APIKey == "" {
		return fmt.Errorf("openai provider requires EMBEDDING_API_KEY")
	}
	if c.Provider == "inference" && c.ServiceToken == "" {
		return fmt.Errorf("inference provider requires EMBEDDING_SERVICE_TOKEN")
	}
	return nil
}
