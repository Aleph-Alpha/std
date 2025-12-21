// Package kafka provides functionality for interacting with Apache Kafka.
//
// The kafka package offers a simplified interface for working with Kafka
// message brokers, providing connection management, message publishing, and
// consuming capabilities with a focus on reliability and ease of use.
//
// Core Features:
//   - Robust connection management with automatic reconnection
//   - Simple publishing interface with error handling
//   - Consumer interface with automatic commit handling
//   - Consumer group support
//   - Integration with the Logger package for structured logging
//   - Distributed tracing support via message headers
//
// Basic Usage:
//
//	import (
//		"github.com/Aleph-Alpha/std/v1/kafka"
//		"github.com/Aleph-Alpha/std/v1/logger"
//		"context"
//		"sync"
//	)
//
//	// Create a new Kafka client
//	client, err := kafka.NewClient(kafka.Config{
//		Brokers: []string{"localhost:9092"},
//		Topic:   "events",
//		GroupID: "my-consumer-group",
//	})
//	if err != nil {
//		log.Fatal("Failed to connect to Kafka", err, nil)
//	}
//	defer client.Close()
//
//	// Publish a message
//	ctx := context.Background()
//	message := []byte(`{"id": "123", "name": "John"}`)
//	err = client.Publish(ctx, "key", message, nil)
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
//		// Commit the message
//		if err := msg.CommitMsg(); err != nil {
//			log.Error("Failed to commit message", err, nil)
//		}
//	}
//
// High-Throughput Consumption with Parallel Workers:
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
//			log.Error("Failed to commit message", err, nil)
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
// Thread Safety:
//
// All methods on the Kafka type are safe for concurrent use by multiple
// goroutines, except for Close() which should only be called once.
package kafka
