package vectordb

import "context"

// Service is the common interface for all vector databases.
// It provides a database-agnostic abstraction for vector similarity search,
// allowing applications to switch between different vector databases
// (Qdrant, pgVector, etc.) without changing application code.
//
// Example usage:
//
//	func NewSearchService(db vectordb.Service) *SearchService {
//	    return &SearchService{db: db}
//	}
//
//	// Works with any implementation:
//	// - vectordb.NewQdrantAdapter(qdrantClient)
//	// - vectordb.NewPgVectorAdapter(pgVectorClient)
type Service interface {
	// Search performs similarity search across one or more requests.
	// Each request can target a different collection with different filters.
	// Returns:
	//   - results: slice of result slices—one []SearchResult per request
	//   - err: combined error (per-request errors and systemic errors joined)
	//
	// Example:
	//   results, err := db.Search(ctx,
	//       SearchRequest{CollectionName: "docs", Vector: vec1, TopK: 10},
	//       SearchRequest{CollectionName: "docs", Vector: vec2, TopK: 5, Filters: filters},
	//   )
	//   if err != nil {
	//       return err
	//   }
	//   for _, res := range results {
	//       // use res...
	//   }
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
