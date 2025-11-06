package qdrant

import (
	"context"
	"fmt"
	"log"
	"time"

	qdrant "github.com/qdrant/go-client/qdrant"
)

// Client wraps the official Qdrant Go client.
//
// It manages connection lifecycle, configuration, and provides
// helper methods for collection and vector operations.
type Client struct {
	api     *qdrant.Client
	cfg     *Config
	started bool
}

// NewQdrantClient constructs and initializes a new Qdrant client.
//
// It’s registered as an Fx provider and automatically injected wherever
// *qdrant.Client is a dependency.
func NewQdrantClient(p QdrantParams) (*Client, error) {
	log.Printf("[Qdrant] Connecting to endpoint: %s", p.Config.Endpoint)

	client, err := qdrant.NewClient(&qdrant.Config{
		Host:   p.Config.Endpoint,
		APIKey: p.Config.ApiKey,
	})
	if err != nil {
		return nil, fmt.Errorf("[Qdrant] failed to initialize client: %w", err)
	}

	c := &Client{
		api:     client,
		cfg:     p.Config,
		started: true,
	}

	if err := c.healthCheck(); err != nil {
		return nil, fmt.Errorf("[Qdrant] health check failed: %w", err)
	}

	log.Println("[Qdrant] Client connected successfully")
	return c, nil
}

// healthCheck calls Qdrant’s /healthz endpoint to verify connectivity.
func (c *Client) healthCheck() error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	resp, err := c.api.Health(ctx)
	if err != nil {
		return err
	}
	if !resp.Result.IsOk {
		return fmt.Errorf("qdrant health check failed: %v", resp)
	}
	log.Printf("[Qdrant] Health check passed (%s)", c.cfg.Endpoint)
	return nil
}

// Close gracefully shuts down the Qdrant client (no-op for SDK clients).
func (c *Client) Close() {
	if !c.started {
		return
	}
	log.Println("[Qdrant] Closing client connection")
	c.started = false
}

// EmbeddingsStore implements the EmbeddingRepository interface.
//
// It provides high-level operations (ensure, insert, search, delete)
// backed by the Qdrant API.
type EmbeddingsStore struct {
	client *Client
}

// NewEmbeddingsStore initializes and returns a new embeddings store.
//
// It verifies that the target collection exists, creating it if necessary.
func NewEmbeddingsStore(client *Client) (*EmbeddingsStore, error) {
	store := &EmbeddingsStore{client: client}

	if err := store.EnsureCollection(context.Background(), client.cfg.Collection); err != nil {
		return nil, fmt.Errorf("[Qdrant] failed to ensure collection: %w", err)
	}

	log.Printf("[Qdrant] EmbeddingsStore ready (collection=%s)", client.cfg.Collection)
	return store, nil
}

// EnsureCollection checks if a collection exists, and creates it if missing.
func (s *EmbeddingsStore) EnsureCollection(ctx context.Context, name string) error {
	_, err := s.client.api.GetCollection(ctx, &qdrant.GetCollectionRequest{
		CollectionName: name,
	})
	if err == nil {
		return nil // already exists
	}

	log.Printf("[Qdrant] Creating new collection: %s", name)
	_, err = s.client.api.CreateCollection(ctx, &qdrant.CreateCollection{
		CollectionName: name,
		VectorSize:     1536,
		//Distance:       qdrant.Distance_Cosine,
	})
	return err
}

// Insert adds or updates embeddings in Qdrant.
func (s *EmbeddingsStore) Insert(ctx context.Context, embeddings []Embedding) error {
	if len(embeddings) == 0 {
		return nil
	}

	points := make([]*qdrant.PointStruct, 0, len(embeddings))
	for _, e := range embeddings {
		points = append(points, &qdrant.PointStruct{
			Id:      qdrant.NewID(e.ID),
			Vectors: qdrant.NewVectors(e.Vector),
			Payload: e.Meta,
		})
	}

	_, err := s.client.api.Upsert(ctx, &qdrant.UpsertPoints{
		CollectionName: s.client.cfg.Collection,
		Points:         points,
		Wait:           true,
	})
	if err != nil {
		return fmt.Errorf("[Qdrant] upsert failed: %w", err)
	}

	log.Printf("[Qdrant] Inserted %d embeddings into %s", len(embeddings), s.client.cfg.Collection)
	return nil
}

// Search performs a vector similarity search.
func (s *EmbeddingsStore) Search(ctx context.Context, vector []float32, topK int) ([]SearchResult, error) {
	resp, err := s.client.api.Query(ctx, &qdrant.QueryPoints{
		CollectionName: s.client.cfg.Collection,
		Query:          &qdrant.Query{Nearest: &qdrant.Nearest{Vector: vector}},
		Limit:          uint64(topK),
		WithPayload:    &qdrant.WithPayloadSelector{Enable: true},
	})
	if err != nil {
		return nil, fmt.Errorf("[Qdrant] search failed: %w", err)
	}

	results := make([]SearchResult, 0, len(resp.Result))
	for _, batch := range resp.Result {
		for _, r := range batch {
			results = append(results, SearchResult{
				ID:    r.Id.GetStringValue(),
				Score: r.Score,
				Meta:  r.Payload,
			})
		}
	}

	log.Printf("[Qdrant] Search returned %d results", len(results))
	return results, nil
}

// Delete removes embeddings from a collection by ID.
func (s *EmbeddingsStore) Delete(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}

	qdrantIDs := make([]*qdrant.PointId, 0, len(ids))
	for _, id := range ids {
		qdrantIDs = append(qdrantIDs, qdrant.NewID(id))
	}

	_, err := s.client.api.Delete(ctx, &qdrant.DeletePoints{
		CollectionName: s.client.cfg.Collection,
		Points:         &qdrant.PointsSelector{PointsSelectorOneOf: &qdrant.PointsSelector_Points{Points: &qdrant.PointsIdsList{Ids: qdrantIDs}}},
		Wait:           true,
	})
	if err != nil {
		return fmt.Errorf("[Qdrant] delete failed: %w", err)
	}

	log.Printf("[Qdrant] Deleted %d embeddings from %s", len(ids), s.client.cfg.Collection)
	return nil
}
