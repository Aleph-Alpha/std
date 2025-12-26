package kafka

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"sync"

	"github.com/Aleph-Alpha/std/v1/observability"
	"github.com/segmentio/kafka-go"
	"github.com/segmentio/kafka-go/compress"
	"github.com/segmentio/kafka-go/sasl"
	"github.com/segmentio/kafka-go/sasl/plain"
	"github.com/segmentio/kafka-go/sasl/scram"
)

// KafkaClient represents a client for interacting with Apache Kafka.
// It manages connections and provides methods for publishing
// and consuming messages.
//
// KafkaClient implements the Client interface.
type KafkaClient struct {
	// cfg stores the configuration for this Kafka client
	cfg Config

	// observer provides optional observability hooks for tracking operations
	observer observability.Observer

	// logger provides optional logging for lifecycle and background operations
	logger Logger

	// writer is the Kafka writer used for publishing messages
	writer *kafka.Writer

	// reader is the Kafka reader used for consuming messages
	reader *kafka.Reader

	// serializer is used to encode messages before publishing
	serializer Serializer

	// deserializer is used to decode messages after consuming
	deserializer Deserializer

	// mu protects concurrent access to writer and reader
	mu sync.RWMutex

	// shutdownSignal is closed when the client is being shut down
	shutdownSignal chan struct{}

	closeShutdownOnce sync.Once
}

// NewClient creates and initializes a new KafkaClient with the provided configuration.
// This function sets up the producer and/or consumer based on the configuration.
//
// Parameters:
//   - cfg: Configuration for connecting to Kafka
//
// Returns a new KafkaClient instance that is ready to use.
//
// Example:
//
//	client, err := kafka.NewClient(config)
//	if err != nil {
//		log.Printf("ERROR: failed to create Kafka client: %v", err)
//		return nil, err
//	}
//	defer client.GracefulShutdown()
func NewClient(cfg Config) (*KafkaClient, error) {
	// Apply defaults
	if cfg.MinBytes == 0 {
		cfg.MinBytes = DefaultMinBytes
	}
	if cfg.MaxBytes == 0 {
		cfg.MaxBytes = DefaultMaxBytes
	}
	if cfg.MaxWait == 0 {
		cfg.MaxWait = DefaultMaxWait
	}
	if cfg.CommitInterval == 0 {
		cfg.CommitInterval = DefaultCommitInterval
	}
	if cfg.StartOffset == 0 {
		cfg.StartOffset = DefaultStartOffset
	}
	if cfg.Partition == 0 {
		cfg.Partition = DefaultPartition
	}
	if cfg.RequiredAcks == 0 {
		cfg.RequiredAcks = DefaultRequiredAcks
	}
	if cfg.BatchSize == 0 {
		cfg.BatchSize = DefaultBatchSize
	}
	if cfg.BatchTimeout == 0 {
		cfg.BatchTimeout = DefaultBatchTimeout
	}
	if cfg.MaxAttempts == 0 {
		cfg.MaxAttempts = DefaultMaxAttempts
	}
	if cfg.WriteTimeout == 0 {
		cfg.WriteTimeout = DefaultWriteTimeout
	}
	// EnableAutoOffsetStore defaults to true if not explicitly set
	// This is handled in createReader

	k := &KafkaClient{
		cfg:            cfg,
		observer:       nil, // No observer by default
		logger:         nil, // No logger by default
		shutdownSignal: make(chan struct{}),
	}

	// Set up TLS config if enabled
	var tlsConfig *tls.Config
	var err error
	if cfg.TLS.Enabled {
		tlsConfig, err = createTLSConfig(cfg.TLS)
		if err != nil {
			return nil, fmt.Errorf("failed to create TLS config: %w", err)
		}
	}

	// Set up SASL mechanism if enabled
	var mechanism sasl.Mechanism
	if cfg.SASL.Enabled {
		mechanism, err = createSASLMechanism(cfg.SASL)
		if err != nil {
			return nil, fmt.Errorf("failed to create SASL mechanism: %w", err)
		}
	}

	// Create writer (producer)
	if !cfg.IsConsumer {
		k.writer = createWriter(cfg, tlsConfig, mechanism, k)
	}

	// Create reader (consumer)
	if cfg.IsConsumer {
		k.reader = createReader(cfg, tlsConfig, mechanism, k)
	}

	// Set default serializers based on DataType if not already set
	k.SetDefaultSerializers()

	return k, nil
}

