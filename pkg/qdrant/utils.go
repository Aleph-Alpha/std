package qdrant

import (
	"context"
)

// Embedding represents a dense embedding vector.
type Embedding struct {
	ID     string
	Vector []float32
	Meta   map[string]any
}

// SearchResult holds results from a similarity search.
type SearchResult struct {
	ID    string
	Score float32
	Meta  map[string]any
}

// EmbeddingRepository defines high-level operations supported by the embeddings store.
// It’s your Go equivalent of the Rust `EmbeddingRepository` trait.
type EmbeddingRepository interface {
	EnsureCollection(ctx context.Context, name string) error
	Insert(ctx context.Context, embeddings []Embedding) error
	Search(ctx context.Context, vector []float32, topK int) ([]SearchResult, error)
	Delete(ctx context.Context, ids []string) error
}
