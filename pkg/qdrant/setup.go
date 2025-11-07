package qdrant

import (
	"context"
	"fmt"
	"log"
	"slices"
	"time"

	qdrant "github.com/qdrant/go-client/qdrant"
)

//
// ──────────────────────────────────────────────────────────────
//   QDRANT CLIENT WRAPPER
// ──────────────────────────────────────────────────────────────
//
// This file defines a thin wrapper around the official Qdrant Go client,
// providing application-level operations for managing embeddings,
// collections, and similarity search.
//
// The goal is to abstract away low-level SDK details while preserving
// fine-grained control over how Qdrant is accessed.
//
// Responsibilities:
//   • Establish and validate connectivity with Qdrant.
//   • Manage collections (create if missing).
//   • Insert, batch insert, delete, and search embeddings.
//   • Offer a safe API suitable for Fx dependency injection.
//

// QdrantClient wraps the official Qdrant Go client
// and provides higher-level operations for working with embeddings and vectors.

type QdrantClient struct {
	api     *qdrant.Client
	cfg     *Config
	started bool
}

const defaultBatchSize = 200 // default chunk size for batch inserts

// ──────────────────────────────────────────────────────────────
// NewQdrantClient
// ──────────────────────────────────────────────────────────────
//
// NewQdrantClient constructs a new instance of QdrantClient and validates
// connectivity via a health check.
//
// The Qdrant Go SDK creates lightweight gRPC connections, so this method
// performs an immediate health check to fail fast if the service is unreachable.
//
// Example:
//
//	client, _ := qdrant.NewQdrantClient(qdrant.QdrantParams{Config: cfg})
func NewQdrantClient(p QdrantParams) (*QdrantClient, error) {
	log.Printf("[Qdrant] Connecting to endpoint: %s", p.Config.Endpoint)

	client, err := qdrant.NewClient(&qdrant.Config{
		Host:   p.Config.Endpoint,
		APIKey: p.Config.ApiKey,
	})
	if err != nil {
		return nil, fmt.Errorf("[Qdrant] failed to initialize client: %w", err)
	}

	qc := &QdrantClient{
		api:     client,
		cfg:     p.Config,
		started: true,
	}

	if err := qc.healthCheck(); err != nil {
		return nil, fmt.Errorf("[Qdrant] health check failed: %w", err)
	}

	log.Println("[Qdrant] Client connected successfully")
	return qc, nil
}

// ──────────────────────────────────────────────────────────────
// healthCheck
// ──────────────────────────────────────────────────────────────
//
// healthCheck verifies the availability of the Qdrant service
// by calling the `/healthz` endpoint through the SDK.
//
// It should be lightweight and fast — typically used during startup or readiness probes.
func (c *QdrantClient) healthCheck() error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if c.api == nil {
		return fmt.Errorf("[Qdrant] client not initialized")
	}

	resp, err := c.api.HealthCheck(ctx)
	if err != nil {
		return fmt.Errorf("[Qdrant] health check failed: %w", err)
	}

	log.Printf("[Qdrant] Health check passed (title=%s, version=%s, endpoint=%s)", resp.Title, resp.Version, c.cfg.Endpoint)

	return nil
}

// ──────────────────────────────────────────────────────────────
// Close
// ──────────────────────────────────────────────────────────────
//
// Close gracefully shuts down the Qdrant client.
//
// Since the official Qdrant Go SDK doesn’t maintain persistent connections,
// this is currently a no-op. It exists for lifecycle symmetry and future safety.
func (c *QdrantClient) Close() error {
	log.Println("[Qdrant] closing client (no-op)")
	return nil
}

// ──────────────────────────────────────────────────────────────
// EnsureCollection
// ──────────────────────────────────────────────────────────────
//
// EnsureCollection verifies if a given collection exists, and creates it if missing.
//
// It’s safe to call this multiple times — if the collection already exists,
// the function exits early. This pattern simplifies startup logic for embedding
// services that may bootstrap their own Qdrant collections.
func (c *QdrantClient) EnsureCollection(ctx context.Context, name string) error {
	if name == "" {
		return fmt.Errorf("collection name cannot be empty")
	}

	collections, err := c.api.ListCollections(ctx)
	if err != nil {
		return fmt.Errorf("[Qdrant] failed to list collections: %w", err)
	}

	if slices.Contains(collections, name) {
		log.Printf("[Qdrant] Collection '%s' already exists", name)
		return nil
	}

	log.Printf("[Qdrant] Collection '%s' not found, creating it...", name)

	req := &qdrant.CreateCollection{
		CollectionName: name,
		VectorsConfig: qdrant.NewVectorsConfig(&qdrant.VectorParams{
			Size:     1536,                   // default dimension (model-dependent)
			Distance: qdrant.Distance_Cosine, // cosine similarity
		}),
	}

	if err := c.api.CreateCollection(ctx, req); err != nil {
		return fmt.Errorf("[Qdrant] failed to create collection '%s': %w", name, err)
	}

	log.Printf("[Qdrant] Created collection '%s' successfully", name)
	return nil
}

// ──────────────────────────────────────────────────────────────
// Insert
// ──────────────────────────────────────────────────────────────
//
// Insert adds a single embedding to Qdrant.
//
// Internally, it reuses the BatchInsert logic to ensure consistent handling
// of payload serialization and error management.
func (c *QdrantClient) Insert(ctx context.Context, input EmbeddingInput) error {
	return c.BatchInsert(ctx, []EmbeddingInput{input})
}

