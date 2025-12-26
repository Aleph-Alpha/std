package rabbit

import "context"

// Config defines the top-level configuration structure for the RabbitMQ client.
// It contains all the necessary configuration sections for establishing connections,
// setting up channels, and configuring dead-letter behavior.
type Config struct {
	// Connection contains the settings needed to establish a connection to the RabbitMQ server
	Connection Connection

	// Channel contains configuration for exchanges, queues, and message routing
	Channel Channel

	// DeadLetter contains configuration for the dead-letter exchange and queue
	// used for handling failed messages
	DeadLetter DeadLetter
}

// Connection contains the configuration parameters needed to establish
// a connection to a RabbitMQ server, including authentication and TLS settings.
type Connection struct {
	// Host is the RabbitMQ server hostname or IP address
	Host string

	// Port is the RabbitMQ server port (typically 5672 for non-SSL, 5671 for SSL)
	Port uint

	// User is the RabbitMQ username for authentication
	User string

	// Password is the RabbitMQ password for authentication
	Password string

	// IsSSLEnabled determines whether to use SSL/TLS for the connection
	// When true, connections will use the AMQPs protocol
	IsSSLEnabled bool

	// UseCert determines whether to use client certificate authentication
	// When true, client certificates will be sent for mutual TLS authentication
	UseCert bool

	// CACertPath is the file path to the CA certificate for verifying the server
	// Used when IsSSLEnabled is true
	CACertPath string

	// ClientCertPath is the file path to the client certificate
	// Used when both IsSSLEnabled and UseCert are true
	ClientCertPath string

	// ClientKeyPath is the file path to the client certificate's private key
	// Used when both IsSSLEnabled and UseCert are true
	ClientKeyPath string

	// ServerName is the server name to use for TLS verification
	// This should match a CN or SAN in the server's certificate
	ServerName string
}

// Channel contains configuration for AMQP channels, exchanges, queues, and bindings.
// These settings determine how messages are routed and processed.
type Channel struct {
	// ExchangeName is the name of the exchange to publish to or consume from
	ExchangeName string

	// ExchangeType defines the routing behavior of the exchange
	// Common values: "direct", "fanout", "topic", "headers"
	ExchangeType string

	// RoutingKey is used for routing messages from exchanges to queues
	// The meaning depends on the exchange type:
	// - For direct exchanges: exact matching key
	// - For topic exchanges: routing pattern with wildcards
	// - For fanout exchanges: ignored
	RoutingKey string

	// QueueName is the name of the queue to declare or consume from
	QueueName string

	// DelayToReconnect is the time in milliseconds to wait between reconnection attempts
	DelayToReconnect int

	// PrefetchCount limits the number of unacknowledged messages that can be sent to a consumer
	// A value of 0 means no limit (not recommended for production)
	PrefetchCount int

	// IsConsumer determines whether this client will declare exchanges and queues
	// Set to true for consumers, false for publishers that use existing exchanges
	IsConsumer bool

	// ContentType specifies the MIME type of published messages
	// Common values: "application/json", "text/plain", "application/octet-stream"
	ContentType string
}

// DeadLetter contains configuration for dead-letter handling.
// Dead-letter exchanges receive messages that are rejected, expire, or exceed queue limits.
// This provides a mechanism for handling failed message processing.
type DeadLetter struct {
	// ExchangeName is the name of the dead-letter exchange
	ExchangeName string

	// QueueName is the name of the queue bound to the dead-letter exchange
	QueueName string

	// RoutingKey is the routing key used when dead-lettering messages
	// This can be different from the original routing key
	RoutingKey string

	// Ttl is the time-to-live for messages in seconds
	// Messages that remain in the queue longer than this TTL will be dead-lettered
	// A value of 0 means no TTL (messages never expire)
	Ttl int
}

// Logger is an interface that matches the std/v1/logger.Logger interface.
// It provides context-aware structured logging with optional error and field parameters.
type Logger interface {
	// InfoWithContext logs an informational message with trace context.
	InfoWithContext(ctx context.Context, msg string, err error, fields ...map[string]interface{})

	// WarnWithContext logs a warning message with trace context.
	WarnWithContext(ctx context.Context, msg string, err error, fields ...map[string]interface{})

	// ErrorWithContext logs an error message with trace context.
	ErrorWithContext(ctx context.Context, msg string, err error, fields ...map[string]interface{})
}
