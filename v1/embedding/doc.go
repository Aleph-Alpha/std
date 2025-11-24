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
//	client.Create(ctx, "model-name", "hello")
//
// or batch embeddings via:
//
//	client.Create(ctx, "model-name", "a", "b", "c")
//
// This package exclusively supports OpenAI-compatible embeddings via the
// /v1/embeddings endpoint.
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
// # Summary
//
// The embedding package provides:
//
//   - A clean, stable API for OpenAI-compatible embeddings.
//   - A no-leak abstraction over the Aleph Alpha inference service.
//
// Usage:
//
//	client := embedding.NewClient(cfg)
//	client.Create(ctx, "model-name", texts...)
package embedding
