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
//
// Basic Usage:
//
//	import (
//		"gitlab.aleph-alpha.de/engineering/pharia-data-search/data-go-packages/pkg/rabbit"
//		"gitlab.aleph-alpha.de/engineering/pharia-data-search/data-go-packages/pkg/logger"
//		"context"
//		"sync"
//	)
//
//	// Create a logger
//	log, _ := logger.NewLogger(logger.Config{Level: "info"})
//
//	// Create a new RabbitMQ client
//	, err := rabbit.New(rabbit.Config{
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
//	err = client.Publish(ctx, message)
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
