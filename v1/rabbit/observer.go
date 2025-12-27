package rabbit

import (
	"time"

	"github.com/Aleph-Alpha/std/v1/observability"
)

// observeOperation notifies the observer about an operation if one is configured.
// This is used internally to track publish and consume operations for metrics and tracing.
func (rb *RabbitClient) observeOperation(operation, resource, subResource string, duration time.Duration, err error, size int64) {
	if rb.observer != nil {
		rb.observer.ObserveOperation(observability.OperationContext{
			Component:   "rabbit",
			Operation:   operation,
			Resource:    resource,
			SubResource: subResource,
			Duration:    duration,
			Error:       err,
			Size:        size,
			Metadata:    nil,
		})
	}
}