// WithObserver attaches an observer to the Kafka client for tracking operations.
// This method uses the builder pattern and returns the client for method chaining.
//
// The observer will be notified of all produce and consume operations, allowing
// external code to collect metrics, create traces, or log operations.
//
// This is useful for non-FX usage where you want to enable observability after
// creating the client. When using FX, use NewClientWithDI instead, which
// automatically injects the observer.
//
// Example:
//
//	client, err := kafka.NewClient(config)
//	if err != nil {
//	    return err
//	}
//	client = client.WithObserver(myObserver)
//	defer client.GracefulShutdown()
func (k *KafkaClient) WithObserver(observer observability.Observer) *KafkaClient {
	k.observer = observer
	return k
}

// WithLogger attaches a logger to the Kafka client for internal logging.
// This method uses the builder pattern and returns the client for method chaining.
//
// The logger will be used for lifecycle events, background worker logs, and cleanup errors.
// This is particularly useful for debugging and monitoring consumer worker behavior.
//
// This is useful for non-FX usage where you want to enable logging after
// creating the client. When using FX, the logger is automatically injected via NewClientWithDI.
//
// Example:
//
//	client, err := kafka.NewClient(config)
//	if err != nil {
//	    return err
//	}
//	client = client.WithLogger(myLogger)
//	defer client.GracefulShutdown()
func (k *KafkaClient) WithLogger(logger Logger) *KafkaClient {
	k.logger = logger
	return k
}

// WithSerializer attaches a serializer to the Kafka client for encoding messages.
// This method uses the builder pattern and returns the client for method chaining.
//
// The serializer will be used to encode messages before publishing to Kafka.
// If not set, you can only publish []byte data directly.
//
// This is useful for non-FX usage where you want to set serializers after
// creating the client. When using FX, serializers can be injected via NewClientWithDI.
//
// Example:
//
//	client, err := kafka.NewClient(config)
//	if err != nil {
//	    return err
//	}
//	client = client.WithSerializer(&kafka.JSONSerializer{})
//	defer client.GracefulShutdown()
func (k *KafkaClient) WithSerializer(serializer Serializer) *KafkaClient {
	k.SetSerializer(serializer)
	return k
}

// WithDeserializer attaches a deserializer to the Kafka client for decoding messages.
// This method uses the builder pattern and returns the client for method chaining.
//
// The deserializer will be used to decode messages when consuming from Kafka.
// If not set, msg.BodyAs() will use JSONDeserializer as a fallback.
//
// This is useful for non-FX usage where you want to set deserializers after
// creating the client. When using FX, deserializers can be injected via NewClientWithDI.
//
// Example:
//
//	client, err := kafka.NewClient(config)
//	if err != nil {
//	    return err
//	}
//	client = client.WithDeserializer(&kafka.JSONDeserializer{})
//	defer client.GracefulShutdown()
func (k *KafkaClient) WithDeserializer(deserializer Deserializer) *KafkaClient {
	k.SetDeserializer(deserializer)
	return k
}

// logInfo logs an informational message using the configured logger if available.
// This is used for lifecycle and background operation logging.
func (k *KafkaClient) logInfo(ctx context.Context, msg string, fields map[string]interface{}) {
	if k.logger != nil {
		k.logger.InfoWithContext(ctx, msg, nil, fields)
	}
	// Silently skip if no logger configured
}

// logWarn logs a warning message using the configured logger if available.
// This is used for non-critical issues during shutdown or background operations.
func (k *KafkaClient) logWarn(ctx context.Context, msg string, fields map[string]interface{}) {
	if k.logger != nil {
		k.logger.WarnWithContext(ctx, msg, nil, fields)
	}
	// Silently skip if no logger configured
}

// logError logs an error message using the configured logger if available.
// This is only used for errors in background goroutines that can't be returned to the caller.
func (k *KafkaClient) logError(ctx context.Context, msg string, fields map[string]interface{}) {
	if k.logger != nil {
		k.logger.ErrorWithContext(ctx, msg, nil, fields)
	}
	// Silently skip if no logger configured
}

// SetSerializer sets the serializer for the Kafka client.
// This is typically called by the FX module during initialization.
func (k *KafkaClient) SetSerializer(s Serializer) {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.serializer = s
}

// SetDeserializer sets the deserializer for the Kafka client.
// This is typically called by the FX module during initialization.
func (k *KafkaClient) SetDeserializer(d Deserializer) {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.deserializer = d
}

// createErrorLogger creates a Kafka error logger from the client's logger
func createErrorLogger(client *KafkaClient) kafka.LoggerFunc {
	// Use our optional logger if available
	if client.logger != nil {
		return kafka.LoggerFunc(func(msg string, args ...interface{}) {
			formattedMsg := msg
			if len(args) > 0 {
				formattedMsg = fmt.Sprintf(msg, args...)
			}
			client.logger.ErrorWithContext(context.Background(), "Kafka internal error", nil, map[string]interface{}{
				"error": formattedMsg,
			})
		})
	}

	// If no logger, silently ignore (don't use log package)
	// Kafka library errors will be returned to caller instead
	return kafka.LoggerFunc(func(msg string, args ...interface{}) {
		// No-op: silently ignore kafka internal errors if no logger configured
	})
}

