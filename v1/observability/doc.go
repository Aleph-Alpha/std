// Package observability provides a unified interface for observing operations
// across all std infrastructure packages.
//
// # Overview
//
// The observability package defines a single Observer interface that all std packages
// can use to emit operation events. This allows applications to implement metrics,
// tracing, and logging in a consistent way across all infrastructure layers.
//
// # Design Philosophy
//
// 1. **Optional**: std packages work perfectly without an observer
// 2. **Unified**: Same interface for all infrastructure (DB, storage, queues, etc.)
// 3. **Flexible**: Observer can implement metrics, tracing, logging, or all three
// 4. **Generic**: OperationContext works across different infrastructure types
// 5. **Non-intrusive**: Minimal code in std packages
//
// # Usage in std Packages
//
// Infrastructure packages accept an optional Observer in their config:
//
//	// std/v1/postgres/config.go
//	import "github.com/Aleph-Alpha/std/v1/observability"
//
//	type Config struct {
//	    Host     string
//	    Port     int
//	    Database string
//
//	    // Optional observer for operation tracking
//	    Observer observability.Observer
//	}
//
// Then call the observer when operations complete:
//
//	func (p *postgres) Create(ctx context.Context, value interface{}) error {
//	    start := time.Now()
//	    err := p.db.WithContext(ctx).Create(value).Error
//
//	    // Notify observer if present
//	    if p.config.Observer != nil {
//	        p.config.Observer.ObserveOperation(observability.OperationContext{
//	            Component: "postgres",
//	            Operation: "insert",
//	            Resource:  extractTableName(value),
//	            Duration:  time.Since(start),
//	            Error:     err,
//	        })
//	    }
//
//	    return err
//	}
//
// # Usage in Applications
//
// Applications implement the Observer interface to handle operations:
//
//	type MetricsObserver struct {
//	    metrics *prometheus.Metrics
//	}
//
//	func (o *MetricsObserver) ObserveOperation(ctx observability.OperationContext) {
//	    // Record metrics based on component and operation
//	    switch ctx.Component {
//	    case "postgres", "mariadb":
//	        o.metrics.RecordDatabaseQuery(ctx.Operation, ctx.Resource, ctx.Duration, ctx.Error)
//
//	    case "minio":
//	        o.metrics.RecordStorageOperation(ctx.Operation, ctx.Resource, ctx.SubResource, ctx.Size, ctx.Duration, ctx.Error)
//
//	    case "rabbitmq":
//	        o.metrics.RecordQueueOperation(ctx.Operation, ctx.Resource, ctx.SubResource, ctx.Duration, ctx.Error)
//
//	    case "kafka":
//	        o.metrics.RecordKafkaOperation(ctx.Operation, ctx.Resource, ctx.Size, ctx.Duration, ctx.Error)
//	    }
//	}
//
// # Multi-Purpose Observer
//
// A single observer can handle metrics, tracing, and logging:
//
//	type CompositeObserver struct {
//	    metrics *prometheus.Metrics
//	    tracer  trace.Tracer
//	    logger  *zap.Logger
//	}
//
//	func (o *CompositeObserver) ObserveOperation(ctx observability.OperationContext) {
//	    // Record metrics
//	    o.metrics.RecordOperation(ctx)
//
//	    // Create trace span
//	    span := o.tracer.StartSpan(fmt.Sprintf("%s.%s", ctx.Component, ctx.Operation))
//	    span.SetAttributes("resource", ctx.Resource)
//	    span.Finish()
//
//	    // Log operation
//	    if ctx.Error != nil {
//	        o.logger.Error("operation failed",
//	            zap.String("component", ctx.Component),
//	            zap.String("operation", ctx.Operation),
//	            zap.Error(ctx.Error),
//	        )
//	    }
//	}
//
// # FX Integration
//
// Wire the observer through dependency injection:
//
//	// Provide observer implementation
//	fx.Provide(
//	    fx.Annotate(
//	        NewMetricsObserver,
//	        fx.As(new(observability.Observer)),
//	    ),
//	)
//
//	// Observer automatically injected into all std config providers
//	func PostgresConfigProvider(cfg Config, observer observability.Observer) stdPostgres.Config {
//	    return stdPostgres.Config{
//	        Host:     cfg.GetHost(),
//	        Observer: observer, // ← Automatically wired
//	    }
//	}
//
// # OperationContext Fields
//
// The OperationContext struct provides a flexible way to describe any infrastructure operation:
//
//   - Component: Which std package (postgres, minio, kafka, etc.)
//   - Operation: What was done (insert, put, publish, etc.)
//   - Resource:  Primary resource (table, bucket, topic, etc.)
//   - SubResource: Secondary resource (key, routing key, partition, etc.)
//   - Duration:  How long it took
//   - Error:     Any error that occurred
//   - Size:      Size of data (rows, bytes, etc.)
//   - Metadata:  Additional context
//
// # Examples Across Different Infrastructure
//
// Database (Postgres/MariaDB):
//
//	OperationContext{
//	    Component: "postgres",
//	    Operation: "insert",
//	    Resource:  "users",
//	    Duration:  23 * time.Millisecond,
//	    Size:      1, // rows affected
//	}
//
// Object Storage (MinIO):
//
//	OperationContext{
//	    Component:   "minio",
//	    Operation:   "put",
//	    Resource:    "uploads",
//	    SubResource: "files/123/data.json",
//	    Duration:    145 * time.Millisecond,
//	    Size:        1024000, // bytes
//	}
//
// Message Queue (RabbitMQ):
//
//	OperationContext{
//	    Component:   "rabbitmq",
//	    Operation:   "publish",
//	    Resource:    "events",
//	    SubResource: "user.created",
//	    Duration:    5 * time.Millisecond,
//	    Size:        512, // message size
//	}
//
// Stream Platform (Kafka):
//
//	OperationContext{
//	    Component:   "kafka",
//	    Operation:   "produce",
//	    Resource:    "user-events",
//	    SubResource: "3", // partition
//	    Duration:    12 * time.Millisecond,
//	    Size:        2048, // message size
//	    Metadata:    map[string]interface{}{"offset": 12345},
//	}
//
// # Performance
//
// The observer pattern adds minimal overhead:
//   - One nil check per operation
//   - One function call if observer is present
//   - ~1-5 nanoseconds overhead
//   - No allocations if observer is nil
//
// # Thread Safety
//
// Observer implementations must be thread-safe. They will be called concurrently
// from multiple goroutines.
package observability
