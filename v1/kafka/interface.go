package kafka

import (
	"context"
	"sync"
)

// Client provides a high-level interface for interacting with Apache Kafka.
// It abstracts producer and consumer operations with a simplified API.
//
// This interface is implemented by the concrete *KafkaClient type.
type Client interface {
	// Producer operations

	// Publish sends a single message to Kafka with the specified key and body.
	// The body is serialized using the configured serializer.
	// Optional headers can be provided as the last parameter.
	Publish(ctx context.Context, key string, data interface{}, headers ...map[string]interface{}) error

	// Consumer operations

	// Consume starts consuming messages from Kafka with a single worker.
	// Returns a channel that delivers consumed messages.
	Consume(ctx context.Context, wg *sync.WaitGroup) <-chan Message

	// ConsumeParallel starts consuming messages with multiple concurrent workers.
	// Provides better throughput for high-volume topics.
	ConsumeParallel(ctx context.Context, wg *sync.WaitGroup, numWorkers int) <-chan Message

	// Serialization

	// Deserialize converts a Message to an object using the configured deserializer.
	Deserialize(msg Message, target interface{}) error

	// SetSerializer sets the serializer for outgoing messages.
	SetSerializer(s Serializer)

	// SetDeserializer sets the deserializer for incoming messages.
	SetDeserializer(d Deserializer)

	// SetDefaultSerializers configures default serializers based on the DataType config.
	SetDefaultSerializers()

	// Error handling

	// TranslateError translates Kafka errors to more user-friendly messages.
	TranslateError(err error) error

	// IsRetryableError checks if an error can be retried.
	IsRetryableError(err error) bool

	// IsTemporaryError checks if an error is temporary.
	IsTemporaryError(err error) bool

	// IsPermanentError checks if an error is permanent.
	IsPermanentError(err error) bool

	// IsAuthenticationError checks if an error is authentication-related.
	IsAuthenticationError(err error) bool

	// Lifecycle

	// GracefulShutdown closes all Kafka connections cleanly.
	GracefulShutdown()
}

// Message defines the interface for consumed messages from Kafka.
// This interface abstracts the underlying Kafka message structure and
// provides methods for committing messages.
type Message interface {
	// CommitMsg commits the message, informing Kafka that the message
	// has been successfully processed.
	CommitMsg() error

	// Body returns the message payload as a byte slice.
	Body() []byte

	// BodyAs deserializes the message body into the target structure.
	// It uses the configured Deserializer, or falls back to JSONDeserializer.
	// This is a convenience method equivalent to calling Deserialize(msg, target).
	BodyAs(target interface{}) error

	// Key returns the message key as a string.
	Key() string

	// Header returns the headers associated with the message.
	Header() map[string]interface{}

	// Partition returns the partition this message came from.
	Partition() int

	// Offset returns the offset of this message.
	Offset() int64
}
