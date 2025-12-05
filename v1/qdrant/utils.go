package qdrant

import (
	"fmt"

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

// Collection ──────────────────────────────────────────────────────────────
// Collection
// ──────────────────────────────────────────────────────────────
//
// Collection represents a high-level, decoupled view of a Qdrant collection.
//
// It provides essential metadata about a vector collection without exposing
// Qdrant SDK types, allowing the application layer to remain independent
// of the underlying database implementation.
//
// Fields:
//   • Name        — The unique name of the collection.
//   • Status      — Current operational state (e.g., "Green", "Yellow").
//   • VectorSize  — The dimension of stored vectors (e.g., 1536).
//   • Distance    — The similarity metric used ("Cosine", "Dot", "Euclid").
//   • Vectors     — Total number of stored vectors in the collection.
//   • Points      — Total number of indexed points/documents in the collection.
//
// This struct serves as an abstraction layer between Qdrant’s low-level
// protobuf models and the higher-level application logic.

type Collection struct {
	Name       string
	Status     string
	VectorSize int
	Distance   string
	Vectors    uint64
	Points     uint64
}

// SearchRequest represents a single search request for batch operations
type SearchRequest struct {
	CollectionName string
	Vector         []float32
	TopK           int
	Filters        map[string]string //Optional: key-value filters
}

// validateSearchInput validates common search parameters
func validateSearchInput(collectionName string, vector []float32, topK int) error {
	if collectionName == "" {
		return fmt.Errorf("collection name cannot be empty")
	}
	if len(vector) == 0 {
		return fmt.Errorf("vector cannot be empty")
	}
	if topK <= 0 {
		return fmt.Errorf("topK must be greater than 0")
	}
	return nil
}

// buildFilter constructs a Qdrant filter from key-value pairs (AND logic)
func buildFilter(filters map[string]string) *qdrant.Filter {
	if len(filters) == 0 {
		return nil
	}

	conditions := make([]*qdrant.Condition, 0, len(filters))
	for key, value := range filters {
		conditions = append(conditions, &qdrant.Condition{
			ConditionOneOf: &qdrant.Condition_Field{
				Field: &qdrant.FieldCondition{
					Key: key,
					Match: &qdrant.Match{
						MatchValue: &qdrant.Match_Keyword{
							Keyword: value,
						},
					},
				},
			},
		})
	}

	return &qdrant.Filter{Must: conditions}
}

// extractVectorDetails ──────────────────────────────────────────────────────────────
// extractVectorDetails
// ──────────────────────────────────────────────────────────────
//
// extractVectorDetails safely extracts the vector size (embedding dimension)
// and distance metric (e.g., "Cosine", "Dot", "Euclid") from a Qdrant
// `CollectionInfo` object.
//
// Qdrant represents vector configuration data using a deeply nested protobuf
// structure with “oneof” wrappers. This helper navigates that hierarchy,
// performs type assertions, and guards against nil pointer dereferences.
//
// It returns:
//   • int    — vector dimension (size of embedding vectors)
//   • string — distance metric used for similarity search
//
// If any nested field is missing or of an unexpected type, the function
// gracefully returns default values (0, "").
//
// Example:
//
//	size, distance := extractVectorDetails(info)
//	log.Printf("Vector size=%d, distance=%s", size, distance)

func extractVectorDetails(info *qdrant.CollectionInfo) (int, string) {
	if info == nil ||
		info.Config == nil ||
		info.Config.Params == nil ||
		info.Config.Params.VectorsConfig == nil ||
		info.Config.Params.VectorsConfig.Config == nil {
		return 0, ""
	}

	if cfg, ok := info.Config.Params.VectorsConfig.Config.(*qdrant.VectorsConfig_Params); ok {
		return int(cfg.Params.Size), cfg.Params.Distance.String()
	}

	return 0, ""
}

// derefUint64 safely dereferences a *uint64 pointer.
// If the pointer is nil, it returns 0 instead of panicking.
func derefUint64(v *uint64) uint64 {
	if v != nil {
		return *v
	}
	return 0
}

