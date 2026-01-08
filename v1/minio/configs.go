package minio

import (
	"context"
	"time"
)

// Constants for MinIO client configuration and behavior.
const (
	// unknownSize represents an unknown object size used in upload operations.
	// When size is unknown, MinIO will use chunked transfer encoding.
	unknownSize int64 = -1

	// connectionHealthCheckInterval defines how often the client checks connection health.
	// The client will attempt to reconnect if a check fails.
	connectionHealthCheckInterval = 3 * time.Second

	// minPartSizeForUpload is the minimum part size (5 MiB) allowed for multipart uploads.
	// This is a limit imposed by the S3 specification.
	minPartSizeForUpload int64 = 5 * 1024 * 1024

	// minioLimitPartCount is the maximum number of parts (10,000) allowed in a multipart upload.
	// This is a limit imposed by the S3 specification.
	minioLimitPartCount int64 = 10000

	// MultipartThreshold is the default size threshold (50 MiB) above which uploads
	// will automatically use multipart uploading for better performance and reliability.
	MultipartThreshold int64 = 50 * 1024 * 1024

	// MaxObjectSize is the maximum size (5 TiB) for a single object in MinIO/S3.
	// This is a limit imposed by the S3 specification.
	MaxObjectSize int64 = 5 * 1024 * 1024 * 1024 * 1024

	// UploadDefaultContentType is the default MIME type to use for uploaded objects.
	UploadDefaultContentType string = "application/octet-stream"

	MultipartDownloadDefaultPartSize int64 = 5 * 1024 * 1024
)

// Config defines the top-level configuration for MinIO.
// This structure contains all configuration options for the MinIO client,
// organized into logical sections for different aspects of functionality.
type Config struct {
	// Connection contains basic connection parameters for the MinIO server
	Connection ConnectionConfig

	// UploadConfig defines parameters related to uploading objects
	UploadConfig UploadConfig

	// DownloadConfig defines parameters related to downloading objects
	DownloadConfig DownloadConfig

	// PresignedConfig contains settings for generating presigned URLs
	PresignedConfig PresignedConfig

	// Notification defines event notification configuration
	Notification NotificationConfig
}

// ConnectionConfig contains MinIO server connection details.
// These parameters are required to establish a connection to a MinIO server.
type ConnectionConfig struct {
	// Endpoint is the MinIO server address (e.g., "minio.example.com:9000")
	Endpoint string

	// AccessKeyID is the MinIO access key (similar to a username)
	AccessKeyID string

	// SecretAccessKey is the MinIO secret key (similar to a password)
	SecretAccessKey string

	// UseSSL determines whether to use HTTPS (true) or HTTP (false)
	UseSSL bool

	// Region specifies the S3 region (e.g., "us-east-1")
	Region string
}

// UploadConfig defines the configuration for upload constraints.
// These parameters control how objects are uploaded, particularly for large objects.
type UploadConfig struct {
	// MaxObjectSize is the maximum size in bytes allowed for a single object
	// Default: 5 TiB (S3 specification limit)
	MaxObjectSize int64

	// MinPartSize is the minimum size in bytes for each part in a multipart upload
	// Default: 5 MiB (S3 specification minimum)
	MinPartSize uint64

	// MultipartThreshold is the size in bytes above which an upload automatically
	// switches to multipart mode for better performance
	// Default: 50 MiB
	MultipartThreshold int64

	// DefaultContentType is the default MIME type to use for uploaded objects
	DefaultContentType string
}

// DownloadConfig defines parameters that control download behavior.
// These settings optimize memory usage when downloading objects of different sizes.
type DownloadConfig struct {
	// SmallFileThreshold is the size in bytes below which a file is considered "small"
	// Small files use a simpler, direct download approach
	SmallFileThreshold int64

	// InitialBufferSize is the starting buffer size in bytes for downloading large files
	// This affects memory usage when downloading large objects
	InitialBufferSize int

	// DefaultMultipartDownload is the default size of parts to download at each part
	DefaultMultipartDownload int
}

// PresignedConfig contains configuration options for presigned URLs.
// Presigned URLs allow temporary access to objects without requiring AWS credentials.
type PresignedConfig struct {
	// Enabled determines whether presigned URL functionality is available
	Enabled bool

	// ExpiryDuration sets how long presigned URLs remain valid
	// Example: 15 * time.Minute for 15-minute expiration
	ExpiryDuration time.Duration

	// BaseURL allows overriding the host/domain in generated presigned URLs
	// Useful when using a CDN or custom domain in front of MinIO
	// Example: "https://cdn.example.com"
	BaseURL string
}

// NotificationConfig defines the configuration for event notifications.
// MinIO can send notifications when events occur on buckets (e.g., object created).
type NotificationConfig struct {
	// Enabled determines whether event notifications are active
	Enabled bool

	// Webhook contains configuration for webhook notification targets
	Webhook []WebhookNotification

	// AMQP contains configuration for AMQP (RabbitMQ, etc.) notification targets
	AMQP []AMQPNotification

	// Redis contains configuration for Redis notification targets
	Redis []RedisNotification

	// Kafka contains configuration for Kafka notification targets
	Kafka []KafkaNotification

	// MQTT contains configuration for MQTT notification targets
	MQTT []MQTTNotification
}

