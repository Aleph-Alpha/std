package embedding

import "context"

// Provider contract
type Provider interface {
	// Create generates embeddings for the given texts using the specified model.
	Create(ctx context.Context, model string, texts ...string) ([][]float64, error)
}
