package qdrant

import (
	"context"
	"fmt"
	"log"
	"slices"

	qdrant "github.com/qdrant/go-client/qdrant"
)

// EnsureCollection ──────────────────────────────────────────────────────────────
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

// Insert ──────────────────────────────────────────────────────────────
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

// BatchInsert ──────────────────────────────────────────────────────────────
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

// GetCollection ──────────────────────────────────────────────────────────────
// GetCollection
// ──────────────────────────────────────────────────────────────
//
// GetCollection retrieves detailed information about a specific Qdrant collection.
//
// It returns the Qdrant `CollectionInfo` struct if found, or an error if
// the collection doesn’t exist.
//
// Example:
//
//	info, err := client.GetCollection(ctx, "my_collection")
//	if err != nil {
//	    log.Printf("Collection not found or error: %v", err)
//	} else {
//	    log.Printf("Collection '%s' has vector size %d", "my_collection", info.VectorsCount)
//	}
func (c *QdrantClient) GetCollection(ctx context.Context, name string) (*qdrant.CollectionInfo, error) {
	if c.api == nil {
		return nil, fmt.Errorf("[Qdrant] client not initialized")
	}

	if name == "" {
		return nil, fmt.Errorf("collection name cannot be empty")
	}

	info, err := c.api.GetCollectionInfo(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("[Qdrant] failed to get collection '%s': %w", name, err)
	}

	log.Printf("[Qdrant] Collection '%s' retrieved (vectors=%d, status=%s)",
		name, info.VectorsCount, info.Status.String())

	return info, nil
}

// Search ──────────────────────────────────────────────────────────────
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

// Delete ──────────────────────────────────────────────────────────────
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
