package qdrant

import (
	qdrant "github.com/qdrant/go-client/qdrant"
)

// EmbeddingInput is the type the application provides to insert embeddings.
// Keeps the app decoupled from internal Qdrant SDK structs.
type EmbeddingInput struct {
	ID     string         // Unique identifier for the embedding (e.g., document ID)
	Vector []float32      // Dense vector representation of the embedding
	Meta   map[string]any // Optional metadata associated with the embedding
}

// Embedding represents a dense embedding vector.
type Embedding struct {
	ID     string         // Unique identifier (same as Qdrant point ID)
	Vector []float32      // Vector representation of the embedding
	Meta   map[string]any // Optional metadata associated with the embedding
}

// SearchResult holds results from a similarity search.
type SearchResult struct {
	ID         string
	Score      float32
	Meta       map[string]*qdrant.Value
	Vector     []float32
	Collection string
}

// GetID returns the result's unique identifier.
func (r SearchResult) GetID() string { return r.ID }

// GetScore returns the similarity score associated with the result.
func (r SearchResult) GetScore() float32 { return r.Score }

// GetMeta returns the metadata stored with the vector.
func (r SearchResult) GetMeta() map[string]*qdrant.Value { return r.Meta }

// GetVector returns the dense embedding vector if available.
func (r SearchResult) GetVector() []float32 { return r.Vector }

// HasVector reports whether the result contains a non-empty vector payload.
func (r SearchResult) HasVector() bool { return len(r.Vector) > 0 }

// GetCollectionName returns the name of the collection from which the result originated.
func (r SearchResult) GetCollectionName() string { return r.Collection }

// NewEmbedding converts a high-level EmbeddingInput into the internal Embedding type.
// Having this builder allows for future validation or normalization logic.
// For now, it performs a shallow copy.
func NewEmbedding(input EmbeddingInput) Embedding {
	return Embedding(input)
}

// SearchResultInterface is the public interface for search results.
// It provides a consistent way to access search results regardless of the underlying implementation.
type SearchResultInterface interface {
	GetID() string                     // Unique result identifier
	GetScore() float32                 // Similarity score
	GetMeta() map[string]*qdrant.Value // Metadata associated with the result
	GetVector() []float32              // Optional embedding vector
	HasVector() bool                   // Whether a vector payload is present
	GetCollectionName() string         // Name of the Qdrant collection
}
