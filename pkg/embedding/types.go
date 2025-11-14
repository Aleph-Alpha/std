package embedding

import "context"

// EmbeddingStrategyType enumerates the supported embedding strategies.
type EmbeddingStrategyType string

const (
	StrategySemantic EmbeddingStrategyType = "semantic"
	StrategyInstruct EmbeddingStrategyType = "instruct"
	StrategyVLLM     EmbeddingStrategyType = "vllm" // OpenAI-compatible
)

type HybridIndex string

const (
	HybridIndexBM25 HybridIndex = "bm25"
)

// instruction: { "query": "...", "document": "..." }
type InstructInstruction struct {
	Query    string `json:"query"`
	Document string `json:"document"`
}

// EmbeddingStrategy describes how embeddings should be computed.
// Different strategies use different subsets of fields.
type EmbeddingStrategy struct {
	Type  EmbeddingStrategyType `json:"type"`
	Model string                `json:"model"`

	//Instruct only
	Instruction *InstructInstruction `json:"instruction,omitempty"`

	// Semantic only
	Representation *string `json:"representation,omitempty"` // "symmetric" | "asymmetric"

	HybridIndex *HybridIndex `json:"hybrid_index,omitempty"`

	// Common flags
	Normalize      bool `json:"normalize,omitempty"`
	CompressToSize *int `json:"dimension,omitempty"`
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
