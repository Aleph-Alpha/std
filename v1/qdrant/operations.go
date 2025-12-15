package qdrant

import (
	"context"
	"fmt"
	"log"
	"slices"

	"github.com/Aleph-Alpha/std/v1/vectordb"
	qdrant "github.com/qdrant/go-client/qdrant"
	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/semaphore"
)

// Ensure Adapter implements Service at compile time
var _ vectordb.Service = (*Adapter)(nil)

// ══════════════════════════════════════════════════════════════════════════════
// Adapter - implements vectordb.Service interface
// ══════════════════════════════════════════════════════════════════════════════

// Adapter implements vectordb.Service for Qdrant.
// It wraps a Qdrant client and converts between generic vectordb types
// and Qdrant-specific protobuf types.
//
// This is the recommended way to use Qdrant - it provides a database-agnostic
// interface that allows switching between different vector databases.
type Adapter struct {
	client *qdrant.Client
}

// NewAdapter creates a new Qdrant adapter for the vectordb interface.
// Pass the underlying SDK client via QdrantClient.Client().
//
// Example:
//
//	qc, _ := qdrant.NewQdrantClient(params)
//	adapter := qdrant.NewAdapter(qc.Client())
//	var db vectordb.Service = adapter
func NewAdapter(client *qdrant.Client) *Adapter {
	return &Adapter{client: client}
}

