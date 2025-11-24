package embedding

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// postJSON sends an HTTP POST request to the inference API.
// It marshals the given body as JSON, attaches required headers,
// handles HTTP error codes, and optionally decodes the response JSON into `out`.
func (p *InferenceProvider) postJSON(ctx context.Context, url string, body any, out any) error {

	// Convert request payload into JSON bytes.
	data, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("encode request: %w", err)
	}

	// Construct the HTTP POST request with context (supports cancellation & timeout).
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}

	// Add JSON header and service-token authentication.
	req.Header.Set("Content-Type", "application/json")
	if p.serviceToken != "" {
		req.Header.Set("Authorization", "Bearer "+p.serviceToken)
	}

	// Execute the HTTP request. Client timeout is configured in NewInferenceProvider.
	resp, err := p.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("http error: %w", err)
	}
	defer resp.Body.Close()

	// Treat any non-2xx status code as an error.
	if resp.StatusCode >= 300 {
		return fmt.Errorf("http %d for %s", resp.StatusCode, url)
	}

	// If caller expects a response body, decode it into `out`.
	if out != nil {
		return json.NewDecoder(resp.Body).Decode(out)
	}

	// No response body requested → success.
	return nil
}
