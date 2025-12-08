// Package sparseembedding provides a type-safe HTTP client for interacting with
// the BM25 sparse embedding API service.
//
// # Overview
//
// The sparseembedding package offers a client generated from an OpenAPI specification
// for computing BM25 sparse embeddings. BM25 (Best Matching 25) is a ranking function
// used to estimate the relevance of documents to a given search query, producing sparse
// vector embeddings suitable for semantic search and information retrieval tasks.
//
// # Basic Usage
//
// Create a new client:
//
//	client, err := sparseembedding.NewClient("https://api.example.com")
//	if err != nil {
//		log.Fatal("Failed to create client", err, nil)
//	}
//
// For easier response handling, use ClientWithResponses:
//
//	clientWithResponses, err := sparseembedding.NewClientWithResponses("https://api.example.com")
//	if err != nil {
//		log.Fatal("Failed to create client", err, nil)
//	}
//
// Generate BM25 embedding:
//
//	ctx := context.Background()
//	request := sparseembedding.BM25EmbedRequest{
//		Text:     "This is a sample text for embedding",
//		Language: sparseembedding.English, // Optional: omit for auto-detection
//		AverageWordCount: func() *int {
//			count := 256 // Optional: default is 256
//			return &count
//		}(),
//	}
//
//	response, err := clientWithResponses.EmbedBm25EmbedBm25PostWithResponse(ctx, request)
//	if err != nil {
//		log.Error("Failed to generate embedding", err, nil)
//		return
//	}
//
//	if response.StatusCode() == 200 && response.JSON200 != nil {
//		embedding := response.JSON200
//		// Use the sparse embedding
//		// embedding.Indices contains the token indices
//		// embedding.Values contains the corresponding BM25 scores
//	}
//
// # Client Options
//
// The client supports various configuration options:
//
// Custom HTTP client:
//
//	httpClient := &http.Client{
//		Timeout: 30 * time.Second,
//	}
//	client, err := sparseembedding.NewClient(
//		"https://api.example.com",
//		sparseembedding.WithHTTPClient(httpClient),
//	)
//
// Request editor for authentication:
//
//	client, err := sparseembedding.NewClient(
//		"https://api.example.com",
//		sparseembedding.WithRequestEditorFn(func(ctx context.Context, req *http.Request) error {
//			req.Header.Set("Authorization", "Bearer your-token")
//			return nil
//		}),
//	)
//
// # Supported Languages
//
// The BM25 embedding service supports the following languages:
//
//   - Arabic, Basque, Catalan, Danish, Dutch, English, Finnish, French, German,
//     Greek, Hungarian, Indonesian, Italian, Norwegian, Portuguese, Romanian,
//     Russian, Spanish, Swedish, Turkish
//
// If no language is specified, the service will automatically detect the language
// of the input text.
//
// # Sparse Embedding Format
//
// The BM25 embedding is returned as a sparse vector with two arrays:
//
//   - Indices: Array of integers representing token indices
//   - Values: Array of float32 values representing BM25 scores for each token
//
// This sparse representation is memory-efficient as it only stores non-zero values,
// making it suitable for large vocabulary spaces.
//
// # Average Word Count
//
// The average_word_count parameter represents the average word count after stemming
// and removal of stopwords. Since there's no precise way to calculate this beforehand,
// a good estimate is approximately 60% of the original word count. If not specified,
// the default value is 256.
//
// # Error Handling
//
// The client returns structured error responses:
//
//   - 200 OK: Successful response with SparseEmbedding in JSON200
//   - 422 Unprocessable Entity: Validation error with details in JSON422
//
// # Code Generation
//
// The client code is automatically generated from the OpenAPI specification using
// oapi-codegen. To regenerate the client after updating api.yml:
//
//	cd v1/sparseembedding
//	go generate
//
// This will regenerate client.gen.go based on the OpenAPI specification in api.yml.
//
// # Thread Safety
//
// The Client struct is safe for concurrent use by multiple goroutines. Each HTTP
// request is independent and does not modify shared state.
package sparseembedding

