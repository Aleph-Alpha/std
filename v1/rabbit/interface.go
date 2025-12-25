package rabbit

import (
	"context"
	"sync"

	amqp "github.com/rabbitmq/amqp091-go"
)

// Client provides a high-level interface for interacting with RabbitMQ.
// It abstracts connection management, channel operations, and message publishing/consuming.
//
// This interface is implemented by the concrete *RabbitClient type.
type Client interface {
	// Publisher operations

	// Publish sends a message to RabbitMQ with optional headers.
	// The message is sent using the configured exchange and routing key.
	Publish(ctx context.Context, msg []byte, headers ...map[string]interface{}) error

	// Consumer operations

	// Consume starts consuming messages from the main queue.
	// Returns a channel that delivers consumed messages.
	Consume(ctx context.Context, wg *sync.WaitGroup) <-chan Message

	// ConsumeDLQ starts consuming messages from the dead letter queue (DLQ).
	// This allows processing of messages that failed in the main queue.
	ConsumeDLQ(ctx context.Context, wg *sync.WaitGroup) <-chan Message

	// Connection management

	// RetryConnection monitors the connection and automatically reconnects on failure.
	// This method should be run in a goroutine.
	RetryConnection(cfg Config)

	// Lifecycle

	// GracefulShutdown closes all RabbitMQ connections and channels cleanly.
	GracefulShutdown()

	// GetChannel returns the underlying AMQP channel for direct operations when needed.
	GetChannel() *amqp.Channel
}

// Message represents a consumed message from RabbitMQ.
// It provides methods for acknowledging, rejecting, and accessing message data.
type Message interface {
	// AckMsg acknowledges the message, removing it from the queue.
	AckMsg() error

	// NackMsg negatively acknowledges the message.
	// If requeue is true, the message is requeued; otherwise it goes to DLQ.
	NackMsg(requeue bool) error

	// Body returns the message payload as a byte slice.
	Body() []byte

	// Header returns the message headers.
	Header() map[string]interface{}
}
