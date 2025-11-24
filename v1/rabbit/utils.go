package rabbit

import (
	"context"
	"log"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

// Message defines the interface for consumed messages from RabbitMQ.
// This interface abstracts the underlying AMQP message structure and
// provides methods for acknowledging or rejecting messages.
type Message interface {
	// AckMsg acknowledges the message, informing RabbitMQ that the message
	// has been successfully processed and can be removed from the queue.
	AckMsg() error

	// NackMsg rejects the message. If requeue is true, the message will be
	// returned to the queue for redelivery; otherwise, it will be discarded
	// or sent to a dead-letter exchange if configured.
	NackMsg(requeue bool) error

	// Body returns the message payload as a byte slice.
	Body() []byte

	// Header returns the headers associated with the message.
	// Message headers provide metadata about the message and can contain
	// application-specific information set by the message publisher.
	//
	// Headers are a map of key-value pairs where the keys are strings
	// and values can be of various types. Common uses for headers include:
	//   - Message type identification
	//   - Content format specification
	//   - Routing information
	//   - Tracing context propagation
	//   - Custom application metadata
	//
	// For distributed tracing with OpenTelemetry, headers can carry trace
	// context between services, enabling end-to-end tracing across
	// message-based communication.
	Header() map[string]interface{}
}

// ConsumerMessage implements the Message interface and wraps an AMQP delivery.
// This struct provides access to the message content and acknowledgment methods.
type ConsumerMessage struct {
	body     []byte         // The message payload
	delivery *amqp.Delivery // The underlying AMQP delivery object
}

// consumeQueue consumes messages from a specified queue and sends them to a channel.
// This is an internal method that handles the actual consumption logic.
//
// Parameters:
//   - ctx: Context for cancellation control
//   - wg: WaitGroup for coordinating shutdown
//   - queueName: Name of the queue to consume from
//
// Returns a channel that delivers consumed messages. This channel will be closed
// when consumption stops due to context cancellation, shutdown signal, or errors.
//
// The method includes:
//   - Automatic reconnection when the channel is closed
//   - Context-aware cancellation
//   - Graceful shutdown handling
//   - Buffered output channel to improve throughput
func (rb *Rabbit) consumeQueue(ctx context.Context, wg *sync.WaitGroup, queueName string) <-chan Message {
	outChan := make(chan Message, 100)

	wg.Add(1)
	go func() {
		defer wg.Done()
		defer close(outChan)
	outerLoop:
		for {
			select {
			case <-rb.shutdownSignal:
				log.Println("INFO: Stopping consumer due to shutdown signal")
				return
			case <-ctx.Done():
				log.Println("INFO: Stopping consumer due to context cancellation")
				return
			default:
				rb.mu.RLock()
				msgs, err := rb.Channel.Consume(
					queueName,
					"",    // consumer
					false, // autoAck
					false, // exclusive
					false, // noLocal
					false, // noWait
					nil,   // args
				)
				rb.mu.RUnlock()

				if err != nil {
					log.Printf("ERROR: Failed to establish consumer for queue %s: %v", queueName, err)
					time.Sleep(100 * time.Millisecond)
					continue
				}

				for {
					select {
					case <-ctx.Done():
						log.Printf("INFO: Stopping consumer due to context cancellation: %v", ctx.Err())
						return
					case <-rb.shutdownSignal:
						log.Println("INFO: Stopping consumer due to shutdown signal")
						return
					case msg, ok := <-msgs:
						if !ok {
							continue outerLoop
						}
						outChan <- &ConsumerMessage{
							body:     msg.Body,
							delivery: &msg,
						}
					}
				}
			}
		}
	}()
	return outChan
}

// Consume starts consuming messages from the queue specified in the configuration.
// This method provides a channel where consumed messages will be delivered.
//
// Parameters:
//   - ctx: Context for cancellation control
//   - wg: WaitGroup for coordinating shutdown
//
// Returns a channel that delivers Message interfaces for each consumed message.
//
// Example:
//
//	wg := &sync.WaitGroup{}
//	ctx, cancel := context.WithCancel(context.Background())
//	defer cancel()
//
//	msgChan := rabbitClient.Consume(ctx, wg)
//	for msg := range msgChan {
//	    // Process the message
//	    fmt.Println("Received:", string(msg.Body()))
//
//	    // Acknowledge successful processing
//	    if err := msg.AckMsg(); err != nil {
//	        log.Printf("Failed to ack message: %v", err)
//	    }
//	}
func (rb *Rabbit) Consume(ctx context.Context, wg *sync.WaitGroup) <-chan Message {
	return rb.consumeQueue(ctx, wg, rb.cfg.Channel.QueueName)
}

// ConsumeDLQ starts consuming messages from the dead-letter queue.
// This method is useful for processing failed messages sent
// to the dead-letter queue.
//
// Parameters:
//   - ctx: Context for cancellation control
//   - wg: WaitGroup for coordinating shutdown
//
// Returns a channel that delivers Message interfaces for each consumed message
// from the dead-letter queue.
//
// Example:
//
//	wg := &sync.WaitGroup{}
//	ctx, cancel := context.WithCancel(context.Background())
//	defer cancel()
//
//	dlqChan := rabbitClient.ConsumeDLQ(ctx, wg)
//	for msg := range dlqChan {
//	    // Process the failed message
//	    fmt.Println("Failed message:", string(msg.Body()))
//
//	    // Acknowledge after processing
//	    msg.AckMsg()
//	}
func (rb *Rabbit) ConsumeDLQ(ctx context.Context, wg *sync.WaitGroup) <-chan Message {
	return rb.consumeQueue(ctx, wg, "dlq-queue")
}

// Publish sends a message to the RabbitMQ exchange specified in the configuration.
// This method is thread-safe and respects context cancellation.
//
// Parameters:
//   - ctx: Context for cancellation control
//   - msg: Message payload as a byte slice
//   - header: Optional message headers as a map of key-value pairs; can be used for metadata
//     and distributed tracing propagation
//
// The headers parameter is particularly useful for distributed tracing, allowing trace
// context to be propagated across service boundaries through message queues. When using
// with the tracer package, you can extract trace headers and include them in the message:
//
//	traceHeaders := tracerClient.GetCarrier(ctx)
//	err := rabbitClient.Publish(ctx, message, traceHeaders)
//
// Returns an error if publishing fails or if the context is canceled.
//
// Example:
//
//	ctx := context.Background()
//	message := []byte("Hello, RabbitMQ!")
//
//	// Basic publishing without headers
//	err := rabbitClient.Publish(ctx, message, nil)
//	if err != nil {
//	    log.Printf("Failed to publish message: %v", err)
//	} else {
//	    log.Println("Message published successfully")
//	}
//
// Example with distributed tracing:
//
//	// Create a span for the publish operation
//	ctx, span := tracer.StartSpan(ctx, "publish-message")
//	defer span.End()
//
//	// Add relevant attributes to the span
//	span.SetAttributes(map[string]interface{}{
//	    "message.size": len(message),
//	    "routing.key": rabbitClient.Config().Channel.RoutingKey,
//	})
//
//	// Extract trace context to include in the message headers
//	traceHeaders := tracerClient.GetCarrier(ctx)
//
//	// Publish the message with trace headers
//	err := rabbitClient.Publish(ctx, message, traceHeaders)
//	if err != nil {
//	    span.RecordError(err)
//	    log.Printf("Failed to publish message: %v", err)
//	    return err
//	}
//
//	log.Println("Message published successfully with trace context")
func (rb *Rabbit) Publish(ctx context.Context, msg []byte, headers ...map[string]interface{}) error {
	select {
	case <-ctx.Done():
		log.Printf("context error for publishing msg into rabbit: %v", ctx.Err())
		return ctx.Err()
	default:
		// Initialize header variable
		var header map[string]interface{}

		// If headers were provided, use the first one
		if len(headers) > 0 {
			header = headers[0]
		}

		rb.mu.RLock()
		err := rb.Channel.Publish(rb.cfg.Channel.ExchangeName,
			rb.cfg.Channel.RoutingKey,
			false,
			false,
			amqp.Publishing{
				Headers:     header,
				ContentType: rb.cfg.Channel.ContentType,
				//ContentEncoding: "",
				//DeliveryMode:    0,
				//Priority:        0,
				//CorrelationId:   "",
				//ReplyTo:         "",
				//Expiration:      "",
				//MessageId:       "",
				//Timestamp:       time.Time{},
				//Type:            "",
				//UserId:          "",
				//AppId:           "",
				Body: msg,
			},
		)
		rb.mu.RUnlock()

		return err
	}
}

// AckMsg acknowledges the message, informing RabbitMQ that the message
// has been successfully processed and can be removed from the queue.
//
// Returns an error if the acknowledgment fails.
func (rb *ConsumerMessage) AckMsg() error {
	return rb.delivery.Ack(false)
}

// NackMsg rejects the message. If requeue is true, the message will be
// returned to the queue for redelivery; otherwise, it will be discarded
// or sent to a dead-letter exchange if configured.
//
// Parameters:
//   - requeue: Whether to requeue the message for another delivery attempt
//
// Returns an error if the rejection fails.
func (rb *ConsumerMessage) NackMsg(requeue bool) error {
	return rb.delivery.Nack(false, requeue)
}

// Body returns the message payload as a byte slice.
func (rb *ConsumerMessage) Body() []byte {
	return rb.body
}

// Header returns the headers associated with the message.
// Message headers provide metadata about the message and can contain
// application-specific information set by the message publisher.
//
// Headers are a map of key-value pairs where the keys are strings
// and values can be of various types. Common uses for headers include:
//   - Message type identification
//   - Content format specification
//   - Routing information
//   - Tracing context propagation
//   - Custom application metadata
//
// Returns:
//   - map[string]interface{}: A map containing the message headers
//
// Example:
//
//	msgChan := rabbitClient.Consume(ctx, wg)
//	for msg := range msgChan {
//	    // Access message headers
//	    headers := msg.Header()
//
//	    // Check for specific headers
//	    if contentType, ok := headers["content-type"].(string); ok {
//	        fmt.Printf("Content type: %s\n", contentType)
//	    }
//
//	    // Access trace context from headers for distributed tracing
//	    if traceID, ok := headers["trace-id"].(string); ok {
//	        ctx = tracer.SetTraceID(ctx, traceID)
//	    }
//
//	    // Process the message...
//	}
func (rb *ConsumerMessage) Header() map[string]interface{} {
	return rb.delivery.Headers
}
