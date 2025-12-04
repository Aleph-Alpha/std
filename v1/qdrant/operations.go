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
// GetCollection retrieves detailed metadata about a specific collection
// from the connected Qdrant instance.
//
// It returns a high-level, decoupled `Collection` struct containing
// core details such as:
//   • Collection name
//   • Status (e.g., "Green", "Yellow")
//   • Total vectors and points
//   • Vector size (embedding dimension)
//   • Distance metric (e.g., "Cosine", "Dot", "Euclid")
//
// This abstraction intentionally hides Qdrant SDK internals (`qdrant.CollectionInfo`)
// so that the application layer remains independent of Qdrant’s client library.
//
// Example:
//
//	collection, err := client.GetCollection(ctx, "my_collection")
//	if err != nil {
//	    log.Printf("Failed to fetch collection info: %v", err)
//	    return
//	}
//	log.Printf("Collection '%s': vectors=%d, points=%d, vector_size=%d, distance=%s",
//	    collection.Name,
//	    collection.Vectors,
//	    collection.Points,
//	    collection.VectorSize,
//	    collection.Distance,
//	)

func (c *QdrantClient) GetCollection(ctx context.Context, name string) (*Collection, error) {
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

	size, distance := extractVectorDetails(info)

	collection := &Collection{
		Name:       name,
		Status:     info.Status.String(),
		Vectors:    derefUint64(info.IndexedVectorsCount),
		Points:     derefUint64(info.PointsCount),
		VectorSize: size,
		Distance:   distance,
	}

	return collection, nil
}

// ListCollections ──────────────────────────────────────────────────────────────
// ListCollections
// ──────────────────────────────────────────────────────────────
//
// ListCollections retrieves all existing collections from Qdrant and returns
// their names as a string slice. This can be extended to preload metadata
// using GetCollection for each name if needed.
//
// Example:
//
//	names, err := client.ListCollections(ctx)
//	if err != nil {
//	    log.Fatalf("failed to list collections: %v", err)
//	}
//	log.Printf("Found collections: %v", names)
func (c *QdrantClient) ListCollections(ctx context.Context) ([]string, error) {
	if c.api == nil {
		return nil, fmt.Errorf("[Qdrant] client not initialized")
	}

	names, err := c.api.ListCollections(ctx)
	if err != nil {
		return nil, fmt.Errorf("[Qdrant] failed to list collections: %w", err)
	}

	log.Printf("[Qdrant] Found %d collections", len(names))
	return names, nil
}

// Search ──────────────────────────────────────────────────────────────
// Search
// ──────────────────────────────────────────────────────────────
//
// Search performs a similarity search in the configured collection.
//
// Parameters:
//   - collectionName — the collection to search in
//   - vector — the query embedding to search against.
//   - topK   — maximum number of nearest results to return.
//
// Returns:
//
//	A slice of `SearchResultInterface` instances representing the
//	most similar stored embeddings.
// func (c *QdrantClient) Search(ctx context.Context, collectionName string, vector []float32, topK int) ([]SearchResultInterface, error) {
// 	if err := validateSearchInput(collectionName, vector, topK); err != nil {
// 		return nil, err
// 	}

// 	limit := uint64(topK)
// 	req := &qdrant.QueryPoints{
// 		CollectionName: collectionName,
// 		Query:          qdrant.NewQuery(vector...),
// 		Limit:          &limit,
// 		WithPayload:    qdrant.NewWithPayload(true),
// 	}

// 	resp, err := c.api.Query(ctx, req)
// 	if err != nil {
// 		return nil, fmt.Errorf("[Qdrant] search failed: %w", err)
// 	}

// 	results, err := c.parseSearchResults(resp)
// 	if err != nil {
// 		return nil, err
// 	}

// 	log.Printf("[Qdrant] Search returned %d results", len(results))
// 	return results, nil
// }

