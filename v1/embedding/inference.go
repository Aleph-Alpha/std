package embedding

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type InferenceProvider struct {
	baseURL    string
	httpClient *http.Client
}

func newInferenceProvider(cfg *Config) (*InferenceProvider, error) {
	if cfg.Endpoint == "" {
		return nil, fmt.Errorf("inference: missing EMBEDDING_ENDPOINT")
	}

	// Remove trailing slash if user added it.
	base := strings.TrimRight(cfg.Endpoint, "/")

	return &InferenceProvider{
		baseURL:    base,
		httpClient: &http.Client{Timeout: time.Duration(cfg.HTTPTimeoutS) * time.Second},
	}, nil
}

// Create generates embeddings for the given texts using the specified model.
// It uses the OpenAI-compatible /v1/embeddings endpoint.
func (p *InferenceProvider) Create(ctx context.Context, token, model string, texts ...string) ([][]float64, error) {
	if len(texts) == 0 {
		return nil, fmt.Errorf("inference: no texts provided")
	}
	if model == "" {
		return nil, fmt.Errorf("inference: model is required")
	}
	if token == "" {
		return nil, fmt.Errorf("inference: token is required")
	}

	reqBody := map[string]any{
		"model": model,
		"input": texts,
	}

	url := fmt.Sprintf("%s/embeddings", p.baseURL)

	var parsed struct {
		Data []struct {
			Embedding []float64 `json:"embedding"`
		} `json:"data"`
	}

	if err := p.postJSON(ctx, token, url, reqBody, &parsed); err != nil {
		return nil, err
	}

	if len(parsed.Data) == 0 {
		return nil, fmt.Errorf("inference: embeddings empty data")
	}

	out := make([][]float64, len(parsed.Data))
	for i, d := range parsed.Data {
		out[i] = d.Embedding
	}

	return out, nil
}
