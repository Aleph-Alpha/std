package schema_registry

import (
	"time"

	"github.com/Aleph-Alpha/std/v1/observability"
)

// observeOperation notifies the observer about an operation if one is configured.
// This is used internally to track schema registry operations for metrics and tracing.
//
// Notes:
//   - resource: subject name (for subject-specific operations) or "registry" (for ID lookups)
//   - subResource: schema ID or version information
func (c *Client) observeOperation(operation, resource, subResource string, duration time.Duration, err error, metadata map[string]interface{}) {
	if c == nil || c.observer == nil {
		return
	}

	c.observer.ObserveOperation(observability.OperationContext{
		Component:   "schema_registry",
		Operation:   operation,
		Resource:    resource,
		SubResource: subResource,
		Duration:    duration,
		Error:       err,
		Metadata:    metadata,
	})
}
