package embedding

import "context"

// Strategy enum
type EmbeddingStrategyType string

const (
	StrategySemantic EmbeddingStrategyType = "semantic"
	StrategyInstruct EmbeddingStrategyType = "instruct"
	StrategyVLLM     EmbeddingStrategyType = "vllm" // OpenAI-compatible
)

// High-level strategy config used by callers.
type EmbeddingStrategy struct {
	Type           EmbeddingStrategyType
	Model          string
	Instruction    *string // only for instruct
	Representation *string // "symmetric" | "query" | "document" (semantic)
	CompressToSize *int    // e.g., 128 (semantic)
	Normalize      bool
}

// Provider contract
type Provider interface {
	// Single-shot embeddings (len(texts) >= 1)
	CreateEmbeddings(ctx context.Context, texts []string, strategy EmbeddingStrategy) ([][]float64, error)

	// Batch embeddings:
	// - For semantic: texts is a slice of strings per batch item -> [][]string not needed; we just send []string prompts in one request.
	// - For openai: same as above, single call with input: []string
	// - For instruct: no batch endpoint — provider will loop per text.
	CreateBatchEmbeddings(ctx context.Context, texts []string, strategy EmbeddingStrategy) ([][]float64, error)
}
