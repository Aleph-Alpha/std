package embedding

import (
	"context"
	"fmt"
)

// Client is the public entrypoint for computing embeddings.
//
// It hides all provider details (inference endpoints, HTTP, etc.)
// from the application layer.
type Client struct {
	provider Provider
}

// NewClient constructs a Client from Config.
// It validates the config and internally constructs the inference provider.
// Application code should depend on *Client, not on Provider or inferenceProvider.
func NewClient(cfg *Config) (*Client, error) {
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("embedding: invalid config: %w", err)
	}

	p, err := newInferenceProvider(cfg)
	if err != nil {
		return nil, fmt.Errorf("embedding: failed to create provider: %w", err)
	}

	return &Client{provider: p}, nil
}

// CreateEmbeddings executes a single embedding request.
func (c *Client) CreateEmbeddings(ctx context.Context, texts []string, strategy EmbeddingStrategy) ([][]float64, error) {
	return c.provider.CreateEmbeddings(ctx, texts, strategy)
}

// CreateBatchEmbeddings executes a batch embedding request.
func (c *Client) CreateBatchEmbeddings(ctx context.Context, texts []string, strategy EmbeddingStrategy) ([][]float64, error) {
	return c.provider.CreateBatchEmbeddings(ctx, texts, strategy)
}

// Close allows the client to release any internal resources used by the provider.
// Currently this is a no-op unless the provider implements Close().
func (c *Client) Close() error {
	if closer, ok := c.provider.(interface{ Close() error }); ok {
		return closer.Close()
	}
	return nil
}
