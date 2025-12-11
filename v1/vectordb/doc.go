// Package vectordb provides a database-agnostic abstraction for vector similarity search.
//
// # Overview
//
// This package defines a common interface [VectorDBService] that can be implemented
// by different vector database adapters (Qdrant, Weaviate, Pinecone, etc.), allowing
// applications to switch between databases without changing application code.
//
// # Architecture
//
//	┌─────────────────────────────────────────────────────────────┐
//	│                    Application Layer                        │
//	│  (uses vectordb.VectorDBService - no DB-specific imports)  │
//	└──────────────────────────┬──────────────────────────────────┘
//	                           │
//	                           ▼
//	┌─────────────────────────────────────────────────────────────┐
//	│                  vectordb.VectorDBService                   │
//	│          (common interface + DB-agnostic types)             │
//	└──────────────────────────┬──────────────────────────────────┘
//	                           │
//	        ┌──────────────────┼──────────────────┐
//	        ▼                  ▼                  ▼
//	┌───────────────┐  ┌───────────────┐  ┌───────────────┐
//	│ qdrant.Adapter│  │weaviate.Adapter│ │pinecone.Adapter│
//	│  (implements) │  │  (implements)  │ │  (implements)  │
//	└───────────────┘  └───────────────┘  └───────────────┘
//
// # Benefits
//
//   - Single Source of Truth: Filter types, search interfaces, and result types defined once.
//   - Easy to Add New DBs: Just add a new adapter; consuming projects don't change.
//   - Consistent API: All projects using std get the same interface.
//   - Testability: Mock the interface once, works for all DBs.
//
// # Usage
//
// In your application, depend only on the vectordb interface:
//
//	import "github.com/Aleph-Alpha/std/v1/vectordb"
//
//	type SearchService struct {
//	    db vectordb.VectorDBService
//	}
//
//	func NewSearchService(db vectordb.VectorDBService) *SearchService {
//	    return &SearchService{db: db}
//	}
//
//	func (s *SearchService) Search(ctx context.Context, query string, vector []float32) ([]vectordb.SearchResult, error) {
//	    results, err := s.db.Search(ctx, vectordb.SearchRequest{
//	        CollectionName: "documents",
//	        Vector:         vector,
//	        TopK:           10,
//	        Filters: &vectordb.FilterSet{
//	            Must: &vectordb.ConditionSet{
//	                Conditions: []vectordb.FilterCondition{
//	                    vectordb.NewMatch("status", "published"),
//	                },
//	            },
//	        },
//	    })
//	    if err != nil {
//	        return nil, err
//	    }
//	    return results[0], nil
//	}
//
// # Wire Up with Qdrant
//
// In your main or DI setup:
//
//	import (
//	    "github.com/Aleph-Alpha/std/v1/vectordb"
//	    "github.com/Aleph-Alpha/std/v1/qdrant"
//	)
//
//	func main() {
//	    // Create Qdrant client (with health checks, config, etc.)
//	    qc, _ := qdrant.NewQdrantClient(qdrant.QdrantParams{
//	        Config: &qdrant.Config{Endpoint: "localhost", Port: 6334},
//	    })
//
//	    // Create adapter for DB-agnostic usage
//	    db := qdrant.NewVectorDBAdapter(qc.API())
//
//	    // Use in application
//	    svc := NewSearchService(db)
//	    // ...
//	}
//
// # Package Layout
//
//	vectordb/
//	├── interface.go      # VectorDBService interface
//	├── types.go          # SearchRequest, SearchResult, EmbeddingInput, Collection
//	├── filters.go        # FilterSet, FilterCondition, convenience constructors
//	└── doc.go            # This file
//
//	qdrant/                      # Qdrant package (includes adapter)
//	├── client.go                # QdrantClient wrapper
//	├── adapter.go               # VectorDBAdapter - implements VectorDBService
//	├── vectordb_converter.go    # vectordb types → qdrant types
//	├── operations.go            # Direct Qdrant operations
//	├── filters.go               # Qdrant-specific filters
//	└── ...
//
// Future adapters would live in their own packages:
//
//	weaviate/             # (future) Weaviate adapter
//	pinecone/             # (future) Pinecone adapter
//
// # Filter Types
//
// The package provides DB-agnostic filter conditions:
//
//	| Type                  | Description                  | SQL Equivalent                    |
//	|-----------------------|------------------------------|-----------------------------------|
//	| MatchCondition        | Exact value match            | WHERE field = value               |
//	| MatchAnyCondition     | Value in set                 | WHERE field IN (...)              |
//	| MatchExceptCondition  | Value not in set             | WHERE field NOT IN (...)          |
//	| NumericRangeCondition | Numeric range                | WHERE field >= min AND field <= max|
//	| TimeRangeCondition    | Datetime range               | WHERE created_at BETWEEN ...      |
//	| IsNullCondition       | Field is null                | WHERE field IS NULL               |
//	| IsEmptyCondition      | Field is empty/null/missing  | WHERE field IS NULL OR field = '' |
//
// Use convenience constructors for cleaner code:
//
//	// Internal field (top-level in payload)
//	vectordb.NewMatch("status", "published")
//
//	// User-defined field (stored under "custom." prefix)
//	vectordb.NewUserMatch("category", "research")
//
//	// Range conditions
//	vectordb.NewNumericRange("price", &min, &max)
//	vectordb.NewTimeRange("created_at", &startTime, &endTime)
package vectordb
