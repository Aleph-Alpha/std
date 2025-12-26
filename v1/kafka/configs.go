package kafka

import (
	"context"
	"time"
)

// Config defines the top-level configuration structure for the Kafka client.
// It contains all the necessary configuration sections for establishing connections,
// setting up producers and consumers.
type Config struct {
	// Brokers is a list of Kafka broker addresses
	Brokers []string

	// Topic is the Kafka topic to publish to or consume from
	Topic string

	// GroupID is the consumer group ID for coordinated consumption
	// Only used when IsConsumer is true
	GroupID string

	// IsConsumer determines whether this client will act as a consumer
	// Set to true for consumers, false for publishers
	IsConsumer bool

	// MinBytes is the minimum number of bytes to fetch in a single request
	// Default: 1 byte
	MinBytes int

	// MaxBytes is the maximum number of bytes to fetch in a single request
	// Default: 10MB
	MaxBytes int

	// MaxWait is the maximum amount of time to wait for MinBytes to become available
	// Default: 10s
	MaxWait time.Duration

	// CommitInterval is how often to commit offsets automatically
	// Only used when EnableAutoCommit is true
	// Default: 1s
	CommitInterval time.Duration

	// EnableAutoCommit determines whether offsets are committed automatically
	// When true, offsets are committed at CommitInterval
	// When false, you must call msg.CommitMsg() manually
	// Default: false (manual commit for safety)
	EnableAutoCommit bool

	// EnableAutoOffsetStore determines whether to automatically store offsets
	// When true with EnableAutoCommit false, offsets are stored but not committed
	// This allows you to control commit timing while still tracking progress
	// Default: true
	EnableAutoOffsetStore bool

	// StartOffset determines where to start consuming from when there's no committed offset
	// Options: FirstOffset (-2), LastOffset (-1)
	// Default: FirstOffset
	StartOffset int64

	// Partition is the partition to produce to or consume from
	// Set to -1 for automatic partition assignment
	// Default: -1
	Partition int

	// RequiredAcks determines how many replica acknowledgments to wait for
	// Options:
	//   RequireNone (0): Don't wait for acknowledgment (fire-and-forget, fastest but least safe)
	//   RequireOne (1): Wait for leader only (balance of speed and durability)
	//   RequireAll (-1): Wait for all in-sync replicas (slowest but most durable)
	// Default: RequireAll (-1)
	RequiredAcks int

	// WriteTimeout is the timeout for write operations
	// If RequiredAcks > 0, this is how long to wait for acknowledgment
	// Default: 10s
	WriteTimeout time.Duration

	// Async enables async write mode for producer
	// When true, writes are batched and sent asynchronously
	// Default: false
	Async bool

	// BatchSize is the maximum number of messages to batch together
	// Only used when Async is true
	// Default: 100
	BatchSize int

	// BatchTimeout is the maximum time to wait before sending a batch
	// Only used when Async is true
	// Default: 1s
	BatchTimeout time.Duration

	// CompressionCodec specifies the compression algorithm to use
	// Options: nil (no compression), gzip, snappy, lz4, zstd
	// Default: nil
	CompressionCodec string

	// MaxAttempts is the maximum number of attempts to deliver a message
	// Default: 10
	MaxAttempts int

	// AllowAutoTopicCreation determines whether to allow automatic topic creation
	// Default: false
	AllowAutoTopicCreation bool

	// TLS contains TLS/SSL configuration
	TLS TLSConfig

	// SASL contains SASL authentication configuration
	SASL SASLConfig

	// DataType specifies the default data type for automatic serializer selection
	// When no explicit serializer is provided, the client will use a default serializer
	// based on this type.
	// Options: "json" (default), "string", "protobuf", "avro", "gob", "bytes"
	// Default: "json"
	DataType string
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

// TLSConfig contains TLS/SSL configuration parameters.
type TLSConfig struct {
	// Enabled determines whether to use TLS/SSL for the connection
	Enabled bool

	// CACertPath is the file path to the CA certificate for verifying the broker
	CACertPath string

	// ClientCertPath is the file path to the client certificate
	ClientCertPath string

	// ClientKeyPath is the file path to the client certificate's private key
	ClientKeyPath string

	// InsecureSkipVerify controls whether to skip verification of the server's certificate
	// WARNING: Setting this to true is insecure and should only be used in testing
	InsecureSkipVerify bool
}

// SASLConfig contains SASL authentication configuration parameters.
type SASLConfig struct {
	// Enabled determines whether to use SASL authentication
	Enabled bool

	// Mechanism specifies the SASL mechanism to use
	// Options: "PLAIN", "SCRAM-SHA-256", "SCRAM-SHA-512"
	Mechanism string

	// Username is the SASL username
	Username string

	// Password is the SASL password
	Password string
}

// Default values for configuration
const (
	DefaultMinBytes         = 1
	DefaultMaxBytes         = 10e6 // 10MB
	DefaultMaxWait          = 10 * time.Second
	DefaultCommitInterval   = 1 * time.Second
	DefaultStartOffset      = -2 // FirstOffset
	DefaultPartition        = -1 // Automatic partition assignment
	DefaultRequiredAcks     = -1 // WaitForAll
	DefaultBatchSize        = 100
	DefaultBatchTimeout     = 1 * time.Second
	DefaultRebalanceTimeout = 30 * time.Second
	DefaultMaxAttempts      = 10
	DefaultWriteTimeout     = 10 * time.Second

	// Producer acknowledgment modes
	RequireNone = 0  // Fire-and-forget (no acknowledgment)
	RequireOne  = 1  // Wait for leader only
	RequireAll  = -1 // Wait for all in-sync replicas (most durable)

	// Consumer offset modes
	FirstOffset = -2 // Start from the beginning
	LastOffset  = -1 // Start from the end
)
