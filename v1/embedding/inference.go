package embedding

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type InferenceProvider struct {
	baseURL      string
	serviceToken string
	httpClient   *http.Client
}

func newInferenceProvider(cfg *Config) (*InferenceProvider, error) {
	if cfg.Endpoint == "" {
		return nil, fmt.Errorf("inference: missing EMBEDDING_ENDPOINT")
	}
	if cfg.ServiceToken == "" {
		return nil, fmt.Errorf("inference: missing EMBEDDING_SERVICE_TOKEN")
	}

	// Remove trailing slash if user added it.
	base := strings.TrimRight(cfg.Endpoint, "/")

	return &InferenceProvider{
		baseURL:      base,
		serviceToken: cfg.ServiceToken,
		httpClient:   &http.Client{Timeout: time.Duration(cfg.HTTPTimeoutS) * time.Second},
	}, nil
}

func (p *InferenceProvider) CreateEmbeddings(ctx context.Context, texts []string, s EmbeddingStrategy) ([][]float64, error) {
	if len(texts) == 0 {
		return nil, fmt.Errorf("inference: no texts provided")
	}

	switch s.Type {
	case StrategySemantic:
		return p.semanticEmbed(ctx, texts[0], s)

	case StrategyInstruct:
		return p.instructableEmbed(ctx, texts[0], s)

	case StrategyVLLM:
		return p.openAICompatible(ctx, texts, s)

	default:
		return nil, fmt.Errorf("inference: unsupported strategy %q", s.Type)
	}
}

func (p *InferenceProvider) CreateBatchEmbeddings(ctx context.Context, texts []string, s EmbeddingStrategy) ([][]float64, error) {
	if len(texts) == 0 {
		return nil, fmt.Errorf("inference: empty batch")
	}

	switch s.Type {
	case StrategySemantic:
		return p.batchSemanticEmbed(ctx, texts, s)

	case StrategyInstruct:
		// Instruct embeddings are expanded into multiple sequential calls.
		out := make([][]float64, 0, len(texts))
		for _, t := range texts {
			vecs, err := p.instructableEmbed(ctx, t, s)
			if err != nil {
				return nil, err
			}
			out = append(out, vecs[0])
		}
		return out, nil

	case StrategyVLLM:
		return p.openAICompatible(ctx, texts, s)

	default:
		return nil, fmt.Errorf("inference: unsupported strategy %q", s.Type)
	}
}

//
// --- Semantic Embedding ---
//

func (p *InferenceProvider) semanticEmbed(ctx context.Context, text string, s EmbeddingStrategy) ([][]float64, error) {
	reqBody := map[string]any{
		"model":            s.Model,
		"prompt":           text,
		"normalize":        s.Normalize,
		"representation":   normalizeRep(s.Representation),
		"compress_to_size": compressToSizeOrDefault(s.CompressToSize),
	}

	// hybrid_index: optional
	if s.HybridIndex != nil {
		reqBody["hybrid_index"] = *s.HybridIndex // e.g. "bm25"
	}

	url := fmt.Sprintf("%s/semantic_embed", p.baseURL)

	var parsed struct {
		Embedding []float64 `json:"embedding"`
	}

	if err := p.postJSON(ctx, url, reqBody, &parsed); err != nil {
		return nil, err
	}
	if len(parsed.Embedding) == 0 {
		return nil, fmt.Errorf("inference: semantic_embed empty embedding")
	}

	return [][]float64{parsed.Embedding}, nil
}

func (p *InferenceProvider) batchSemanticEmbed(ctx context.Context, texts []string, s EmbeddingStrategy) ([][]float64, error) {
	reqBody := map[string]any{
		"model":            s.Model,
		"prompts":          texts,
		"normalize":        s.Normalize,
		"representation":   normalizeRep(s.Representation),
		"compress_to_size": compressToSizeOrDefault(s.CompressToSize),
	}

	if s.HybridIndex != nil {
		reqBody["hybrid_index"] = *s.HybridIndex
	}

	url := fmt.Sprintf("%s/batch_semantic_embed", p.baseURL)

	var parsed struct {
		Embeddings [][]float64 `json:"embeddings"`
	}

	if err := p.postJSON(ctx, url, reqBody, &parsed); err != nil {
		return nil, err
	}

	if len(parsed.Embeddings) == 0 {
		return nil, fmt.Errorf("inference: batch_semantic_embed empty embeddings")
	}

	return parsed.Embeddings, nil
}

//
// --- Instruct Embedding ---
//

func (p *InferenceProvider) instructableEmbed(ctx context.Context, text string, s EmbeddingStrategy) ([][]float64, error) {
	if s.Instruction == nil {
		return nil, fmt.Errorf("inference: instruction required for instruct strategy")
	}

	reqBody := map[string]any{
		"model":       s.Model,
		"input":       text,
		"instruction": s.Instruction,
		"normalize":   s.Normalize,
	}

	url := fmt.Sprintf("%s/instructable_embed", p.baseURL)

	var parsed struct {
		Embedding []float64 `json:"embedding"`
	}

	if err := p.postJSON(ctx, url, reqBody, &parsed); err != nil {
		return nil, err
	}

	if len(parsed.Embedding) == 0 {
		return nil, fmt.Errorf("inference: instructable_embed empty embedding")
	}

	return [][]float64{parsed.Embedding}, nil
}

//
// --- OpenAI-Compatible / vLLM Embedding ---
//

func (p *InferenceProvider) openAICompatible(ctx context.Context, texts []string, s EmbeddingStrategy) ([][]float64, error) {
	reqBody := map[string]any{
		"model": s.Model,
		"input": texts,
	}

	url := fmt.Sprintf("%s/v1/embeddings", p.baseURL)

	var parsed struct {
		Data []struct {
			Embedding []float64 `json:"embedding"`
		} `json:"data"`
	}

	if err := p.postJSON(ctx, url, reqBody, &parsed); err != nil {
		return nil, err
	}

	if len(parsed.Data) == 0 {
		return nil, fmt.Errorf("inference: v1/embeddings empty data")
	}

	out := make([][]float64, len(parsed.Data))
	for i, d := range parsed.Data {
		out[i] = d.Embedding
	}

	return out, nil
}
