package minio

import (
	"time"

	"github.com/Aleph-Alpha/std/v1/observability"
)

// observeOperation notifies the observer about an operation if one is configured.
// This is used internally to track storage operations for metrics and tracing.
//
// Notes:
//   - resource: bucket name
//   - subResource: object key
func (m *MinioClient) observeOperation(operation, resource, subResource string, duration time.Duration, err error, size int64, metadata map[string]interface{}) {
	if m == nil || m.observer == nil {
		return
	}

	// Use bucket as resource, object key as subresource
	if resource == "" {
		resource = m.cfg.Connection.BucketName
	}

	m.observer.ObserveOperation(observability.OperationContext{
		Component:   "minio",
		Operation:   operation,
		Resource:    resource,
		SubResource: subResource,
		Duration:    duration,
		Error:       err,
		Size:        size,
		Metadata:    metadata,
	})
}