// ──────────────────────────────────────────────────────────────
// BatchInsert
// ──────────────────────────────────────────────────────────────
//
// BatchInsert efficiently inserts multiple embeddings in batches
// to reduce network overhead.
//
// This method is safe to call for large datasets — it will automatically
// split inserts into smaller chunks (`defaultBatchSize`) and perform
// multiple upserts sequentially.
//
// Logs batch indices and collection name for debugging.
func (c *QdrantClient) BatchInsert(ctx context.Context, inputs []EmbeddingInput) error {
	if len(inputs) == 0 {
		return nil
	}

	// Convert all inputs into internal embeddings
	embeddings := make([]Embedding, len(inputs))
	for i, in := range inputs {
		embeddings[i] = NewEmbedding(in)
	}

	for start := 0; start < len(embeddings); start += defaultBatchSize {
		end := start + defaultBatchSize
		if end > len(embeddings) {
			end = len(embeddings)
		}
		batch := embeddings[start:end]

		if err := c.upsertBatch(ctx, batch); err != nil {
			return fmt.Errorf("[Qdrant] batch upsert failed at [%d:%d]: %w", start, end, err)
		}
		log.Printf("[Qdrant] Inserted batch [%d:%d] (collection=%s)", start, end, c.cfg.Collection)
	}

	return nil
}

// ──────────────────────────────────────────────────────────────
// upsertBatch
// ──────────────────────────────────────────────────────────────
//
// upsertBatch sends a single `Upsert` request for a slice of embeddings.
//
// Converts Embedding structs into Qdrant’s `PointStruct` objects and
// triggers a blocking insert (`Wait=true`) to ensure data persistence
// before returning.
func (c *QdrantClient) upsertBatch(ctx context.Context, batch []Embedding) error {
	points := make([]*qdrant.PointStruct, 0, len(batch))
	for _, e := range batch {
		points = append(points, &qdrant.PointStruct{
			Id:      qdrant.NewID(e.ID),
			Vectors: qdrant.NewVectors(e.Vector...),
			Payload: qdrant.NewValueMap(e.Meta),
		})
	}

	wait := true
	req := &qdrant.UpsertPoints{
		CollectionName: c.cfg.Collection,
		Points:         points,
		Wait:           &wait,
	}

	if _, err := c.api.Upsert(ctx, req); err != nil {
		return fmt.Errorf("[Qdrant] upsert failed: %w", err)
	}
	return nil
}

// ──────────────────────────────────────────────────────────────
// Search
// ──────────────────────────────────────────────────────────────
//
// Search performs a similarity search in the configured collection.
//
// Parameters:
//   - vector — the query embedding to search against.
//   - topK   — maximum number of nearest results to return.
//
// Returns:
//
//	A slice of `SearchResultInterface` instances representing the
//	most similar stored embeddings.
//
// Logs the total count of hits for observability.
func (c *QdrantClient) Search(ctx context.Context, vector []float32, topK int) ([]SearchResultInterface, error) {
	limit := uint64(topK)
	req := &qdrant.QueryPoints{
		CollectionName: c.cfg.Collection,
		Query:          qdrant.NewQuery(vector...),
		Limit:          &limit,
		WithPayload:    qdrant.NewWithPayload(true),
	}

	resp, err := c.api.Query(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("[Qdrant] search failed: %w", err)
	}

	results := make([]SearchResultInterface, 0, len(resp))
	for _, r := range resp {
		var id string
		switch v := r.Id.PointIdOptions.(type) {
		case *qdrant.PointId_Num:
			id = fmt.Sprintf("%d", v.Num)
		case *qdrant.PointId_Uuid:
			id = v.Uuid
		default:
			return nil, fmt.Errorf("[Qdrant] unexpected PointId type: %T", v)
		}

		results = append(results, SearchResult{
			ID:    id,
			Score: r.Score,
			Meta:  r.Payload,
		})
	}

	log.Printf("[Qdrant] Search returned %d results", len(results))
	return results, nil
}

// ──────────────────────────────────────────────────────────────
// Delete
// ──────────────────────────────────────────────────────────────
//
// Delete removes embeddings from a collection by their IDs.
//
// It constructs a `DeletePoints` request containing a list of `PointId`s,
// waits synchronously for completion, and logs the operation status.
func (c *QdrantClient) Delete(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}

	qdrantIDs := make([]*qdrant.PointId, 0, len(ids))
	for _, id := range ids {
		qdrantIDs = append(qdrantIDs, qdrant.NewID(id))
	}

	wait := true
	req := &qdrant.DeletePoints{
		CollectionName: c.cfg.Collection,
		Points: &qdrant.PointsSelector{
			PointsSelectorOneOf: &qdrant.PointsSelector_Points{
				Points: &qdrant.PointsIdsList{Ids: qdrantIDs},
			},
		},
		Wait: &wait,
	}

	resp, err := c.api.Delete(ctx, req)
	if err != nil {
		return fmt.Errorf("[Qdrant] delete failed: %w", err)
	}

	log.Printf("[Qdrant] Delete completed (status=%s, collection=%s)",
		resp.Status.String(), c.cfg.Collection)
	return nil
}
