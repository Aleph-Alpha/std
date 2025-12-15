package qdrant

import (
	"fmt"

	qdrant "github.com/qdrant/go-client/qdrant"
)

// validateSearchInput validates common search parameters
func validateSearchInput(collectionName string, vector []float32, topK int) error {
	if collectionName == "" {
		return fmt.Errorf("collection name cannot be empty")
	}
	if len(vector) == 0 {
		return fmt.Errorf("vector cannot be empty")
	}
	if topK <= 0 {
		return fmt.Errorf("topK must be greater than 0")
	}
	return nil
}

// extractVectorDetails ──────────────────────────────────────────────────────────────
// extractVectorDetails
// ──────────────────────────────────────────────────────────────
//
// extractVectorDetails safely extracts the vector size (embedding dimension)
// and distance metric (e.g., "Cosine", "Dot", "Euclid") from a Qdrant
// `CollectionInfo` object.
//
// Qdrant represents vector configuration data using a deeply nested protobuf
// structure with "oneof" wrappers. This helper navigates that hierarchy,
// performs type assertions, and guards against nil pointer dereferences.
//
// It returns:
//   - int    — vector dimension (size of embedding vectors)
//   - string — distance metric used for similarity search
//
// If any nested field is missing or of an unexpected type, the function
// gracefully returns default values (0, "").
//
// Example:
//
//	size, distance := extractVectorDetails(info)
//	log.Printf("Vector size=%d, distance=%s", size, distance)
func extractVectorDetails(info *qdrant.CollectionInfo) (int, string) {
	if info == nil ||
		info.Config == nil ||
		info.Config.Params == nil ||
		info.Config.Params.VectorsConfig == nil ||
		info.Config.Params.VectorsConfig.Config == nil {
		return 0, ""
	}

	if cfg, ok := info.Config.Params.VectorsConfig.Config.(*qdrant.VectorsConfig_Params); ok {
		return int(cfg.Params.Size), cfg.Params.Distance.String()
	}

	return 0, ""
}

// derefUint64 safely dereferences a *uint64 pointer.
// If the pointer is nil, it returns 0 instead of panicking.
func derefUint64(v *uint64) uint64 {
	if v != nil {
		return *v
	}
	return 0
}
