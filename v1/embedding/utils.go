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
//
// This helper is reused by all embedding calls (semantic, instruct, vLLM),
// so embedding methods remain simple and consistent.
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

// normalizeRep validates and normalizes the embedding representation.
//
// The `representation` field can be either:
//   - "symmetric"
//   - "asymmetric"
//
// If the caller provides:
//   - nil
//   - an empty string
//   - or an invalid value
//
// the function returns the safe default: "symmetric".
//
// This keeps the SDK resilient and ensures we never send unsupported
// values to the embedding service.
func normalizeRep(rep *string) string {
	// If caller didn't set a representation, default to symmetric.
	if rep == nil || *rep == "" {
		return "symmetric"
	}

	// Only allow valid backend-supported strings.
	switch *rep {
	case "symmetric", "asymmetric":
		return *rep
	default:
		// If user gave an invalid value, fall back to symmetric.
		return "symmetric"
	}
}

// compressToSizeOrDefault is a passthrough helper for the optional
// `compress_to_size` field.
//
// The backend expects an integer representing the desired output
// embedding dimension. Unlike older versions of the API, there are
// no special variants (e.g., "variant128") and no enum mapping is
// required.
//
// The SDK therefore simply returns the value supplied by the caller.
// If `v` is nil, the field is omitted from the JSON payload.
func compressToSizeOrDefault(v *int) *int {
	return v
}
