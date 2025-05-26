package minio

import (
	"context"
	"crypto/rand"
	"fmt"
	"net/url"
	"time"
)

// urlGenerator replaces the host of a presigned URL with a custom base URL.
// This internal helper function allows modifying generated presigned URLs
// to use a different endpoint, which is useful for proxied setups.
//
// Parameters:
//   - presignedUrl: The original URL from MinIO
//   - baseUrl: The base URL to use instead
//
// Returns:
//   - string: The modified URL
//   - error: Any error that occurred during URL processing
func urlGenerator(presignedUrl *url.URL, baseUrl string) (string, error) {
	base, err := url.Parse(baseUrl)
	if err != nil {
		return "", fmt.Errorf("invalid BaseURL format: %v", err)
	}

	finalURL, err := url.Parse(presignedUrl.String())
	if err != nil {
		return "", fmt.Errorf("failed to parse presigned URL: %v", err)
	}
	finalURL.Scheme = base.Scheme
	finalURL.Host = base.Host
	if base.Path != "" && base.Path != "/" {
		finalURL.Path = base.ResolveReference(&url.URL{Path: finalURL.Path}).Path
	}
	return finalURL.String(), nil
}

// extractTraceMetadataFromContext extracts tracing information from context
// or generates new trace IDs if they don't exist.
//
// Parameters:
//   - ctx: Context potentially containing trace information
//
// Returns a map of trace metadata suitable for MinIO UserMetadata
func extractTraceMetadataFromContext(ctx context.Context) map[string]string {
	metadata := make(map[string]string)

	// Try to extract trace ID from context
	traceID, ok := ctx.Value("trace_id").(string)
	if !ok || traceID == "" {
		traceID = generateTraceID()
	}
	metadata["Trace-Id"] = traceID

	// Try to extract span ID from context
	spanID, ok := ctx.Value("span_id").(string)
	if !ok || spanID == "" {
		spanID = generateSpanID()
	}
	metadata["Span-Id"] = spanID

	// If there's a parent span in the context, include it
	if parentSpanID, ok := ctx.Value("parent_span_id").(string); ok && parentSpanID != "" {
		metadata["Parent-Span-Id"] = parentSpanID
	}

	// Add timestamp
	metadata["Timestamp"] = time.Now().UTC().Format(time.RFC3339)

	// Add any other useful metadata
	if requestID, ok := ctx.Value("request_id").(string); ok && requestID != "" {
		metadata["Request-Id"] = requestID
	}

	// Could also add a service name, operation name, etc.
	metadata["Service"] = "your-service-name"
	metadata["Operation"] = "object-upload"

	return metadata
}

// generateTraceID creates a unique trace identifier for distributed tracing.
// It generates a 16-byte random identifier using crypto/rand, which provides
// cryptographically secure random numbers suitable for trace IDs.
//
// The function follows these steps:
//  1. Attempts to generate 16 random bytes
//  2. Verifies that all requested bytes were successfully read
//  3. Falls back to a timestamp-based ID if random generation fails
//
// Returns:
//   - A 32-character lowercase hexadecimal string representing the trace ID
//
// The fallback mechanism ensures that a valid trace ID is always returned,
// even in the rare case where random number generation fails, maintaining
// the integrity of the tracing system.
//
// Example trace ID: "a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4"
func generateTraceID() string {
	b := make([]byte, 16)
	n, err := rand.Read(b)
	if err != nil || n != len(b) {
		// Fallback to a timestamp-based ID if a random generation fails
		timestamp := time.Now().UnixNano()
		return fmt.Sprintf("%016x", timestamp)
	}
	return fmt.Sprintf("%x", b)
}

// generateSpanID creates a unique span identifier for distributed tracing.
// It generates an 8-byte random identifier using crypto/rand, which provides
// cryptographically secure random numbers suitable for span IDs.
//
// The function follows these steps:
//  1. Attempts to generate 8 random bytes
//  2. Verifies that all requested bytes were successfully read
//  3. Falls back to a timestamp-based ID if random generation fails
//
// Returns:
//   - A 16-character lowercase hexadecimal string representing the span ID
//
// The fallback mechanism uses a bitwise AND operation with the maximum int64 value
// to ensure the timestamp-based ID stays within range, preventing overflow errors
// while still maintaining uniqueness.
//
// Example span ID: "a1b2c3d4e5f6a1b2"
//
// Each span ID should be unique within a trace to properly identify different
// operations within the distributed system.
func generateSpanID() string {
	b := make([]byte, 8)
	n, err := rand.Read(b)
	if err != nil || n != len(b) {
		// Fallback to a timestamp-based ID if a random generation fails
		timestamp := time.Now().UnixNano()
		return fmt.Sprintf("%016x", timestamp&0x7FFFFFFFFFFFFFFF)
	}
	return fmt.Sprintf("%x", b)
}
