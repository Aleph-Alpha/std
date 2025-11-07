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
//	import "github.com/Aleph-Alpha/data-go-packages/pkg/qdrant"
//
//	// Create a new client
//	client, err := qdrant.NewQdrantClient(qdrant.QdrantParams{
//		Config: &qdrant.Config{
//			Endpoint:   "localhost:6334",
//			ApiKey:     "",
//			Collection: "documents",
//		},
//	})
//	if err != nil {
//		log.Fatal(err)
//	}
//
//	// Insert single embedding
//	input := qdrant.EmbeddingInput{
//		ID:     "doc_1",
//		Vector: []float32{0.12, 0.43, 0.85},
//		Meta:   map[string]any{"title": "My Document"},
//	}
//	if err := client.Insert(ctx, input); err != nil {
//		log.Fatal(err)
//	}
//
//	// Batch insert embeddings
//	batch := []qdrant.EmbeddingInput{input1, input2, input3}
//	if err := client.BatchInsert(ctx, batch); err != nil {
//		log.Fatal(err)
//	}
//
//	// Perform similarity search
//	results, err := client.Search(ctx, queryVector, 5)
//	for _, res := range results {
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
// Configuration:
//
// Qdrant can be configured via environment variables or YAML:
//
//	QDRANT_ENDPOINT=http://localhost:6334
//	QDRANT_API_KEY=your-api-key
//	QDRANT_COLLECTION=documents
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
//	├── setup.go         // Qdrant client implementation
//	├── utils.go         // Shared types and interfaces
//	└── config.go        // Configuration and Fx module definitions
package qdrant
