package vectordb

// SearchRequest represents a single similarity search query.
// Use with VectorDBService.Search() for single or batch queries.
type SearchRequest struct {
	// CollectionName is the target collection to search in
	CollectionName string `json:"collectionName"`

	// Vector is the query embedding to find similar vectors for
	Vector []float32 `json:"vector"`

	// TopK is the maximum number of results to return
	TopK int `json:"maxResults"`

	// Filters is optional metadata filtering (AND/OR/NOT logic)
	Filters *FilterSet `json:"filters,omitempty"`
}

// SearchResult represents a single search result with its similarity score.
// This is database-agnostic—payload is converted to map[string]any.
type SearchResult struct {
	// ID is the unique identifier of the matched point
	ID string `json:"id"`

	// Score is the similarity score (higher = more similar for cosine)
	Score float32 `json:"score"`

	// Payload contains the metadata stored with the vector
	Payload map[string]any `json:"payload"`

	// Vector is the stored embedding (only populated if requested)
	Vector []float32 `json:"vector,omitempty"`

	// CollectionName identifies which collection this result came from
	CollectionName string `json:"collectionName,omitempty"`
}

// EmbeddingInput is the input for inserting vectors into a collection.
type EmbeddingInput struct {
	// ID is the unique identifier for this embedding
	ID string `json:"id"`

	// Vector is the dense embedding representation
	Vector []float32 `json:"vector"`

	// Payload is optional metadata to store with the vector
	Payload map[string]any `json:"payload,omitempty"`
}

// Collection contains metadata about a vector collection.
type Collection struct {
	// Name is the unique identifier of the collection
	Name string `json:"name"`

	// Status indicates the operational state (e.g., "Green", "Yellow")
	Status string `json:"status"`

	// VectorSize is the dimension of vectors in this collection
	VectorSize int `json:"vectorSize"`

	// Distance is the similarity metric (e.g., "Cosine", "Dot", "Euclid")
	Distance string `json:"distance"`

	// VectorCount is the number of indexed vectors
	VectorCount uint64 `json:"vectorCount"`

	// PointCount is the number of stored points/documents
	PointCount uint64 `json:"pointCount"`
}
