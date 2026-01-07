package minio

import (
	"time"

	"github.com/Aleph-Alpha/std/v1/observability"
)

// observeOperation notifies the observer about an operation if one is configured.
// This is used internally to track storage operations for metrics and tracing.
//
// Parameters:
//   - operation: The type of operation being performed (e.g., "put", "get", "delete")
//   - bucket: The bucket name (used as resource)
//   - objectKey: The object key (used as subResource)
//   - duration: How long the operation took
//   - err: Any error that occurred during the operation
//   - size: The size in bytes of data transferred
//   - metadata: Additional metadata about the operation
func (m *MinioClient) observeOperation(operation, bucket, objectKey string, duration time.Duration, err error, size int64, metadata map[string]interface{}) {
	if m == nil || m.observer == nil {
		return
	}

	m.observer.ObserveOperation(observability.OperationContext{
		Component:   "minio",
		Operation:   operation,
		Resource:    bucket,
		SubResource: objectKey,
		Duration:    duration,
		Error:       err,
		Size:        size,
		Metadata:    metadata,
	})
}
