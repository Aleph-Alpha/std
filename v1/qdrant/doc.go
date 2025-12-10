// Package qdrant provides a modular, dependency-injected client for the Qdrant vector database.
//
// The qdrant package is designed to simplify interaction with Qdrant in Go applications,
// offering a clean, testable abstraction layer for common vector database operations such as
// collection management, embedding insertion, similarity search, and deletion. It integrates
// seamlessly with the fx dependency injection framework and supports builder-style configuration.
//
// Core Features:
//
//   - Managed Qdrant client lifecycle with Fx integration
//   - Config struct supporting environment and YAML loading
//   - Automatic health checks on client initialization
//   - Safe, batched insertion of embeddings with configurable batch size
//   - Vector similarity search with abstracted SearchResult interface
//   - Type-safe collection creation and existence checks
//   - Support for payload metadata and optional vector retrieval
//   - Extensible abstraction layer for alternate vector stores (e.g., Pinecone, Postgres)
//
// Basic Usage:
//
//	import "github.com/Aleph-Alpha/std/v1/qdrant"
//
//	// Create a new client
//	client, err := qdrant.NewQdrantClient(qdrant.QdrantParams{
//		Config: &qdrant.Config{
//			Endpoint: "localhost:6334",
//			ApiKey:   "",
//		},
//	})
//	if err != nil {
//		log.Fatal(err)
//	}
//
//	collectionName := "documents"
//
//	// Insert single embedding
//	input := qdrant.EmbeddingInput{
//		ID:     "doc_1",
//		Vector: []float32{0.12, 0.43, 0.85},
//		Meta:   map[string]any{"title": "My Document"},
//	}
//	if err := client.Insert(ctx, collectionName, input); err != nil {
//		log.Fatal(err)
//	}
//
//	// Batch insert embeddings
//	batch := []qdrant.EmbeddingInput{input1, input2, input3}
//	if err := client.BatchInsert(ctx, collectionName, batch); err != nil {
//		log.Fatal(err)
//	}
//
//	// Perform similarity search
//	results, err := client.Search(ctx, qdrant.SearchRequest{
//		CollectionName: collectionName,
//		Vector:         queryVector,
//		TopK:           5,
//	})
//	for _, res := range results[0] {
//		fmt.Printf("ID=%s Score=%.4f\n", res.GetID(), res.GetScore())
//	}
//
// FX Module Integration:
//
// The package exposes an Fx module for automatic dependency injection:
//
//	app := fx.New(
//		qdrant.FXModule,
//		// other modules...
//	)
//	app.Run()
//
// Abstractions:
//
// The package defines a lightweight SearchResultInterface that encapsulates
// search results via methods such as GetID(), GetScore(), GetMeta(), and GetCollectionName().
// The underlying concrete type remains SearchResult, allowing both strong typing internally
// and loose coupling externally.
//
// Example:
//
//	type SearchResultInterface interface {
//		GetID() string
//		GetScore() float32
//		GetMeta() map[string]*qdrant.Value
//		GetCollectionName() string
//	}
//
//	type SearchResult struct { /* implements SearchResultInterface */ }
//
//	// Function signature:
//	func (c *QdrantClient) Search(ctx context.Context, vector []float32, topK int) ([]SearchResultInterface, error)
//
// # Filtering
//
// The package provides a comprehensive, type-safe filtering system for vector searches.
// Filters support boolean logic (AND, OR, NOT) and various condition types.
//
// Filter Structure:
//
//	type FilterSet struct {
//	    Must    *ConditionSet  // AND - all conditions must match
//	    Should  *ConditionSet  // OR - at least one condition must match
//	    MustNot *ConditionSet  // NOT - none of the conditions should match
//	}
//
// Condition Types:
//
//   - TextCondition: Exact string match
//   - BoolCondition: Exact boolean match
//   - IntCondition: Exact integer match
//   - TextAnyCondition: String IN operator (match any of values)
//   - IntAnyCondition: Integer IN operator
//   - TextExceptCondition: String NOT IN operator
//   - IntExceptCondition: Integer NOT IN operator
//   - TimeRangeCondition: DateTime range filter (gte, lte, gt, lt)
//   - NumericRangeCondition: Numeric range filter
//   - IsNullCondition: Check if field is null
//   - IsEmptyCondition: Check if field is empty, null, or missing
//
// Field Types (Internal vs User):
//
// The package distinguishes between system-managed and user-defined metadata:
//
//	const (
//	    InternalField FieldType = iota  // Top-level: "search_store_id"
//	    UserField                        // Nested: "custom.document_id"
//	)
//
// User fields are automatically prefixed with "custom." when querying Qdrant.
//
// Basic Filter Example:
//
//	// Filter: city = "London" AND active = true
//	filters := &qdrant.FilterSet{
//	    Must: &qdrant.ConditionSet{
//	        Conditions: []qdrant.FilterCondition{
//	            qdrant.TextCondition{Key: "city", Value: "London"},
//	            qdrant.BoolCondition{Key: "active", Value: true},
//	        },
//	    },
//	}
//
//	results, err := client.Search(ctx, qdrant.SearchRequest{
//	    CollectionName: "documents",
//	    Vector:         queryVector,
//	    TopK:           10,
//	    Filters:        filters,
//	})
//
// OR Conditions (Should):
//
//	// Filter: city = "London" OR city = "Berlin"
//	filters := &qdrant.FilterSet{
//	    Should: &qdrant.ConditionSet{
//	        Conditions: []qdrant.FilterCondition{
//	            qdrant.TextCondition{Key: "city", Value: "London"},
//	            qdrant.TextCondition{Key: "city", Value: "Berlin"},
//	        },
//	    },
//	}
//
// IN Operator (MatchAny):
//
//	// Filter: city IN ["London", "Berlin", "Paris"]
//	filters := &qdrant.FilterSet{
//	    Must: &qdrant.ConditionSet{
//	        Conditions: []qdrant.FilterCondition{
//	            qdrant.TextAnyCondition{
//	                Key:    "city",
//	                Values: []string{"London", "Berlin", "Paris"},
//	            },
//	        },
//	    },
//	}
//
// Time Range Filter:
//
//	now := time.Now()
//	yesterday := now.Add(-24 * time.Hour)
//
//	filters := &qdrant.FilterSet{
//	    Must: &qdrant.ConditionSet{
//	        Conditions: []qdrant.FilterCondition{
//	            qdrant.TimeRangeCondition{
//	                Key: "created_at",
//	                Value: qdrant.TimeRange{
//	                    Gte: &yesterday,
//	                    Lt:  &now,
//	                },
//	            },
//	        },
//	    },
//	}
//
// Complex Filter (Combined Clauses):
//
//	// Filter: (status = "published") AND (category = "tech" OR "science") AND NOT (deleted = true)
//	filters := &qdrant.FilterSet{
//	    Must: &qdrant.ConditionSet{
//	        Conditions: []qdrant.FilterCondition{
//	            qdrant.TextCondition{Key: "status", Value: "published"},
//	        },
//	    },
//	    Should: &qdrant.ConditionSet{
//	        Conditions: []qdrant.FilterCondition{
//	            qdrant.TextCondition{Key: "category", Value: "tech", FieldType: qdrant.UserField},
//	            qdrant.TextCondition{Key: "category", Value: "science", FieldType: qdrant.UserField},
//	        },
//	    },
//	    MustNot: &qdrant.ConditionSet{
//	        Conditions: []qdrant.FilterCondition{
//	            qdrant.BoolCondition{Key: "deleted", Value: true},
//	        },
//	    },
//	}
//
// UUID Filtering:
//
// UUIDs are filtered as strings using TextCondition:
//
//	filters := &qdrant.FilterSet{
//	    Must: &qdrant.ConditionSet{
//	        Conditions: []qdrant.FilterCondition{
//	            qdrant.TextCondition{
//	                Key:       "document_id",
//	                Value:     "f47ac10b-58cc-4372-a567-0e02b2c3d479",
//	                FieldType: qdrant.UserField,
//	            },
//	        },
//	    },
//	}
//
// Payload Structure Helper:
//
// The BuildPayload function creates properly structured payloads that separate
// internal and user fields:
//
//	payload := qdrant.BuildPayload(
//	    map[string]any{"search_store_id": "store-123"},  // Internal (top-level)
//	    map[string]any{"document_id": "doc-456"},        // User (under "custom.")
//	)
//
//	// Result:
//	// {
//	//     "search_store_id": "store-123",
//	//     "custom": {
//	//         "document_id": "doc-456"
//	//     }
//	// }
//
// This structure ensures user-defined filter indexes are created at the correct
// path (custom.field_name).
//
// Configuration:
//
// Qdrant can be configured via environment variables or YAML:
//
//	QDRANT_ENDPOINT=http://localhost:6334
//	QDRANT_API_KEY=your-api-key
//
// Performance Considerations:
//
// The BatchInsert method automatically splits large embedding inserts into smaller
// upserts (default batch size = 500). This minimizes memory usage and avoids timeouts
// when ingesting large datasets.
//
// Thread Safety:
//
// All exported methods on QdrantClient are safe for concurrent use by multiple goroutines.
//
// Testing:
//
// For testing and mocking, application code should depend on the public interface types
// (e.g., SearchResultInterface, EmbeddingInput) instead of concrete Qdrant structs.
// This allows replacing the QdrantClient with in-memory or mock implementations in tests.
//
// Example Mock:
//
//	type MockResult struct {
//		id    string
//		score float32
//		meta  map[string]any
//	}
//	func (m MockResult) GetID() string           { return m.id }
//	func (m MockResult) GetScore() float32       { return m.score }
//	func (m MockResult) GetMeta() map[string]any { return m.meta }
//
// Package Layout:
//
//	qdrant/
//	├── client.go        // Qdrant client implementation
//	├── operations.go    // CRUD operations (Insert, Search, Delete, etc.)
//	├── filters.go       // Type-safe filtering system
//	├── utils.go         // Shared types and interfaces
//	├── configs.go       // Configuration struct and builder methods
//	└── fx_module.go     // Fx dependency injection module
package qdrant