// Search performs similarity search across one or more requests.
func (a *Adapter) Search(ctx context.Context, requests ...vectordb.SearchRequest) ([][]vectordb.SearchResult, error) {
	if len(requests) == 0 {
		return nil, fmt.Errorf("at least one search request is required")
	}

	log.Printf("[Qdrant] Starting search batch with %d requests", len(requests))

	// Validate all requests first
	for i, searchReq := range requests {
		if err := validateSearchInput(searchReq.CollectionName, searchReq.Vector, searchReq.TopK); err != nil {
			return nil, fmt.Errorf("request [%d]: %w", i, err)
		}
	}

	results := make([][]vectordb.SearchResult, len(requests))

	// Create errgroup with context
	g, ctx := errgroup.WithContext(ctx)

	// Create semaphore to limit concurrent searches
	sem := semaphore.NewWeighted(maxConcurrentSearches)

	for i, searchReq := range requests {
		i, searchReq := i, searchReq // Capture loop variables
		g.Go(func() error {
			// Acquire semaphore (blocks if at max concurrency)
			if err := sem.Acquire(ctx, 1); err != nil {
				return fmt.Errorf("request [%d]: failed to acquire semaphore: %w", i, err)
			}
			defer sem.Release(1)

			res, err := searchInternal(ctx, a.client, searchReq)
			if err != nil {
				return fmt.Errorf("request [%d]: search failed: %w", i, err)
			}
			results[i] = res
			log.Printf("[Qdrant] Search request [%d] returned %d results", i, len(res))
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return nil, fmt.Errorf("search batch failed: %w", err)
	}
	return results, nil
}

// Insert adds embeddings to a collection using batch processing.
func (a *Adapter) Insert(ctx context.Context, collectionName string, inputs []vectordb.EmbeddingInput) error {
	if len(inputs) == 0 {
		return nil
	}
	if collectionName == "" {
		return fmt.Errorf("collection name cannot be empty")
	}
	return insertInternal(ctx, a.client, collectionName, inputs)
}

// Delete removes points by their IDs from a collection.
func (a *Adapter) Delete(ctx context.Context, collectionName string, ids []string) error {
	return deleteInternal(ctx, a.client, collectionName, ids)
}

// EnsureCollection creates a collection if it doesn't exist.
func (a *Adapter) EnsureCollection(ctx context.Context, name string, vectorSize uint64) error {
	return ensureCollectionInternal(ctx, a.client, name, vectorSize)
}

// GetCollection retrieves metadata about a collection.
func (a *Adapter) GetCollection(ctx context.Context, name string) (*vectordb.Collection, error) {
	if name == "" {
		return nil, fmt.Errorf("collection name cannot be empty")
	}

	info, err := a.client.GetCollectionInfo(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("failed to get collection '%s': %w", name, err)
	}

	size, distance := extractVectorDetails(info)

	collection := &vectordb.Collection{
		Name:        name,
		Status:      info.Status.String(),
		VectorSize:  size,
		Distance:    distance,
		VectorCount: derefUint64(info.IndexedVectorsCount),
		PointCount:  derefUint64(info.PointsCount),
	}

	return collection, nil
}

// ListCollections returns names of all collections.
func (a *Adapter) ListCollections(ctx context.Context) ([]string, error) {
	return listCollectionsInternal(ctx, a.client)
}

// ══════════════════════════════════════════════════════════════════════════════
// Internal Functions
// ══════════════════════════════════════════════════════════════════════════════

func searchInternal(ctx context.Context, client *qdrant.Client, searchReq vectordb.SearchRequest) ([]vectordb.SearchResult, error) {
	limit := uint64(searchReq.TopK)
	queryReq := &qdrant.QueryPoints{
		CollectionName: searchReq.CollectionName,
		Query:          qdrant.NewQuery(searchReq.Vector...),
		Limit:          &limit,
		WithPayload:    qdrant.NewWithPayload(true),
		Filter:         convertVectorDBFilterSets(searchReq.Filters),
	}

	resp, err := client.Query(ctx, queryReq)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}
	return parseVectorDBSearchResults(resp)
}

func insertInternal(ctx context.Context, client *qdrant.Client, collectionName string, inputs []vectordb.EmbeddingInput) error {
	for start := 0; start < len(inputs); start += defaultBatchSize {
		end := start + defaultBatchSize
		if end > len(inputs) {
			end = len(inputs)
		}
		batch := inputs[start:end]

		points := make([]*qdrant.PointStruct, 0, len(batch))
		for _, e := range batch {
			points = append(points, &qdrant.PointStruct{
				Id:      qdrant.NewID(e.ID),
				Vectors: qdrant.NewVectors(e.Vector...),
				Payload: qdrant.NewValueMap(e.Payload),
			})
		}

		wait := true
		req := &qdrant.UpsertPoints{
			CollectionName: collectionName,
			Points:         points,
			Wait:           &wait,
		}

		if _, err := client.Upsert(ctx, req); err != nil {
			return fmt.Errorf("[Qdrant] batch upsert failed at [%d:%d]: %w", start, end, err)
		}
		log.Printf("[Qdrant] Inserted batch [%d:%d] (collection=%s)", start, end, collectionName)
	}
	return nil
}

func deleteInternal(ctx context.Context, client *qdrant.Client, collectionName string, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	if collectionName == "" {
		return fmt.Errorf("collection name cannot be empty")
	}

	qdrantIDs := make([]*qdrant.PointId, 0, len(ids))
	for _, id := range ids {
		qdrantIDs = append(qdrantIDs, qdrant.NewID(id))
	}

	wait := true
	req := &qdrant.DeletePoints{
		CollectionName: collectionName,
		Points: &qdrant.PointsSelector{
			PointsSelectorOneOf: &qdrant.PointsSelector_Points{
				Points: &qdrant.PointsIdsList{Ids: qdrantIDs},
			},
		},
		Wait: &wait,
	}

	resp, err := client.Delete(ctx, req)
	if err != nil {
		return fmt.Errorf("[Qdrant] delete failed: %w", err)
	}

	log.Printf("[Qdrant] Delete completed (status=%s, collection=%s)", resp.Status.String(), collectionName)
	return nil
}

func ensureCollectionInternal(ctx context.Context, client *qdrant.Client, name string, vectorSize uint64) error {
	if name == "" {
		return fmt.Errorf("collection name cannot be empty")
	}

	collections, err := client.ListCollections(ctx)
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
			Size:     vectorSize,
			Distance: qdrant.Distance_Cosine,
		}),
	}

	if err := client.CreateCollection(ctx, req); err != nil {
		return fmt.Errorf("[Qdrant] failed to create collection '%s': %w", name, err)
	}

	log.Printf("[Qdrant] Created collection '%s' successfully", name)
	return nil
}

func listCollectionsInternal(ctx context.Context, client *qdrant.Client) ([]string, error) {
	names, err := client.ListCollections(ctx)
	if err != nil {
		return nil, fmt.Errorf("[Qdrant] failed to list collections: %w", err)
	}
	log.Printf("[Qdrant] Found %d collections", len(names))
	return names, nil
}