// BaseNotification contains common properties for all notification types.
// This is embedded in specific notification target types.
type BaseNotification struct {
	// Events is a list of event types to subscribe to
	// Examples: "s3:ObjectCreated:*", "s3:ObjectRemoved:*"
	Events []string

	// Prefix filters notifications to only objects with this key prefix
	// Example: "uploads/" to only get events for objects in the uploads directory
	Prefix string

	// Suffix filters notifications to only objects with this key suffix
	// Example: ".jpg" to only get events for JPEG images
	Suffix string

	// Name is a unique identifier for this notification configuration
	Name string
}

// WebhookNotification defines a webhook notification target.
// Webhook notifications send HTTP POST requests to a specified endpoint.
type WebhookNotification struct {
	// Embed the common notification properties
	BaseNotification

	// Endpoint is the URL where webhook notifications will be sent
	Endpoint string

	// AuthToken is an optional token for authenticating with the webhook endpoint
	AuthToken string
}

// AMQPNotification defines an AMQP notification target.
// AMQP notifications can be used with systems like RabbitMQ.
type AMQPNotification struct {
	// Embed the common notification properties
	BaseNotification

	// URL is the AMQP server connection URL
	URL string

	// Exchange is the name of the AMQP exchange
	Exchange string

	// RoutingKey defines how messages are routed to queues
	RoutingKey string

	// ExchangeType defines the AMQP exchange type (e.g., "fanout", "direct")
	ExchangeType string

	// Mandatory flag requires messages to be routable
	Mandatory bool

	// Immediate flag requires immediate delivery
	Immediate bool

	// Durable flag makes the exchange survive broker restarts
	Durable bool

	// Internal flag makes the exchange only accessible to other exchanges
	Internal bool

	// NoWait flag makes declare operations non-blocking
	NoWait bool

	// AutoDeleted flag automatically deletes the exchange when no longer used
	AutoDeleted bool
}

// RedisNotification defines a Redis notification target.
// Redis notifications publish events to a Redis pub/sub channel or list.
type RedisNotification struct {
	// Embed the common notification properties
	BaseNotification

	// Addr is the Redis server address (host:port)
	Addr string

	// Password is the Redis server password, if required
	Password string

	// Key is the Redis key where events will be published
	Key string
}

// KafkaNotification defines a Kafka notification target.
// Kafka notifications publish events to a Kafka topic.
type KafkaNotification struct {
	// Embed the common notification properties
	BaseNotification

	// Brokers is a list of Kafka broker addresses
	Brokers []string

	// Topic is the Kafka topic where events will be published
	Topic string

	// SASL contains optional SASL authentication details for Kafka
	SASL *KafkaSASLAuth
}

// KafkaSASLAuth contains Kafka SASL authentication details.
// SASL is used for authenticating with Kafka brokers.
type KafkaSASLAuth struct {
	// Enable turns on SASL authentication
	Enable bool

	// Username for SASL authentication
	Username string

	// Password for SASL authentication
	Password string
}

// MQTTNotification defines an MQTT notification target.
// MQTT notifications publish events to an MQTT broker.
type MQTTNotification struct {
	// Embed the common notification properties
	BaseNotification

	// Broker is the MQTT broker address (host:port)
	Broker string

	// Topic is the MQTT topic where events will be published
	Topic string

	// QoS (Quality of Service) level for message delivery:
	// 0: At most once (fire and forget)
	// 1: At least once (acknowledged delivery)
	// 2: Exactly once (assured delivery)
	QoS byte

	// Username for MQTT broker authentication
	Username string

	// Password for MQTT broker authentication
	Password string

	// Reconnect flag enables automatic reconnection on connection loss
	Reconnect bool
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

// Functional options types for flexible API configuration

// PutOption is a functional option for configuring Put operations.
type PutOption func(*PutOptions)

// PutOptions contains options for Put operations.
type PutOptions struct {
	// Size is the size of the object in bytes. If not specified, size is unknown.
	Size int64

	// ContentType is the MIME type of the object
	ContentType string

	// Metadata is custom metadata to attach to the object
	Metadata map[string]string

	// PartSize is the part size for multipart uploads
	PartSize uint64
}

// GetOption is a functional option for configuring Get operations.
type GetOption func(*GetOptions)

// GetOptions contains options for Get operations.
type GetOptions struct {
	// VersionID specifies a particular version of an object to retrieve
	VersionID string

	// Range specifies a byte range to retrieve
	Range *ByteRange
}

// ByteRange represents a byte range for partial object retrieval.
type ByteRange struct {
	// Start is the starting byte position (inclusive)
	Start int64

	// End is the ending byte position (inclusive)
	End int64
}

// BucketOption is a functional option for configuring bucket operations.
type BucketOption func(*BucketOptions)

// BucketOptions contains options for bucket operations.
type BucketOptions struct {
	// Region specifies the region where the bucket should be created
	Region string

	// ObjectLocking enables object locking for the bucket
	ObjectLocking bool
}

// BucketInfo contains information about a bucket.
type BucketInfo struct {
	// Name is the name of the bucket
	Name string

	// CreationDate is when the bucket was created
	CreationDate time.Time
}
