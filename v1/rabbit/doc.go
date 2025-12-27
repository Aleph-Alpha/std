// Package rabbit provides functionality for interacting with RabbitMQ.
//
// The rabbit package offers a simplified interface for working with RabbitMQ
// message queues, providing connection management, message publishing, and
// consuming capabilities with a focus on reliability and ease of use.
//
// # Architecture
//
// This package follows the "accept interfaces, return structs" design pattern:
//   - Client interface: Defines the contract for RabbitMQ operations
//   - RabbitClient struct: Concrete implementation of the Client interface
//   - Message interface: Defines the contract for consumed messages
//   - NewClient constructor: Returns *RabbitClient (concrete type)
//   - FX module: Provides both *RabbitClient and Client interface for dependency injection
//
// Core Features:
//   - Robust connection management with automatic reconnection
//   - Simple publishing interface with error handling
//   - Consumer interface with automatic acknowledgment handling
//   - Dead letter queue support
//   - Optional observability hooks for metrics and tracing
//   - Optional context-aware logging for lifecycle events
//   - Distributed tracing support via message headers
//
// # Direct Usage (Without FX)
//
// For simple applications or tests, create a client directly:
//
//	import (
//		"github.com/Aleph-Alpha/std/v1/rabbit"
//		"context"
//		"sync"
//	)
//
//	// Create a new RabbitMQ client (returns concrete *RabbitClient)
//	client, err := rabbit.NewClient(rabbit.Config{
//		Connection: rabbit.Connection{
//			Host: "localhost",
//			Port: 5672,
//			User: "guest",
//			Password: "guest",
//		},
//		Channel: rabbit.Channel{
//			ExchangeName: "events",
//			ExchangeType: "topic",
//			RoutingKey:   "user.created",
//			QueueName:    "user-events",
//			IsConsumer:   true,
//		},
//	})
//	if err != nil {
//		return err
//	}
//	defer client.GracefulShutdown()
//
//	// Optionally attach logger and observer
//	client = client.
//		WithLogger(myLogger).
//		WithObserver(myObserver)
//
//	// Publish a message
//	ctx := context.Background()
//	message := []byte(`{"id": "123", "name": "John"}`)
//	err = client.Publish(ctx, message)
//
// # Builder Pattern for Optional Dependencies
//
// The client supports optional dependencies via builder methods:
//
//	import (
//		"github.com/Aleph-Alpha/std/v1/rabbit"
//		"github.com/Aleph-Alpha/std/v1/logger"
//		"github.com/Aleph-Alpha/std/v1/observability"
//	)
//
//	// Create client with optional logger and observer
//	client, err := rabbit.NewClient(config)
//	if err != nil {
//		return err
//	}
//
//	// Attach optional dependencies using builder pattern
//	client = client.
//		WithLogger(loggerInstance).    // Optional: for lifecycle logging
//		WithObserver(observerInstance)  // Optional: for metrics/tracing
//
//	defer client.GracefulShutdown()
//
// # FX Module Integration
//
// For production applications using Uber's fx, use the FXModule which provides
// both the concrete type and interface, with automatic injection of optional
// dependencies:
//
//	import (
//		"github.com/Aleph-Alpha/std/v1/rabbit"
//		"github.com/Aleph-Alpha/std/v1/logger"
//		"github.com/Aleph-Alpha/std/v1/observability"
//		"go.uber.org/fx"
//	)
//
//	app := fx.New(
//		logger.FXModule,      // Optional: provides std logger
//		rabbit.FXModule,      // Provides *RabbitClient and rabbit.Client interface
//		fx.Provide(
//			func() rabbit.Config {
//				return rabbit.Config{
//					Connection: rabbit.Connection{
//						Host: "localhost",
//						Port: 5672,
//						User: "guest",
//						Password: "guest",
//					},
//					Channel: rabbit.Channel{
//						ExchangeName: "events",
//						QueueName:    "user-events",
//						IsConsumer:   true,
//					},
//				}
//			},
//			// Optional: provide observer for metrics
//			func(metrics *prometheus.Metrics) observability.Observer {
//				return NewObserverAdapter(metrics)
//			},
//		),
//		fx.Invoke(func(client *rabbit.RabbitClient) {
//			// Logger and Observer are automatically injected if provided
//			ctx := context.Background()
//			client.Publish(ctx, []byte("message"))
//		}),
//	)
//	app.Run()
//
// The FX module automatically injects optional dependencies:
//   - Logger (rabbit.Logger): If provided via fx, automatically attached
//   - Observer (observability.Observer): If provided via fx, automatically attached
//
// # Observability
//
// The package supports optional observability hooks for tracking operations.
// When an observer is attached, it will be notified of all publish and consume
// operations with detailed context.
//
// Observer Integration:
//
//	import (
//		"github.com/Aleph-Alpha/std/v1/observability"
//		"github.com/Aleph-Alpha/std/v1/rabbit"
//	)
//
//	// Create an observer (typically wraps your metrics system)
//	type MetricsObserver struct {
//		metrics *prometheus.Metrics
//	}
//
//	func (o *MetricsObserver) ObserveOperation(ctx observability.OperationContext) {
//		// Track metrics based on the operation
//		switch ctx.Operation {
//		case "produce":
//			o.metrics.MessageQueue.MessagesPublished.
//				WithLabelValues(ctx.Resource, ctx.SubResource, errorStatus(ctx.Error)).
//				Inc()
//		case "consume":
//			o.metrics.MessageQueue.MessagesConsumed.
//				WithLabelValues(ctx.Resource, errorStatus(ctx.Error)).
//				Inc()
//		}
//	}
//
//	// Attach observer to client
//	client = client.WithObserver(&MetricsObserver{metrics: promMetrics})
//
// Observer receives the following context for each operation:
//
// Publish operations:
//   - Component: "rabbit"
//   - Operation: "produce"
//   - Resource: exchange name
//   - SubResource: routing key
//   - Duration: time taken to publish
//   - Error: any error that occurred
//   - Size: message size in bytes
//
// Consume operations:
//   - Component: "rabbit"
//   - Operation: "consume"
//   - Resource: queue name
//   - SubResource: ""
//   - Duration: time taken to receive message
//   - Error: nil (errors in message processing are not tracked here)
//   - Size: message size in bytes
//
// # Logging
//
// The package supports optional context-aware logging for lifecycle events
// and background operations. When a logger is attached, it will be used for:
//   - Connection lifecycle events (connected, disconnected, reconnecting)
//   - Consumer lifecycle events (started, stopped, shutdown)
//   - Background reconnection attempts and errors
//
// Logger Integration:
//
//	import (
//		"github.com/Aleph-Alpha/std/v1/rabbit"
//		"github.com/Aleph-Alpha/std/v1/logger"
//	)
//
//	// Create logger instance
//	loggerClient, err := logger.NewLogger(loggerConfig)
//	if err != nil {
//		return err
//	}
//
//	// Attach logger to client
//	client = client.WithLogger(loggerClient)
//
// The logger interface matches std/v1/logger for seamless integration:
//
//	type Logger interface {
//		InfoWithContext(ctx context.Context, msg string, err error, fields ...map[string]interface{})
//		WarnWithContext(ctx context.Context, msg string, err error, fields ...map[string]interface{})
//		ErrorWithContext(ctx context.Context, msg string, err error, fields ...map[string]interface{})
//	}
//
// Logging is designed to be minimal and non-intrusive:
//   - Errors that are returned to the caller are NOT logged (avoid duplicate logs)
//   - Only background operations and lifecycle events are logged
//   - Context is propagated for distributed tracing
//
// # Type Aliases in Consumer Code
//
// To simplify your code and make it message-broker-agnostic, use type aliases:
//
//	package myapp
//
//	import stdRabbit "github.com/Aleph-Alpha/std/v1/rabbit"
//
//	// Use type alias to reference std's interface
//	type RabbitClient = stdRabbit.Client
//	type RabbitMessage = stdRabbit.Message
//
//	// Now use RabbitClient throughout your codebase
//	func MyFunction(client RabbitClient) {
//		client.Publish(ctx, []byte("message"))
//	}
//
// This eliminates the need for adapters and allows you to switch implementations
// by only changing the alias definition.
//
// # Message Consumption
//
//	// Consume messages
//	wg := &sync.WaitGroup{}
//	ctx, cancel := context.WithCancel(context.Background())
//	defer cancel()
//
//	msgChan := client.Consume(ctx, wg)
//	for msg := range msgChan {
//		// Process the message
//		log.Info("Received message", nil, map[string]interface{}{
//			"body": string(msg.Body()),
//		})
//
//		// Acknowledge the message
//		if err := msg.AckMsg(); err != nil {
//			log.Error("Failed to acknowledge message", err, nil)
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
//	err = rabbitClient.Publish(ctx, message, traceHeaders)
//	if err != nil {
//		span.RecordError(err)
//		log.Error("Failed to publish message", err, nil)
//	}
//
// Consumer Example (continuing the trace):
//
//	msgChan := rabbitClient.Consume(ctx, wg)
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
//			"message.type": "user.created",
//		})
//
//		// Process the message...
//
//		if err := processMessage(msg.Body()) {
//			// Record any errors in the span
//			span.RecordError(err)
//			msg.NackMsg(true) // Requeue for retry
//			continue
//		}
//
//		// Acknowledge successful processing
//		if err := msg.AckMsg(); err != nil {
//			span.RecordError(err)
//			log.Error("Failed to acknowledge message", err, nil)
//		}
//	}
//
// Consuming from Dead Letter Queue:
//
//	dlqChan := client.ConsumeDLQ(ctx, wg)
//	for msg := range dlqChan {
//		log.Info("Processing failed message", nil, map[string]interface{}{
//			"body": string(msg.Body()),
//		})
//
//		// Process the failed message
//
//		// Acknowledge after processing
//		msg.AckMsg()
//	}
//
// FX Module Integration:
//
// This package provides a fx module for easy integration:
//
//	app := fx.New(
//		logger.Module,
//		rabbit.Module,
//		// ... other modules
//	)
//	app.Run()
//
// Configuration:
//
// The rabbit client can be configured via environment variables or explicitly:
//
//	RABBIT_URI=amqp://guest:guest@localhost:5672/
//	RABBIT_EXCHANGE_NAME=events
//	RABBIT_EXCHANGE_TYPE=topic
//	RABBIT_ROUTING_KEY=user.created
//	RABBIT_QUEUE_NAME=user-events
//
// Thread Safety:
//
// All methods on the Rabbit type are safe for concurrent use by multiple
// goroutines, except for Close() which should only be called once.
package rabbit
