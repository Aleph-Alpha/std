package observability

import "time"

// Observer is a unified interface for observability across all std packages.
// It allows external code to observe operations happening in infrastructure packages
// (postgres, mariadb, minio, kafka, rabbitmq, redis, etc.) without coupling them
// to specific observability implementations (metrics, tracing, logging).
//
// This interface is optional - std packages work perfectly fine without an observer.
type Observer interface {
	// ObserveOperation is called when an infrastructure operation completes.
	// It provides all context about the operation in a structured format.
	ObserveOperation(ctx OperationContext)
}

// OperationContext contains all information about an infrastructure operation.
// This struct is designed to be generic enough to work across all std packages
// while providing enough detail for comprehensive observability.
type OperationContext struct {
	// Component identifies which std package performed the operation.
	// Examples: "postgres", "mariadb", "minio", "kafka", "rabbitmq", "redis", "schema_registry"
	Component string

	// Operation describes what operation was performed.
	// Examples:
	//   Database: "insert", "select", "update", "delete", "transaction"
	//   Storage:  "put", "get", "delete", "list", "stat"
	//   Queue:    "publish", "consume", "ack", "nack"
	//   Kafka:    "produce", "consume", "commit"
	Operation string

	// Resource identifies the primary resource being operated on.
	// Examples:
	//   Database: table name ("users", "files", "datasets")
	//   Storage:  bucket name ("uploads", "datasets")
	//   Queue:    exchange name ("events", "tasks")
	//   Kafka:    topic name ("user-events", "data-pipeline")
	Resource string

	// SubResource provides additional resource context (optional).
	// Examples:
	//   Storage:  object key within bucket ("files/123/data.json")
	//   Queue:    routing key within exchange ("user.created")
	//   Kafka:    partition number ("3")
	SubResource string

	// Duration is how long the operation took from start to completion.
	Duration time.Duration

	// Error is the error returned by the operation, if any.
	// nil indicates successful operation.
	Error error

	// Size represents the size of data involved in the operation (optional).
	// Examples:
	//   Database: number of rows affected
	//   Storage:  bytes transferred
	//   Queue:    message size in bytes
	//   Kafka:    message size in bytes
	Size int64

	// Metadata provides additional operation-specific information (optional).
	// This map can contain any extra context that doesn't fit in the standard fields.
	// Examples:
	//   Database: {"query_type": "join", "index_used": "idx_user_email"}
	//   Storage:  {"content_type": "application/json", "storage_class": "STANDARD"}
	//   Queue:    {"delivery_mode": "persistent", "priority": "5"}
	//   Kafka:    {"compression": "snappy", "offset": "12345"}
	Metadata map[string]interface{}
}
