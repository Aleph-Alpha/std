// Package embedding provides a unified, high-level API for computing text
// embeddings through the Aleph Alpha inference service.
//
// # Overview
//
// The package exposes a single public entrypoint, Client, which hides all
// low-level HTTP details, endpoint paths, authentication, and model-specific
// behavior.
//
// A client is constructed using:
//
//	client, err := embedding.NewClient(cfg)
//
// Once created, the client can generate embeddings via:
//
//	client.CreateEmbeddings(ctx, []string{"hello"}, strategy)
//
// or batch embeddings via:
//
//	client.CreateBatchEmbeddings(ctx, []string{"a", "b", "c"}, strategy)
//
// All endpoint routing is automatically determined by the EmbeddingStrategy.
//
// # Strategy-Based Embeddings
//
// Embedding behavior is configured using EmbeddingStrategy. Each strategy type
// maps to a specific model family and inference endpoint:
//
//   - StrategySemantic
//     Uses the semantic embedding endpoint.
//     Supports:
//
//   - representation: "symmetric" or "asymmetric"
//
//   - optional compress_to_size (integer dimensions)
//
//   - optional hybrid_index (e.g. "bm25")
//
//   - optional normalization
//
//   - StrategyInstruct
//     Uses the instruct embedding endpoint.
//     Requires an instruction:
//     { query: "...", document: "..." }
//     Batch requests are internally expanded into repeated single-item calls.
//
//   - StrategyVLLM
//     Uses the OpenAI-compatible /v1/embeddings endpoint.
//     Supports both single and batch embeddings without special handling.
//
// Example strategy:
//
//	strategy := embedding.EmbeddingStrategy{
//	    Type:            embedding.StrategySemantic,
//	    Model:           "luminous-base",
//	    Representation:  pointerTo("symmetric"),
//	    CompressToSize:  pointerTo(128),
//	}
//
// # Configuration
//
// Configuration is sourced from environment variables and constructed by:
//
//	cfg := embedding.NewConfig()
//
// Required variables:
//
//   - EMBEDDING_ENDPOINT
//     Base URL of the inference service (no trailing path or slash).
//
//   - EMBEDDING_SERVICE_TOKEN
//     Internal PHARIA service token for authentication.
//
// Optional variables:
//
//   - EMBEDDING_HTTP_TIMEOUT_SECONDS
//     Request timeout (default: 30 seconds).
//
// Configuration correctness can be verified via:
//
//	if err := cfg.Validate(); err != nil { ... }
//
// # Dependency Injection (Fx)
//
// A ready-to-use Fx module is provided:
//
//	embedding.FXModule
//
// which supplies:
//
//   - *embedding.Config
//   - *embedding.Client
//
// and registers a lifecycle hook to clean up HTTP resources on shutdown.
//
// Example:
//
//	app := fx.New(
//	    embedding.FXModule,
//	    fx.Invoke(func(c *embedding.Client) {
//	        // Use embeddings
//	    }),
//	)
//
// # Design Notes
//
//   - Only a single provider implementation exists (inferenceProvider). It is
//     unexported on purpose to keep all endpoint-level complexity internal.
//
//   - The Client exposes a stable, minimal API surface:
//     CreateEmbeddings
//     CreateBatchEmbeddings
//     with all routing logic, JSON shapes, and backend rules handled internally.
//
//   - Representation values are strictly validated:
//     "symmetric" or "asymmetric"
//     with automatic fallback to "symmetric".
//
//   - compress_to_size is forwarded directly to the backend as an integer.
//     No variant mapping (e.g., "variant128") is performed.
//
//   - Batch instruct embeddings are expanded into multiple sequential calls,
//     matching the backend's behavior.
//
// # Summary
//
// The embedding package provides:
//
//   - A clean, stable API for all embedding types (semantic, instruct, vLLM).
//   - Automatic routing based on strategy.
//   - Consistent behavior across models and endpoints.
//   - A no-leak abstraction over the Aleph Alpha inference service.
//
// For most applications, only three operations are needed:
//
//	client := embedding.NewClient(cfg)
//	client.CreateEmbeddings(ctx, texts, strategy)
//	client.CreateBatchEmbeddings(ctx, texts, strategy)
//
// Everything else—including endpoint selection, request formats,
// authentication, batching behavior, and normalization—is handled internally.
package embedding
