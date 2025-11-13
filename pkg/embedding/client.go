package embedding

import (
	"context"
)

// Client is a thin facade that delegates all requests to the underlying Provider.
type Client struct {
	provider Provider
}

// NewClient constructs a Client from an already-instantiated Provider.
// Provider is created by FX (NewInferenceProvider).
func NewClient(p Provider) *Client {
	return &Client{provider: p}
}

// CreateEmbeddings executes either a single or batch embedding query.
func (c *Client) CreateEmbeddings(ctx context.Context, texts []string, strategy EmbeddingStrategy) ([][]float64, error) {
	return c.provider.CreateEmbeddings(ctx, texts, strategy)
}

// CreateBatchEmbeddings delegates to the underlying provider.
func (c *Client) CreateBatchEmbeddings(ctx context.Context, texts []string, strategy EmbeddingStrategy) ([][]float64, error) {
	return c.provider.CreateBatchEmbeddings(ctx, texts, strategy)
}
