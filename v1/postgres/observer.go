package postgres

import (
	"time"

	"github.com/Aleph-Alpha/std/v1/observability"
)

// observeOperation notifies the observer about an operation if one is configured.
// This is used internally to track database operations for metrics and tracing.
//
// Notes:
//   - resource: ideally the table name; if empty it falls back to database name
//   - subResource: optional additional resource context (e.g. schema, migration id)
func (p *Postgres) observeOperation(operation, resource, subResource string, duration time.Duration, err error, rowsAffected int64, metadata map[string]interface{}) {
	if p == nil || p.observer == nil {
		return
	}

	// Prefer table name as the resource if available; otherwise fall back to DB name.
	if resource == "" {
		resource = p.cfg.Connection.DbName
	}

	p.observer.ObserveOperation(observability.OperationContext{
		Component:   "postgres",
		Operation:   operation,
		Resource:    resource,
		SubResource: subResource,
		Duration:    duration,
		Error:       err,
		Size:        rowsAffected,
		Metadata:    metadata,
	})
}
