package rabbit

import (
	"context"
	"fmt"
	amqp "github.com/rabbitmq/amqp091-go"
	"sync"
	"time"
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
				rb.logger.Info("consumer is shutting down due to shutdown signal", nil, nil)
				return
			case <-ctx.Done():
				rb.logger.Info("consumer is shutting down due to context cancellation", ctx.Err(), nil)
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
					rb.logger.Error("error in establishing consumer for rabbit", err, map[string]interface{}{
						"queue_name": queueName,
					})
					time.Sleep(100 * time.Millisecond)
					continue
				}

				for {
					select {
					case <-ctx.Done():
						rb.logger.Info("consumer is shutting down due to context cancellation", ctx.Err(), nil)
						return
					case <-rb.shutdownSignal:
						rb.logger.Info("consumer is shutting down due to shutdown signal", nil, nil)
						return
					case msg, ok := <-msgs:
						if !ok {
							continue outerLoop
						}
						rb.logger.Debug("message consumed from rabbit", nil, map[string]interface{}{
							"queue_name": queueName,
							"payload":    fmt.Sprintf("%v", string(msg.Body)),
						})
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
//
// Returns an error if publishing fails or if the context is canceled.
//
// Example:
//
//	ctx := context.Background()
//	message := []byte("Hello, RabbitMQ!")
//
//	err := rabbitClient.Publish(ctx, message)
//	if err != nil {
//	    log.Printf("Failed to publish message: %v", err)
//	} else {
//	    log.Println("Message published successfully")
//	}
func (rb *Rabbit) Publish(ctx context.Context, msg []byte) error {

	select {
	case <-ctx.Done():
		rb.logger.Error("context error for publishing msg into rabbit", ctx.Err(), nil)
		return ctx.Err()
	default:
		rb.mu.RLock()
		err := rb.Channel.Publish(rb.cfg.Channel.ExchangeName,
			rb.cfg.Channel.RoutingKey,
			false,
			false,
			amqp.Publishing{
				//Headers:         nil,
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

		if err == nil {
			return nil
		}
		rb.logger.Error("error in publishing msg into rabbit", err, nil)
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
