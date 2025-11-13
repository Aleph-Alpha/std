package embedding

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

func (p *InferenceProvider) postJSON(ctx context.Context, url string, body any, out any) error {
	data, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("encode request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if p.serviceToken != "" {
		req.Header.Set("Authorization", "Bearer "+p.serviceToken)
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("http error: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return fmt.Errorf("http %d for %s", resp.StatusCode, url)
	}

	if out != nil {
		return json.NewDecoder(resp.Body).Decode(out)
	}

	return nil
}

func normalizeRep(rep *string) string {
	if rep == nil || *rep == "" {
		return "symmetric"
	}
	switch *rep {
	case "symmetric", "query", "document":
		return *rep
	default:
		return "symmetric"
	}
}

func compressToSizeOrDefault(v *int, def int) any {
	if v == nil {
		return "variant128"
	}
	if *v == 128 {
		return "variant128"
	}
	return "variant128"
}
