// Package qdrant provides a modular, dependency-injected client for the Qdrant vector database.
//
// The qdrant package is designed to simplify interaction with Qdrant in Go applications,
// offering a clean, testable abstraction layer for common vector database operations such as
// collection management, embedding insertion, similarity search, and deletion. It integrates
// seamlessly with the fx dependency injection framework and supports builder-style configuration.
//
// # Core Features
//
//   - Managed Qdrant client lifecycle with Fx integration
//   - Config struct supporting environment and YAML loading
//   - Automatic health checks on client initialization
//   - Safe, batched insertion of embeddings with configurable batch size
//   - Database-agnostic interface via vectordb.Service
//   - Type-safe collection creation and existence checks
//   - Support for payload metadata and optional vector retrieval
//   - Extensible abstraction layer for alternate vector stores (e.g. pgVector)
//
// # VectorDB Interface
//
// This package includes [Adapter] which implements the database-agnostic
// [vectordb.Service] interface. Use this for new projects to enable
// easy switching between vector databases:
//
//	import (
//	    "github.com/Aleph-Alpha/std/v1/vectordb"
//	    "github.com/Aleph-Alpha/std/v1/qdrant"
//	)
//
//	// Create your existing QdrantClient
//	qc, _ := qdrant.NewQdrantClient(qdrant.QdrantParams{
//	    Config: &qdrant.Config{
//	        Endpoint: "localhost",
//	        Port:     6334,
//	    },
//	})
//
//	// Create adapter for DB-agnostic usage
//	var db vectordb.Service = qdrant.NewAdapter(qc.Client())
//
// This allows switching between vector databases (Qdrant, pgVector) without
// changing application code.
//
// # Basic Usage
//
//	import (
//	    "github.com/Aleph-Alpha/std/v1/qdrant"
//	    "github.com/Aleph-Alpha/std/v1/vectordb"
//	)
//
//	// Create a new client
//	client, err := qdrant.NewQdrantClient(qdrant.QdrantParams{
//	    Config: &qdrant.Config{
//	        Endpoint: "localhost",
//	        Port:     6334,
//	    },
//	})
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	// Create adapter
//	adapter := qdrant.NewAdapter(client.Client())
//
//	collectionName := "documents"
//
//	// Ensure collection exists
//	if err := adapter.EnsureCollection(ctx, collectionName, 1536); err != nil {
//	    log.Fatal(err)
//	}
//
//	// Insert embeddings
//	inputs := []vectordb.EmbeddingInput{
//	    {
//	        ID:      "doc_1",
//	        Vector:  []float32{0.12, 0.43, 0.85, ...},
//	        Payload: map[string]any{"title": "My Document"},
//	    },
//	}
//	if err := adapter.Insert(ctx, collectionName, inputs); err != nil {
//	    log.Fatal(err)
//	}
//
//	// Perform similarity search
//	results, err := adapter.Search(ctx, vectordb.SearchRequest{
//	    CollectionName: collectionName,
//	    Vector:         queryVector,
//	    TopK:           5,
//	})
//	for _, res := range results[0] {
//	    fmt.Printf("ID=%s Score=%.4f\n", res.ID, res.Score)
//	}
//
// # FX Module Integration
//
// The package exposes an Fx module for automatic dependency injection:
//
//	app := fx.New(
//	    qdrant.FXModule,
//	    // other modules...
//	)
//	app.Run()
//
// # Search Results
//
// Search results are returned as [vectordb.SearchResult] structs with public fields:
//
//	type SearchResult struct {
//	    ID             string         // Unique identifier of the matched point
//	    Score          float32        // Similarity score
//	    Payload        map[string]any // Metadata stored with the vector
//	    Vector         []float32      // Stored embedding (if requested)
//	    CollectionName string         // Source collection name
//	}
//
// Access fields directly (no getter methods needed):
//
//	for _, result := range results[0] {
//	    fmt.Println(result.ID, result.Score, result.Payload["title"])
//	}
//
// # Filtering
//
// Filters are defined in the [vectordb] package and support boolean logic (AND, OR, NOT).
// The qdrant adapter converts these to native Qdrant filters automatically.
//
// Filter Structure:
//
//	type FilterSet struct {
//	    Must    *ConditionSet  // AND - all conditions must match
//	    Should  *ConditionSet  // OR - at least one condition must match
//	    MustNot *ConditionSet  // NOT - none of the conditions should match
//	}
//
// Condition Types (all in vectordb package):
//
//   - MatchCondition: Exact match (string, bool, int64)
//   - MatchAnyCondition: IN operator (match any of values)
//   - MatchExceptCondition: NOT IN operator
//   - NumericRangeCondition: Numeric range filter (gt, gte, lt, lte)
//   - TimeRangeCondition: DateTime range filter
//   - IsNullCondition: Check if field is null
//   - IsEmptyCondition: Check if field is empty, null, or missing
//
// Field Types (Internal vs User):
//
// The package distinguishes between system-managed and user-defined metadata:
//
//	const (
//	    InternalField FieldType = iota  // Top-level: "status"
//	    UserField                        // Prefixed: "custom.document_id"
//	)
//
// User fields are automatically prefixed with "custom." when querying Qdrant.
//
// # Filter Examples Using Convenience Constructors
//
// The vectordb package provides convenience constructors for clean filter creation:
//
// Basic Filter (Must - AND logic):
//
//	// Filter: city = "London" AND active = true
//	results, err := adapter.Search(ctx, vectordb.SearchRequest{
//	    CollectionName: "documents",
//	    Vector:         queryVector,
//	    TopK:           10,
//	    Filters: []*vectordb.FilterSet{
//	        vectordb.NewFilterSet(
//	            vectordb.Must(
//	                vectordb.NewMatch("city", "London"),
//	                vectordb.NewMatch("active", true),
//	            ),
//	        ),
//	    },
//	})
//
// OR Conditions (Should):
//
//	// Filter: city = "London" OR city = "Berlin"
//	filters := []*vectordb.FilterSet{
//	    vectordb.NewFilterSet(
//	        vectordb.Should(
//	            vectordb.NewMatch("city", "London"),
//	            vectordb.NewMatch("city", "Berlin"),
//	        ),
//	    ),
//	}
//
// IN Operator (MatchAny):
//
//	// Filter: city IN ["London", "Berlin", "Paris"]
//	filters := []*vectordb.FilterSet{
//	    vectordb.NewFilterSet(
//	        vectordb.Must(
//	            vectordb.NewMatchAny("city", "London", "Berlin", "Paris"),
//	        ),
//	    ),
//	}
//
// Numeric Range Filter:
//
//	// Filter: price >= 100 AND price < 500
//	min, max := float64(100), float64(500)
//	filters := []*vectordb.FilterSet{
//	    vectordb.NewFilterSet(
//	        vectordb.Must(
//	            vectordb.NewNumericRange("price", vectordb.NumericRange{
//	                Gte: &min,
//	                Lt:  &max,
//	            }),
//	        ),
//	    ),
//	}
//
// Time Range Filter:
//
//	// Filter: created_at >= yesterday AND created_at < now
//	now := time.Now()
//	yesterday := now.Add(-24 * time.Hour)
//	filters := []*vectordb.FilterSet{
//	    vectordb.NewFilterSet(
//	        vectordb.Must(
//	            vectordb.NewTimeRange("created_at", vectordb.TimeRange{
//	                Gte: &yesterday,
//	                Lt:  &now,
//	            }),
//	        ),
//	    ),
//	}
//
// Complex Filter (Combined Clauses):
//
//	// Filter: status = "published" AND (tag = "ml" OR tag = "ai") AND NOT deleted = true
//	filters := []*vectordb.FilterSet{
//	    vectordb.NewFilterSet(
//	        vectordb.Must(vectordb.NewMatch("status", "published")),
//	        vectordb.Should(
//	            vectordb.NewMatch("tag", "ml"),
//	            vectordb.NewMatch("tag", "ai"),
//	        ),
//	        vectordb.MustNot(vectordb.NewMatch("deleted", true)),
//	    ),
//	}
//
// User-Defined Fields:
//
// For fields stored under a custom prefix, use the User* constructors:
//
//	// Filter on user-defined field: custom.document_id = "doc-123"
//	filters := []*vectordb.FilterSet{
//	    vectordb.NewFilterSet(
//	        vectordb.Must(
//	            vectordb.NewUserMatch("document_id", "doc-123"),
//	        ),
//	    ),
//	}
//
// Multiple FilterSets (AND between sets):
//
// When you provide multiple FilterSets, they are combined with AND logic:
//
//	// Filter: (color = "red") AND (size < 20)
//	lt := float64(20)
//	filters := []*vectordb.FilterSet{
//	    vectordb.NewFilterSet(vectordb.Must(vectordb.NewMatch("color", "red"))),
//	    vectordb.NewFilterSet(vectordb.Must(vectordb.NewNumericRange("size", vectordb.NumericRange{Lt: &lt}))),
//	}
//
// # Configuration
//
// Qdrant can be configured via environment variables or YAML:
//
//	QDRANT_ENDPOINT=localhost
//	QDRANT_PORT=6334
//	QDRANT_API_KEY=your-api-key
//
// # Performance Considerations
//
// The Insert method automatically splits large embedding batches into smaller
// upserts (default batch size = 100). This minimizes memory usage and avoids timeouts
// when ingesting large datasets.
//
// # Thread Safety
//
// All exported methods on the Adapter are safe for concurrent use by multiple goroutines.
//
// # Testing
//
// For testing and mocking, depend on the [vectordb.Service] interface:
//
//	type MockVectorDB struct{}
//
//	func (m *MockVectorDB) Search(ctx context.Context, requests ...vectordb.SearchRequest) ([][]vectordb.SearchResult, error) {
//	    return [][]vectordb.SearchResult{
//	        {{ID: "doc-1", Score: 0.95, Payload: map[string]any{"title": "Test"}}},
//	    }, nil
//	}
//
//	// Use in tests:
//	var db vectordb.Service = &MockVectorDB{}
//
// # Package Layout
//
//	qdrant/
//	├── client.go        // Qdrant client wrapper and lifecycle
//	├── operations.go    // Service implementation (Adapter)
//	├── converter.go     // vectordb ↔ Qdrant type conversion
//	├── utils.go         // Qdrant-specific helper functions
//	├── configs.go       // Configuration struct
//	└── fx_module.go     // Fx dependency injection module
//
// # Related Packages
//
//   - [vectordb]: Database-agnostic types and interfaces
//   - [vectordb.FilterSet]: Filter structures for search queries
//   - [vectordb.SearchResult]: Search result type
//   - [vectordb.EmbeddingInput]: Input type for inserting vectors
package qdrant
