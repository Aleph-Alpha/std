// Package kafka provides functionality for interacting with Apache Kafka.
//
// The kafka package offers a simplified interface for working with Kafka
// message brokers, providing connection management, message publishing, and
// consuming capabilities with a focus on reliability and ease of use.
//
// # Architecture
//
// The package follows the "accept interfaces, return structs" Go idiom:
//   - Client interface: Defines the contract for Kafka operations
//   - KafkaClient struct: Concrete implementation of the Client interface
//   - Message interface: Defines the contract for consumed messages
//   - Constructor returns *KafkaClient (concrete type)
//   - FX module provides both *KafkaClient and Client interface
//
// This design allows:
//   - Direct usage: Use *KafkaClient for simple cases
//   - Interface usage: Depend on Client interface for testability and flexibility
//   - Zero adapters needed: Consumer code can use type aliases
//
// Core Features:
//   - Robust connection management with automatic reconnection
//   - Simple publishing interface with error handling
//   - Consumer interface with automatic commit handling
//   - Consumer group support
//   - Separate schema_registry package for Confluent Schema Registry
//   - Integration with the Logger package for structured logging
//   - Distributed tracing support via message headers
//
// # Basic Usage (Direct)
//
//	import (
//		"github.com/Aleph-Alpha/std/v1/kafka"
//		"context"
//		"sync"
//	)
//
//	// Create a new Kafka client (returns concrete *KafkaClient)
//	client, err := kafka.NewClient(kafka.Config{
//		Brokers:    []string{"localhost:9092"},
//		Topic:      "events",
//		GroupID:    "my-consumer-group",
//		IsConsumer: true,
//		EnableAutoCommit: false,  // Manual commit for safety
//		DataType:   "json",       // Automatic JSON serializer (default)
//	})
//	if err != nil {
//		log.Fatal("Failed to connect to Kafka", err)
//	}
//	defer client.GracefulShutdown()
//
//	// Publish a message (automatically serialized as JSON)
//	ctx := context.Background()
//	event := map[string]interface{}{"id": "123", "name": "John"}
//	err = client.Publish(ctx, "key", event)
//	if err != nil {
//		log.Printf("Failed to publish message: %v", err)
//	}
//
// # FX Module Integration
//
// The package provides an FX module that injects both concrete and interface types:
//
//	import (
//		"github.com/Aleph-Alpha/std/v1/kafka"
//		"go.uber.org/fx"
//	)
//
//	app := fx.New(
//		kafka.FXModule,
//		fx.Provide(
//			func() kafka.Config {
//				return kafka.Config{
//					Brokers:    []string{"localhost:9092"},
//					Topic:      "events",
//					IsConsumer: false,
//					DataType:   "json",
//				}
//			},
//		),
//		fx.Invoke(func(k kafka.Client) {
//			// Use the Client interface
//			ctx := context.Background()
//			event := map[string]interface{}{"id": "123"}
//			k.Publish(ctx, "key", event)
//		}),
//	)
//	app.Run()
//
// # Using Type Aliases (Recommended for Consumer Code)
//
// Consumer applications can use type aliases to avoid creating adapters:
//
//	// In your application's messaging package
//	package messaging
//
//	import stdKafka "github.com/Aleph-Alpha/std/v1/kafka"
//
//	// Type aliases reference std interfaces directly
//	type Client = stdKafka.Client
//	type Message = stdKafka.Message
//
// Then use these aliases throughout your application:
//
//	func MyService(kafka messaging.Client) {
//		kafka.Publish(ctx, "key", data)
//	}
//
// # Consuming Messages
//
//	wg := &sync.WaitGroup{}
//	ctx, cancel := context.WithCancel(context.Background())
//	defer cancel()
//
//	msgChan := client.Consume(ctx, wg)
//	for msg := range msgChan {
//		// Process the message
//		fmt.Printf("Received: %s\n", string(msg.Body()))
//
//		// Commit the message
//		if err := msg.CommitMsg(); err != nil {
//			log.Printf("Failed to commit: %v", err)
//		}
//	}
//
// # High-Throughput Consumption with Parallel Workers
//
// For high-volume topics, use ConsumeParallel to process messages concurrently:
//
//	wg := &sync.WaitGroup{}
//	ctx, cancel := context.WithCancel(context.Background())
//	defer cancel()
//
//	// Use 5 concurrent workers for better throughput
//	msgChan := client.ConsumeParallel(ctx, wg, 5)
//	for msg := range msgChan {
//		// Process messages concurrently
//		processMessage(msg)
//
//		// Commit the message
//		if err := msg.CommitMsg(); err != nil {
//			log.Printf("Failed to commit: %v", err)
//		}
//	}
//
// # Distributed Tracing with Message Headers
//
// This package supports distributed tracing by allowing you to propagate trace context
// through message headers, enabling end-to-end visibility across services.
//
// Publisher Example (sending trace context):
//
//	import (
//		"github.com/Aleph-Alpha/std/v1/tracer"
//		// other imports...
//	)
//
//	// Create a tracer client
//	tracerClient := tracer.NewClient(tracerConfig, log)
//
//	// Create a span for the operation that includes publishing
//	ctx, span := tracerClient.StartSpan(ctx, "process-and-publish")
//	defer span.End()
//
//	// Process data...
//
//	// Extract trace context as headers before publishing
//	traceHeaders := tracerClient.GetCarrier(ctx)
//
//	// Publish with trace headers
//	err = kafkaClient.Publish(ctx, "key", message, traceHeaders)
//	if err != nil {
//		span.RecordError(err)
//		log.Error("Failed to publish message", err, nil)
//	}
//
// Consumer Example (continuing the trace):
//
//	msgChan := kafkaClient.Consume(ctx, wg)
//	for msg := range msgChan {
//		// Extract trace headers from the message
//		headers := msg.Header()
//
//		// Create a new context with the trace information
//		ctx = tracerClient.SetCarrierOnContext(ctx, headers)
//
//		// Create a span as a child of the incoming trace
//		ctx, span := tracerClient.StartSpan(ctx, "process-message")
//		defer span.End()
//
//		// Add relevant attributes to the span
//		span.SetAttributes(map[string]interface{}{
//			"message.size": len(msg.Body()),
//			"message.key":  msg.Key(),
//		})
//
//		// Process the message...
//
//		if err := processMessage(msg.Body()) {
//			// Record any errors in the span
//			span.RecordError(err)
//			continue
//		}
//
//		// Commit successful processing
//		if err := msg.CommitMsg(); err != nil {
//			span.RecordError(err)
//			log.Error("Failed to commit message", err, nil)
//		}
//	}
//
// FX Module Integration:
//
// This package provides a fx module for easy integration:
//
//	app := fx.New(
//		logger.FXModule, // Optional: provides std logger
//		kafka.FXModule,
//		// ... other modules
//	)
//	app.Run()
//
// The Kafka module will automatically use the logger if it's available in the
// dependency injection container.
//
// Configuration:
//
// The kafka client can be configured via environment variables or explicitly:
//
//	KAFKA_BROKERS=localhost:9092,localhost:9093
//	KAFKA_TOPIC=events
//	KAFKA_GROUP_ID=my-consumer-group
//
// Custom Logger Integration:
//
// You can integrate the std/v1/logger for better error logging:
//
//	import (
//		"github.com/Aleph-Alpha/std/v1/logger"
//		"github.com/Aleph-Alpha/std/v1/kafka"
//	)
//
//	// Create logger
//	log := logger.NewLoggerClient(logger.Config{
//		Level:       logger.Info,
//		ServiceName: "my-service",
//	})
//
//	// Create Kafka client with logger
//	client, err := kafka.NewClient(kafka.Config{
//		Brokers:    []string{"localhost:9092"},
//		Topic:      "events",
//		Logger:     log, // Kafka internal errors will use this logger
//		IsConsumer: false,
//	})
//
// Commit Modes and Acknowledgment:
//
// Consumer Commit Modes:
//
// The module supports different commit modes for consumers:
//
// Manual Commit (Recommended for safety):
//
//	client, err := kafka.NewClient(kafka.Config{
//	    Brokers:          []string{"localhost:9092"},
//	    Topic:            "events",
//	    GroupID:          "my-group",
//	    IsConsumer:       true,
//	    EnableAutoCommit: false,  // Manual commit (default)
//	})
//
//	msgChan := client.Consume(ctx, wg)
//	for msg := range msgChan {
//	    var event UserEvent
//	    if err := msg.BodyAs(&event); err != nil {
//	        continue  // Don't commit on error
//	    }
//
//	    if err := processEvent(event); err != nil {
//	        continue  // Don't commit on processing error
//	    }
//
//	    // Commit only after successful processing
//	    if err := msg.CommitMsg(); err != nil {
//	        log.Error("Failed to commit", err, nil)
//	    }
//	}
//
// Auto-Commit (For high-throughput, at-least-once semantics):
//
//	client, err := kafka.NewClient(kafka.Config{
//	    Brokers:          []string{"localhost:9092"},
//	    Topic:            "events",
//	    GroupID:          "my-group",
//	    IsConsumer:       true,
//	    EnableAutoCommit: true,           // Enable auto-commit
//	    CommitInterval:   1 * time.Second,  // Commit every second
//	})
//
//	msgChan := client.Consume(ctx, wg)
//	for msg := range msgChan {
//	    // Process message - no need to call msg.CommitMsg()
//	    // Offsets are committed automatically every CommitInterval
//	    processEvent(msg)
//	}
//
// Producer Acknowledgment Modes:
//
// The module supports different producer acknowledgment modes:
//
// Fire-and-Forget (Fastest, least safe):
//
//	client, err := kafka.NewClient(kafka.Config{
//	    Brokers:      []string{"localhost:9092"},
//	    Topic:        "events",
//	    IsConsumer:   false,
//	    RequiredAcks: kafka.RequireNone,  // No acknowledgment (0)
//	})
//
//	// Fast but no guarantee of delivery
//	err := client.Publish(ctx, "key", event)
//
// Leader Acknowledgment (Balanced):
//
//	client, err := kafka.NewClient(kafka.Config{
//	    Brokers:      []string{"localhost:9092"},
//	    Topic:        "events",
//	    IsConsumer:   false,
//	    RequiredAcks: kafka.RequireOne,  // Leader only (1)
//	    WriteTimeout: 5 * time.Second,
//	})
//
//	// Balanced speed and durability
//	err := client.Publish(ctx, "key", event)
//
// All Replicas Acknowledgment (Most durable, default):
//
//	client, err := kafka.NewClient(kafka.Config{
//	    Brokers:      []string{"localhost:9092"},
//	    Topic:        "events",
//	    IsConsumer:   false,
//	    RequiredAcks: kafka.RequireAll,  // All in-sync replicas (-1)
//	    WriteTimeout: 10 * time.Second,
//	})
//
//	// Slowest but most durable
//	err := client.Publish(ctx, "key", event)
//
// Automatic Serializer Selection:
//
// The Kafka client can automatically select serializers based on the DataType config.
// This eliminates the need to manually configure serializers for common use cases.
//
// JSON (default):
//
//	client, err := kafka.NewClient(kafka.Config{
//	    Brokers:  []string{"localhost:9092"},
//	    Topic:    "events",
//	    DataType: "json",  // Auto-uses JSONSerializer
//	})
//
//	// Can now publish structs directly
//	event := UserEvent{Name: "John", Age: 30}
//	err = client.Publish(ctx, "key", event)  // Auto-serialized to JSON
//
// String:
//
//	client, err := kafka.NewClient(kafka.Config{
//	    Brokers:  []string{"localhost:9092"},
//	    Topic:    "logs",
//	    DataType: "string",  // Auto-uses StringSerializer
//	})
//
//	err = client.Publish(ctx, "key", "log message")
//
// Gob (Go binary encoding):
//
//	client, err := kafka.NewClient(kafka.Config{
//	    Brokers:  []string{"localhost:9092"},
//	    Topic:    "data",
//	    DataType: "gob",  // Auto-uses GobSerializer
//	})
//
//	data := MyStruct{Field1: "value", Field2: 123}
//	err = client.Publish(ctx, "key", data)
//
// Raw bytes (no serialization):
//
//	client, err := kafka.NewClient(kafka.Config{
//	    Brokers:  []string{"localhost:9092"},
//	    Topic:    "binary",
//	    DataType: "bytes",  // No-op serializer
//	})
//
//	rawBytes := []byte{0x01, 0x02, 0x03}
//	err = client.Publish(ctx, "key", rawBytes)
//
// Supported DataTypes:
//   - "json" (default): JSONSerializer/Deserializer
//   - "string": StringSerializer/Deserializer
//   - "gob": GobSerializer/Deserializer
//   - "bytes": NoOpSerializer/Deserializer
//   - "protobuf": Requires custom serializer
//   - "avro": Requires custom serializer
//
// Serialization Support:
//
// The kafka package provides optional serialization support for automatic encoding/decoding
// of messages. Serializers are injected via dependency injection (FX) for better testability.
//
// Using JSON Serialization (with FX):
//
//	import (
//	    "go.uber.org/fx"
//	    "github.com/Aleph-Alpha/std/v1/kafka"
//	)
//
//	type UserEvent struct {
//	    Event  string `json:"event"`
//	    UserID int    `json:"user_id"`
//	    Email  string `json:"email"`
//	}
//
//	app := fx.New(
//	    kafka.FXModule,
//	    fx.Provide(
//	        func() kafka.Config {
//	            return kafka.Config{
//	                Brokers:    []string{"localhost:9092"},
//	                Topic:      "user-events",
//	                IsConsumer: false,
//	            }
//	        },
//	        // Inject serializers
//	        func() kafka.Serializer {
//	            return &kafka.JSONSerializer{}
//	        },
//	        func() kafka.Deserializer {
//	            return &kafka.JSONDeserializer{}
//	        },
//	    ),
//	)
//
//	// Publishing with automatic serialization
//	event := UserEvent{
//	    Event:  "signup",
//	    UserID: 123,
//	    Email:  "user@example.com",
//	}
//	err := client.Publish(ctx, "user-123", event)  // Automatically serialized
//
//	// Consuming with automatic deserialization
//	msgChan := client.Consume(ctx, wg)
//	for msg := range msgChan {
//	    var event UserEvent
//	    if err := msg.BodyAs(&event); err != nil {  // Automatic JSON fallback
//	        log.Error("Failed to deserialize", err, nil)
//	        continue
//	    }
//	    fmt.Printf("User %d signed up: %s\n", event.UserID, event.Email)
//	    msg.CommitMsg()
//	}
//
// Using Protocol Buffers (with FX):
//
//	import (
//	    "google.golang.org/protobuf/proto"
//	    "github.com/Aleph-Alpha/std/v1/kafka"
//	)
//
//	app := fx.New(
//	    kafka.FXModule,
//	    fx.Provide(
//	        func() kafka.Config { /* ... */ },
//	        // Inject Protobuf serializers
//	        func() kafka.Serializer {
//	            return &kafka.ProtobufSerializer{
//	                MarshalFunc: proto.Marshal,
//	            }
//	        },
//	        func() kafka.Deserializer {
//	            return &kafka.ProtobufDeserializer{
//	                UnmarshalFunc: proto.Unmarshal,
//	            }
//	        },
//	    ),
//	)
//
//	// Publish protobuf message
//	protoMsg := &pb.UserEvent{
//	    Event:  "signup",
//	    UserId: 123,
//	    Email:  "user@example.com",
//	}
//	err = client.Publish(ctx, "user-123", protoMsg)  // Automatically serialized
//
//	// Consume protobuf messages
//	msgChan := consumer.Consume(ctx, wg)
//	for msg := range msgChan {
//	    var event pb.UserEvent
//	    if err := msg.BodyAs(&event); err != nil{
//	        log.Error("Failed to deserialize", err, nil)
//	        continue
//	    }
//	    fmt.Printf("User %d signed up: %s\n", event.UserId, event.Email)
//	    msg.CommitMsg()
//	}
//
// Using Raw Bytes (no serialization):
//
//	// No serializer needed - []byte is always passed through
//	client, err := kafka.NewClient(kafka.Config{
//	    Brokers:    []string{"localhost:9092"},
//	    Topic:      "events",
//	    IsConsumer: false,
//	})
//
//	// Publish raw bytes
//	message := []byte(`{"event":"signup","user_id":123}`)
//	err = client.Publish(ctx, "user-123", message)  // Passed through directly
//
// Custom Serializers:
//
// You can implement your own serializer for custom formats:
//
// Apache Avro Example:
//
//	import "github.com/linkedin/goavro/v2"
//
//	codec, _ := goavro.NewCodec(`{
//	    "type": "record",
//	    "name": "UserEvent",
//	    "fields": [
//	        {"name": "event", "type": "string"},
//	        {"name": "user_id", "type": "int"}
//	    ]
//	}`)
//
//	client, err := kafka.NewClient(kafka.Config{
//	    Brokers: []string{"localhost:9092"},
//	    Topic:   "events",
//	    Serializer: &kafka.AvroSerializer{
//	        MarshalFunc: func(data interface{}) ([]byte, error) {
//	            return codec.BinaryFromNative(nil, data)
//	        },
//	    },
//	    Deserializer: &kafka.AvroDeserializer{
//	        UnmarshalFunc: func(data []byte, target interface{}) error {
//	            native, _, err := codec.NativeFromBinary(data)
//	            if err != nil {
//	                return err
//	            }
//	            if targetMap, ok := target.(*map[string]interface{}); ok {
//	                *targetMap = native.(map[string]interface{})
//	            }
//	            return nil
//	        },
//	    },
//	})
//
//	// Publish and consume work the same way
//	eventData := map[string]interface{}{"event": "signup", "user_id": 123}
//	err = client.Publish(ctx, "user-123", eventData)
//
// Schema Registry Integration:
//
// For production schema management with Confluent Schema Registry,
// use the separate schema_registry package:
//
//	import (
//	    "github.com/Aleph-Alpha/std/v1/kafka"
//	    "github.com/Aleph-Alpha/std/v1/schema_registry"
//	)
//
//	// Create schema registry client
//	registry, _ := schema_registry.NewClient(schema_registry.Config{
//	    URL: "http://localhost:8081",
//	})
//
//	// Create serializer with schema registry
//	serializer, _ := schema_registry.NewAvroSerializer(
//	    schema_registry.AvroSerializerConfig{
//	        Registry:    registry,
//	        Subject:     "users-value",
//	        Schema:      avroSchema,
//	        MarshalFunc: marshalFunc,
//	    },
//	)
//
//	// Inject via FX
//	fx.Provide(
//	    func() (schema_registry.Registry, error) {
//	        return schema_registry.NewClient(schema_registry.Config{
//	            URL: "http://localhost:8081",
//	        })
//	    },
//	    func(registry schema_registry.Registry) (kafka.Serializer, error) {
//	        return schema_registry.NewAvroSerializer(...)
//	    },
//	)
//
// See the schema_registry package documentation for complete examples.
//
// Multi-Format Support:
//
// Use MultiFormatSerializer to support multiple formats:
//
//	multiSerializer := kafka.NewMultiFormatSerializer()
//	multiSerializer.RegisterSerializer("protobuf", &kafka.ProtobufSerializer{
//	    MarshalFunc: proto.Marshal,
//	})
//
//	client, err := kafka.NewClient(kafka.Config{
//	    Brokers:    []string{"localhost:9092"},
//	    Topic:      "events",
//	    Serializer: multiSerializer,
//	})
//
// Built-in Serializers:
//   - JSONSerializer: JSON format (default fallback)
//   - ProtobufSerializer: Protocol Buffers
//   - AvroSerializer: Apache Avro
//   - StringSerializer: Plain text
//   - GobSerializer: Go gob encoding (Go-to-Go only)
//   - NoOpSerializer: Raw []byte passthrough
//   - MultiFormatSerializer: Multiple formats with dynamic selection
//
// # Observability
//
// The Kafka client supports optional observability through the Observer interface from
// std/v1/observability. This allows external code to track all Kafka operations for
// metrics, tracing, and logging without tight coupling.
//
// # Basic Observability Setup
//
// To enable observability, provide an Observer implementation in the configuration:
//
//	import "github.com/Aleph-Alpha/std/v1/observability"
//
//	// Implement the Observer interface
//	type MyObserver struct {
//	    metrics *prometheus.Metrics
//	}
//
//	func (o *MyObserver) ObserveOperation(ctx observability.OperationContext) {
//	    if ctx.Component == "kafka" {
//	        switch ctx.Operation {
//	        case "produce":
//	            o.metrics.RecordKafkaPublish(
//	                ctx.Resource,  // topic name
//	                ctx.Size,      // message bytes
//	                ctx.Duration,  // operation duration
//	                ctx.Error,     // error if any
//	            )
//	        case "consume":
//	            o.metrics.RecordKafkaConsume(
//	                ctx.Resource,    // topic name
//	                ctx.SubResource, // partition
//	                ctx.Size,        // message bytes
//	                ctx.Duration,    // fetch duration
//	                ctx.Error,       // error if any
//	            )
//	        }
//	    }
//	}
//
//	// Configure Kafka client with observer
//	observer := &MyObserver{metrics: myMetrics}
//
//	client, err := kafka.NewClient(kafka.Config{
//	    Brokers:  []string{"localhost:9092"},
//	    Topic:    "events",
//	    Observer: observer, // ← Enable observability
//	})
//
// # Observed Operations
//
// The observer is notified of the following operations:
//
// Produce Operation:
//   - Component: "kafka"
//   - Operation: "produce"
//   - Resource: Topic name (e.g., "user-events")
//   - SubResource: "" (empty)
//   - Duration: Time taken to publish message
//   - Size: Message size in bytes
//   - Error: Any error that occurred during publishing
//   - Metadata: nil (reserved for future use)
//
// Consume Operation:
//   - Component: "kafka"
//   - Operation: "consume"
//   - Resource: Topic name (e.g., "user-events")
//   - SubResource: Partition number (e.g., "3")
//   - Duration: Time taken to fetch message from Kafka
//   - Size: Message size in bytes
//   - Error: Any error that occurred during consumption
//   - Metadata: nil (reserved for future use)
//
// Example OperationContext for Publish:
//
//	observability.OperationContext{
//	    Component:   "kafka",
//	    Operation:   "produce",
//	    Resource:    "user-events",
//	    SubResource: "",
//	    Duration:    12 * time.Millisecond,
//	    Size:        2048,
//	    Error:       nil,
//	}
//
// Example OperationContext for Consume:
//
//	observability.OperationContext{
//	    Component:   "kafka",
//	    Operation:   "consume",
//	    Resource:    "user-events",
//	    SubResource: "3", // partition
//	    Duration:    8 * time.Millisecond,
//	    Size:        1024,
//	    Error:       nil,
//	}
//
// # Integration with Prometheus
//
// Example: Track Kafka metrics with Prometheus:
//
//	import (
//	    "github.com/prometheus/client_golang/prometheus"
//	    "github.com/Aleph-Alpha/std/v1/observability"
//	)
//
//	type KafkaObserver struct {
//	    publishTotal    *prometheus.CounterVec
//	    publishDuration *prometheus.HistogramVec
//	    publishBytes    *prometheus.CounterVec
//	    consumeTotal    *prometheus.CounterVec
//	    consumeDuration *prometheus.HistogramVec
//	    consumeBytes    *prometheus.CounterVec
//	}
//
//	func NewKafkaObserver() *KafkaObserver {
//	    return &KafkaObserver{
//	        publishTotal: prometheus.NewCounterVec(
//	            prometheus.CounterOpts{
//	                Name: "kafka_messages_published_total",
//	                Help: "Total number of messages published to Kafka",
//	            },
//	            []string{"topic", "status"},
//	        ),
//	        publishDuration: prometheus.NewHistogramVec(
//	            prometheus.HistogramOpts{
//	                Name:    "kafka_publish_duration_seconds",
//	                Help:    "Duration of Kafka publish operations",
//	                Buckets: prometheus.DefBuckets,
//	            },
//	            []string{"topic"},
//	        ),
//	        publishBytes: prometheus.NewCounterVec(
//	            prometheus.CounterOpts{
//	                Name: "kafka_publish_bytes_total",
//	                Help: "Total bytes published to Kafka",
//	            },
//	            []string{"topic"},
//	        ),
//	        consumeTotal: prometheus.NewCounterVec(
//	            prometheus.CounterOpts{
//	                Name: "kafka_messages_consumed_total",
//	                Help: "Total number of messages consumed from Kafka",
//	            },
//	            []string{"topic", "partition", "status"},
//	        ),
//	        consumeDuration: prometheus.NewHistogramVec(
//	            prometheus.HistogramOpts{
//	                Name:    "kafka_consume_duration_seconds",
//	                Help:    "Duration of Kafka consume operations",
//	                Buckets: prometheus.DefBuckets,
//	            },
//	            []string{"topic"},
//	        ),
//	        consumeBytes: prometheus.NewCounterVec(
//	            prometheus.CounterOpts{
//	                Name: "kafka_consume_bytes_total",
//	                Help: "Total bytes consumed from Kafka",
//	            },
//	            []string{"topic"},
//	        ),
//	    }
//	}
//
//	func (o *KafkaObserver) ObserveOperation(ctx observability.OperationContext) {
//	    if ctx.Component != "kafka" {
//	        return
//	    }
//
//	    status := "success"
//	    if ctx.Error != nil {
//	        status = "error"
//	    }
//
//	    switch ctx.Operation {
//	    case "produce":
//	        o.publishTotal.WithLabelValues(ctx.Resource, status).Inc()
//	        o.publishDuration.WithLabelValues(ctx.Resource).Observe(ctx.Duration.Seconds())
//	        if ctx.Error == nil {
//	            o.publishBytes.WithLabelValues(ctx.Resource).Add(float64(ctx.Size))
//	        }
//
//	    case "consume":
//	        o.consumeTotal.WithLabelValues(ctx.Resource, ctx.SubResource, status).Inc()
//	        o.consumeDuration.WithLabelValues(ctx.Resource).Observe(ctx.Duration.Seconds())
//	        if ctx.Error == nil {
//	            o.consumeBytes.WithLabelValues(ctx.Resource).Add(float64(ctx.Size))
//	        }
//	    }
//	}
//
// # FX Module with Observer
//
// When using FX, you can inject an observer into the Kafka client:
//
//	import (
//	    "go.uber.org/fx"
//	    "github.com/Aleph-Alpha/std/v1/kafka"
//	    "github.com/Aleph-Alpha/std/v1/observability"
//	)
//
//	app := fx.New(
//	    kafka.FXModule,
//	    fx.Provide(
//	        // Provide your observer implementation
//	        func() observability.Observer {
//	            return NewKafkaObserver()
//	        },
//	        // Provide Kafka config with observer
//	        func(observer observability.Observer) kafka.Config {
//	            return kafka.Config{
//	                Brokers:  []string{"localhost:9092"},
//	                Topic:    "events",
//	                Observer: observer, // Inject observer
//	            }
//	        },
//	    ),
//	)
//	app.Run()
//
// # No-Op Observer
//
// If you don't provide an observer, there's zero overhead:
//
//	client, err := kafka.NewClient(kafka.Config{
//	    Brokers:  []string{"localhost:9092"},
//	    Topic:    "events",
//	    Observer: nil, // ← No observer, no overhead
//	})
//
// The client checks if Observer is nil before calling it, so there's no performance
// impact when observability is disabled.
//
// # Thread Safety
//
// All methods on the Kafka type are safe for concurrent use by multiple
// goroutines, except for Close() which should only be called once.
package kafka
