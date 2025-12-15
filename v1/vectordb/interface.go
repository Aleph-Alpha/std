package vectordb

import "context"

// VectorDBService is the common interface for all vector databases.
// It provides a database-agnostic abstraction for vector similarity search,
// allowing applications to switch between different vector databases
// (Qdrant, Weaviate, Pinecone, etc.) without changing application code.
//
// Example usage:
//
//	func NewSearchService(db vectordb.Service) *SearchService {
//	    return &SearchService{db: db}
//	}
//
//	// Works with any implementation:
//	// - vectordb.NewQdrantAdapter(qdrantClient)
//	// - vectordb.NewWeaviateAdapter(weaviateClient)
type Service interface {
	// Search performs similarity search across one or more requests.
	// Each request can target a different collection with different filters.
	// Returns a slice of result slices—one []SearchResult per request.
	//
	// Example:
	//   results, err := db.Search(ctx,
	//       SearchRequest{CollectionName: "docs", Vector: vec1, TopK: 10},
	//       SearchRequest{CollectionName: "docs", Vector: vec2, TopK: 5, Filters: filters},
	//   )
	//   // results[0] = results for first query
	//   // results[1] = results for second query
	Search(ctx context.Context, requests ...SearchRequest) ([][]SearchResult, error)

	// Insert adds embeddings to a collection.
	// Uses batch processing internally for efficiency.
	Insert(ctx context.Context, collectionName string, inputs []EmbeddingInput) error

	// Delete removes points by their IDs from a collection.
	Delete(ctx context.Context, collection string, ids []string) error

	// EnsureCollection creates a collection if it doesn't exist.
	// Safe to call multiple times—no-op if collection already exists.
	EnsureCollection(ctx context.Context, name string, vectorSize uint64) error

	// GetCollection retrieves metadata about a collection.
	GetCollection(ctx context.Context, name string) (*Collection, error)

	// ListCollections returns names of all collections.
	ListCollections(ctx context.Context) ([]string, error)
}
