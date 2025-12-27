package kafka

import (
	"time"

	"github.com/Aleph-Alpha/std/v1/observability"
)

// observeOperation safely calls the observer if it's not nil.
// This helper reduces boilerplate in operation methods.
func (k *KafkaClient) observeOperation(operation, resource, subResource string, duration time.Duration, err error, size int64) {
	if k.observer != nil {
		k.observer.ObserveOperation(observability.OperationContext{
			Component:   "kafka",
			Operation:   operation,
			Resource:    resource,
			SubResource: subResource,
			Duration:    duration,
			Error:       err,
			Size:        size,
		})
	}
}