// createWriter creates a Kafka writer with the given configuration
func createWriter(cfg Config, tlsConfig *tls.Config, mechanism sasl.Mechanism, client *KafkaClient) *kafka.Writer {
	writerConfig := kafka.WriterConfig{
		Brokers:      cfg.Brokers,
		Topic:        cfg.Topic,
		Balancer:     &kafka.LeastBytes{},
		MaxAttempts:  cfg.MaxAttempts,
		WriteTimeout: cfg.WriteTimeout,
		ErrorLogger:  createErrorLogger(client),
	}

	// Set required acks
	writerConfig.RequiredAcks = cfg.RequiredAcks

	// Set async mode
	if cfg.Async {
		writerConfig.Async = true
		writerConfig.BatchSize = cfg.BatchSize
		writerConfig.BatchTimeout = cfg.BatchTimeout
	}

	// Set compression
	switch cfg.CompressionCodec {
	case "gzip":
		writerConfig.CompressionCodec = &compress.GzipCodec
	case "snappy":
		writerConfig.CompressionCodec = &compress.SnappyCodec
	case "lz4":
		writerConfig.CompressionCodec = &compress.Lz4Codec
	case "zstd":
		writerConfig.CompressionCodec = &compress.ZstdCodec
	}

	// Create dialer with TLS and SASL
	dialer := &kafka.Dialer{
		TLS:           tlsConfig,
		SASLMechanism: mechanism,
	}
	writerConfig.Dialer = dialer

	return kafka.NewWriter(writerConfig)
}

// createReader creates a Kafka reader with the given configuration
func createReader(cfg Config, tlsConfig *tls.Config, mechanism sasl.Mechanism, client *KafkaClient) *kafka.Reader {
	readerConfig := kafka.ReaderConfig{
		Brokers:     cfg.Brokers,
		Topic:       cfg.Topic,
		GroupID:     cfg.GroupID,
		MinBytes:    cfg.MinBytes,
		MaxBytes:    cfg.MaxBytes,
		MaxWait:     cfg.MaxWait,
		StartOffset: cfg.StartOffset,
		ErrorLogger: createErrorLogger(client),
	}

	// Configure auto-commit behavior
	if cfg.EnableAutoCommit {
		// Auto-commit enabled: set CommitInterval
		readerConfig.CommitInterval = cfg.CommitInterval
	} else {
		// Auto-commit disabled: set CommitInterval to 0
		readerConfig.CommitInterval = 0
	}

	if cfg.Partition != -1 {
		readerConfig.Partition = cfg.Partition
	}

	// Create dialer with TLS and SASL
	dialer := &kafka.Dialer{
		TLS:           tlsConfig,
		SASLMechanism: mechanism,
	}
	readerConfig.Dialer = dialer

	return kafka.NewReader(readerConfig)
}

// createTLSConfig creates a TLS configuration from the provided config
func createTLSConfig(cfg TLSConfig) (*tls.Config, error) {
	tlsConfig := &tls.Config{
		InsecureSkipVerify: cfg.InsecureSkipVerify,
	}

	// Load CA certificate
	if cfg.CACertPath != "" {
		caCert, err := os.ReadFile(cfg.CACertPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read CA cert: %w", err)
		}
		caCertPool := x509.NewCertPool()
		if !caCertPool.AppendCertsFromPEM(caCert) {
			return nil, fmt.Errorf("failed to parse CA cert")
		}
		tlsConfig.RootCAs = caCertPool
	}

	// Load client certificate
	if cfg.ClientCertPath != "" && cfg.ClientKeyPath != "" {
		cert, err := tls.LoadX509KeyPair(cfg.ClientCertPath, cfg.ClientKeyPath)
		if err != nil {
			return nil, fmt.Errorf("failed to load client cert: %w", err)
		}
		tlsConfig.Certificates = []tls.Certificate{cert}
	}

	return tlsConfig, nil
}

// createSASLMechanism creates a SASL mechanism from the provided config
func createSASLMechanism(cfg SASLConfig) (sasl.Mechanism, error) {
	switch cfg.Mechanism {
	case "PLAIN":
		return plain.Mechanism{
			Username: cfg.Username,
			Password: cfg.Password,
		}, nil
	case "SCRAM-SHA-256":
		return scram.Mechanism(scram.SHA256, cfg.Username, cfg.Password)
	case "SCRAM-SHA-512":
		return scram.Mechanism(scram.SHA512, cfg.Username, cfg.Password)
	default:
		return nil, fmt.Errorf("unsupported SASL mechanism: %s", cfg.Mechanism)
	}
}
