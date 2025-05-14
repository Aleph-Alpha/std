package minio

import "time"

const (
	unknownSize                   int64 = -1
	connectionHealthCheckInterval       = 3 * time.Second
	minPartSizeForUpload          int64 = 5 * 1024 * 1024
	minioLimitPartCount           int64 = 10000
	MultipartThreshold            int64 = 50 * 1024 * 1024
	MaxObjectSize                 int64 = 5 * 1024 * 1024 * 1024 * 1024
)

// Config defines the top-level configuration for MinIO.
type Config struct {
	Connection      ConnectionConfig   // Connection details for MinIO server
	UploadConfig    UploadConfig       // Configuration for upload size and part size
	DownloadConfig  DownloadConfig     // Configuration for download behavior
	PresignedConfig PresignedConfig    // Configuration for presigned operations
	Notification    NotificationConfig // Configuration for notifications
}

// ConnectionConfig contains MinIO server connection details.
type ConnectionConfig struct {
	Endpoint        string // MinIO server endpoint, e.g., "localhost:9000"
	AccessKeyID     string // MinIO access key
	SecretAccessKey string // MinIO secret key
	UseSSL          bool   // Use SSL (true for "https", false for "http")
	BucketName      string // Default bucket name
	Region          string // Region for the bucket (e.g., "us-east-1")
}

// UploadConfig defines the configuration for upload constraints.
type UploadConfig struct {
	MaxObjectSize      int64  // Maximum size of a single object in bytes (e.g., 5 GiB)
	MinPartSize        uint64 // Minimum part size for multipart uploads (e.g., 5 MiB)
	MultipartThreshold int64  // Switch to multipart when the object exceeds this size (e.g., 50 MiB)
}

type DownloadConfig struct {
	SmallFileThreshold int64 // Size in bytes below which we use pre-allocated buffer
	InitialBufferSize  int   // Initial buffer size for large files
}

// PresignedConfig contains configuration options for presigned URLs.
type PresignedConfig struct {
	Enabled        bool          // Enable or disable presigned URL functionality
	ExpiryDuration time.Duration // Expiration for presigned URLs (e.g., 15m)
	BaseURL        string        // Base URL for presigned links (e.g., "https://cdn.mysite.com")
}

// NotificationConfig defines the configuration for event notifications.
type NotificationConfig struct {
	Enabled bool                  // Enable or disable notifications
	Webhook []WebhookNotification // Webhook notification targets
	AMQP    []AMQPNotification    // AMQP notification targets
	Redis   []RedisNotification   // Redis notification targets
	Kafka   []KafkaNotification   // Kafka notification targets
	MQTT    []MQTTNotification    // MQTT notification targets
}

// BaseNotification contains common properties for all notification types
type BaseNotification struct {
	Events []string // Events to subscribe to (e.g., "s3:ObjectCreated:*")
	Prefix string   // Filter by object key prefix
	Suffix string   // Filter by object key suffix
	Name   string   // Unique identifier for this notification config
}

// WebhookNotification defines a webhook notification target
type WebhookNotification struct {
	BaseNotification
	Endpoint  string // Webhook URL
	AuthToken string // Optional authentication token
}

// AMQPNotification defines an AMQP notification target
type AMQPNotification struct {
	BaseNotification
	URL          string // AMQP server URL
	Exchange     string // Exchange name
	RoutingKey   string // Routing key
	ExchangeType string // Exchange type (default: "fanout")
	Mandatory    bool   // Mandatory flag
	Immediate    bool   // Immediate flag
	Durable      bool   // Durable flag
	Internal     bool   // Internal flag
	NoWait       bool   // NoWait flag
	AutoDeleted  bool   // Auto-deleted flag
}

// RedisNotification defines a Redis notification target
type RedisNotification struct {
	BaseNotification
	Addr     string // Redis server address
	Password string // Redis password
	Key      string // Redis key where events will be published
}

// KafkaNotification defines a Kafka notification target
type KafkaNotification struct {
	BaseNotification
	Brokers []string       // Kafka broker addresses
	Topic   string         // Kafka topic
	SASL    *KafkaSASLAuth // Optional SASL authentication
}

// KafkaSASLAuth contains Kafka SASL authentication details
type KafkaSASLAuth struct {
	Enable   bool   // Enable SASL authentication
	Username string // SASL username
	Password string // SASL password
}

// MQTTNotification defines an MQTT notification target
type MQTTNotification struct {
	BaseNotification
	Broker    string // MQTT broker address
	Topic     string // MQTT topic
	QoS       byte   // Quality of Service (0, 1, or 2)
	Username  string // MQTT username
	Password  string // MQTT password
	Reconnect bool   // Auto-reconnect on connection loss
}