// SearchWithFilter ──────────────────────────────────────────────────────────────
// SearchWithFilter
// ──────────────────────────────────────────────────────────────
//
// SearchWithFilter performs a similarity search with required filters (AND logic).
// Returns error if filters is nil or empty - use Search() for unfiltered searches.
//
// Parameters:
//   - collectionName — the collection to search in
//   - vector — the query embedding to search against
//   - topK — maximum number of nearest results to return
//   - filters — required key-value filters (all must match)
//
// Returns:
//
//	A slice of SearchResultInterface instances matching the filter criteria.
// func (c *QdrantClient) SearchWithFilter(ctx context.Context, collectionName string, vector []float32, topK int, filters map[string]string) ([]SearchResultInterface, error) {
// 	if err := validateSearchInput(collectionName, vector, topK); err != nil {
// 		return nil, err
// 	}

// 	if len(filters) == 0 {
// 		return nil, fmt.Errorf("filters cannot be empty, use Search() for unfiltered searches")
// 	}

// 	limit := uint64(topK)
// 	req := &qdrant.QueryPoints{
// 		CollectionName: collectionName,
// 		Query:          qdrant.NewQuery(vector...),
// 		Limit:          &limit,
// 		WithPayload:    qdrant.NewWithPayload(true),
// 		Filter:         buildFilter(filters),
// 	}

// 	resp, err := c.api.Query(ctx, req)
// 	if err != nil {
// 		return nil, fmt.Errorf("[Qdrant] search failed: %w", err)
// 	}

// 	results, err := c.parseSearchResults(resp)
// 	if err != nil {
// 		return nil, err
// 	}

// 	log.Printf("[Qdrant] SearchWithFilter returned %d results", len(results))
// 	return results, nil
// }

// SearchBatch ──────────────────────────────────────────────────────────────
// SearchBatch
// ──────────────────────────────────────────────────────────────
//
// SearchBatch performs multiple searches and returns results for each request.
// Each request can optionally include filters.
//
// Parameters:
//   - requests — variadic SearchRequest structs, each containing:
//   - CollectionName: the collection to search in
//   - Vector: the query embedding
//   - TopK: maximum number of results per request
//   - Filters: optional key-value filters (AND logic)
//
// Returns:
//
//	A slice of result slices — one []SearchResultInterface per request.
//
// Example:
//
//	results, err := client.SearchBatch(ctx,
//	    SearchRequest{CollectionName: "docs", Vector: vec1, TopK: 10, Filters: map[string]string{"partition_id": "store-A"}},
//	    SearchRequest{CollectionName: "docs", Vector: vec2, TopK: 5},
//	)
//	// results[0] = results for first request
//	// results[1] = results for second request
func (c *QdrantClient) Search(ctx context.Context, requests ...SearchRequest) ([][]SearchResultInterface, error) {
	if len(requests) == 0 {
		return nil, fmt.Errorf("at least one search request is required")
	}

	results := make([][]SearchResultInterface, 0, len(requests))

	for i, searchReq := range requests {
		if err := validateSearchInput(searchReq.CollectionName, searchReq.Vector, searchReq.TopK); err != nil {
			return nil, fmt.Errorf("request [%d]: %w", i, err)
		}

		limit := uint64(searchReq.TopK)
		req := &qdrant.QueryPoints{
			CollectionName: searchReq.CollectionName,
			Query:          qdrant.NewQuery(searchReq.Vector...),
			Limit:          &limit,
			WithPayload:    qdrant.NewWithPayload(true),
			Filter:         buildFilter(searchReq.Filters),
		}

		resp, err := c.api.Query(ctx, req)
		if err != nil {
			return nil, fmt.Errorf("request [%d] search failed: %w", i, err)
		}

		res, err := c.parseSearchResults(resp)
		if err != nil {
			return nil, fmt.Errorf("request [%d] parse failed: %w", i, err)
		}

		results = append(results, res)
		log.Printf("[Qdrant] SearchBatch request [%d] returned %d results", i, len(res))
	}

	return results, nil
}

// parseSearchResults converts Qdrant response to SearchResultInterface slice
func (c *QdrantClient) parseSearchResults(resp []*qdrant.ScoredPoint) ([]SearchResultInterface, error) {
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
