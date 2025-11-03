// Package rabbit provides functionality for interacting with RabbitMQ.
//
// The rabbit package offers a simplified interface for working with RabbitMQ
// message queues, providing connection management, message publishing, and
// consuming capabilities with a focus on reliability and ease of use.
//
// Core Features:
//   - Robust connection management with automatic reconnection
//   - Simple publishing interface with error handling
//   - Consumer interface with automatic acknowledgment handling
//   - Dead letter queue support
//   - Integration with the Logger package for structured logging
//   - Distributed tracing support via message headers
//
// Basic Usage:
//
//	import (
//		"github.com/Aleph-Alpha/data-go-packages/pkg/rabbit"
//		"github.com/Aleph-Alpha/data-go-packages/pkg/logger"
//		"context"
//		"sync"
//	)
//
//
//	// Create a new RabbitMQ client
//	client, err := rabbit.New(rabbit.Config{
//		Connection: rabbit.ConnectionConfig{
//			URI: "amqp://guest:guest@localhost:5672/",
//		},
//		Channel: rabbit.ChannelConfig{
//			ExchangeName: "events",
//			ExchangeType: "topic",
//			RoutingKey:   "user.created",
//			QueueName:    "user-events",
//			ContentType:  "application/json",
//		},
//	}, log)
//	if err != nil {
//		log.Fatal("Failed to connect to RabbitMQ", err, nil)
//	}
//	defer client.Close()
//
//	// Publish a message
//	ctx := context.Background()
//	message := []byte(`{"id": "123", "name": "John"}`)
//	err = client.Publish(ctx, message, nil)
//	if err != nil {
//		log.Error("Failed to publish message", err, nil)
//	}
//
//	// Consume messages
//	wg := &sync.WaitGroup{}
//	ctx, cancel := context.WithCancel(context.Background())
//	defer cancel()
//
//	msgChan := client.Consume(ctx, wg)
//	for msg := range msgChan {
//		log.Info("Received message", nil, map[string]interface{}{
//			"body": string(msg.Body()),
//		})
//
//		// Process the message
//
//		// Acknowledge the message
//		if err := msg.AckMsg(); err != nil {
//			log.Error("Failed to acknowledge message", err, nil)
//		}
//	}
//
// Distributed Tracing with Message Headers:
//
// This package supports distributed tracing by allowing you to propagate trace context
// through message headers, enabling end-to-end visibility across services.
//
// Publisher Example (sending trace context):
//
//	import (
//		"github.com/Aleph-Alpha/data-go-packages/pkg/tracer"
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
