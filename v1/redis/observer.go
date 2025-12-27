package redis

import (
	"time"

	"github.com/Aleph-Alpha/std/v1/observability"
)

// observeOperation notifies the observer about an operation if one is configured.
// This is used internally to track Redis operations for metrics and tracing.
//
// Notes:
//   - resource: the Redis key(s) being operated on
//   - subResource: additional context like field names or channel names
func (r *RedisClient) observeOperation(operation, resource, subResource string, duration time.Duration, err error, size int64, metadata map[string]interface{}) {
	if r == nil || r.observer == nil {
		return
	}

	r.observer.ObserveOperation(observability.OperationContext{
		Component:   "redis",
		Operation:   operation,
		Resource:    resource,
		SubResource: subResource,
		Duration:    duration,
		Error:       err,
		Size:        size,
		Metadata:    metadata,
	})
}
