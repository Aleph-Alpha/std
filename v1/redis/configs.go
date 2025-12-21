package redis

import "time"

// Config defines the top-level configuration structure for the Redis client.
// It contains all the necessary configuration sections for establishing connections,
// setting up connection pooling, and configuring TLS/SSL.
type Config struct {
	// Host is the Redis server hostname or IP address
	// Default: "localhost"
	Host string

	// Port is the Redis server port
	// Default: 6379
	Port int

	// Username is the Redis username for ACL authentication (Redis 6.0+)
	// Leave empty for no username-based authentication
	Username string

	// Password is the Redis password for authentication
	// Leave empty for no authentication
	Password string

	// DB is the Redis database number to use (0-15 by default)
	// Default: 0
	DB int

	// PoolSize is the maximum number of socket connections
	// Default: 10 per CPU
	PoolSize int

	// MinIdleConns is the minimum number of idle connections to maintain
	// Default: 0 (no minimum)
	MinIdleConns int

	// MaxConnAge is the maximum duration a connection can be reused
	// Connections older than this will be closed and new ones created
	// Default: 0 (no maximum age)
	MaxConnAge time.Duration

	// PoolTimeout is the amount of time to wait for a connection from the pool
	// Default: ReadTimeout + 1 second
	PoolTimeout time.Duration

	// IdleTimeout is the amount of time after which idle connections are closed
	// Default: 5 minutes
	IdleTimeout time.Duration

	// IdleCheckFrequency is how often to check for idle connections to close
	// Default: 1 minute
	IdleCheckFrequency time.Duration

	// MaxRetries is the maximum number of retries before giving up
	// Default: 3
	// Set to -1 to disable retries
	MaxRetries int

	// MinRetryBackoff is the minimum backoff between each retry
	// Default: 8 milliseconds
	MinRetryBackoff time.Duration

	// MaxRetryBackoff is the maximum backoff between each retry
	// Default: 512 milliseconds
	MaxRetryBackoff time.Duration

	// DialTimeout is the timeout for establishing new connections
	// Default: 5 seconds
	DialTimeout time.Duration

	// ReadTimeout is the timeout for socket reads
	// If reached, commands will fail with a timeout instead of blocking
	// Default: 3 seconds
	ReadTimeout time.Duration

	// WriteTimeout is the timeout for socket writes
	// If reached, commands will fail with a timeout instead of blocking
	// Default: ReadTimeout
	WriteTimeout time.Duration

	// TLS contains TLS/SSL configuration
	TLS TLSConfig

	// Logger is an optional logger from std/v1/logger package
	// If provided, it will be used for Redis error logging
	Logger Logger
}

// TLSConfig contains TLS/SSL configuration parameters.
type TLSConfig struct {
	// Enabled determines whether to use TLS/SSL for the connection
	Enabled bool

	// CACertPath is the file path to the CA certificate for verifying the server
	CACertPath string

	// ClientCertPath is the file path to the client certificate
	ClientCertPath string

	// ClientKeyPath is the file path to the client certificate's private key
	ClientKeyPath string

	// InsecureSkipVerify controls whether to skip verification of the server's certificate
	// WARNING: Setting this to true is insecure and should only be used in testing
	InsecureSkipVerify bool

	// ServerName is used to verify the hostname on the returned certificates
	// If empty, the Host from the main config is used
	ServerName string
}

// ClusterConfig defines the configuration for Redis Cluster mode.
// Use this when connecting to a Redis Cluster deployment.
type ClusterConfig struct {
	// Addrs is a seed list of cluster nodes
	// Example: []string{"localhost:7000", "localhost:7001", "localhost:7002"}
	Addrs []string

	// Username is the Redis username for ACL authentication (Redis 6.0+)
	Username string

	// Password is the Redis password for authentication
	Password string

	// MaxRedirects is the maximum number of retries for MOVED/ASK redirects
	// Default: 3
	MaxRedirects int

	// ReadOnly enables read-only mode (read from replicas)
	// Default: false
	ReadOnly bool

	// RouteByLatency enables routing read-only commands to the closest master or replica node
	// Default: false
	RouteByLatency bool

	// RouteRandomly enables routing read-only commands to random master or replica nodes
	// Default: false
	RouteRandomly bool

	// PoolSize is the maximum number of socket connections per node
	// Default: 10 per CPU
	PoolSize int

	// MinIdleConns is the minimum number of idle connections per node
	// Default: 0
	MinIdleConns int

	// MaxConnAge is the maximum duration a connection can be reused
	// Default: 0 (no maximum age)
	MaxConnAge time.Duration

	// PoolTimeout is the amount of time to wait for a connection from the pool
	// Default: ReadTimeout + 1 second
	PoolTimeout time.Duration

	// IdleTimeout is the amount of time after which idle connections are closed
	// Default: 5 minutes
	IdleTimeout time.Duration

	// MaxRetries is the maximum number of retries before giving up
	// Default: 3
	MaxRetries int

	// MinRetryBackoff is the minimum backoff between each retry
	// Default: 8 milliseconds
	MinRetryBackoff time.Duration

	// MaxRetryBackoff is the maximum backoff between each retry
	// Default: 512 milliseconds
	MaxRetryBackoff time.Duration

	// DialTimeout is the timeout for establishing new connections
	// Default: 5 seconds
	DialTimeout time.Duration

	// ReadTimeout is the timeout for socket reads
	// Default: 3 seconds
	ReadTimeout time.Duration

	// WriteTimeout is the timeout for socket writes
	// Default: ReadTimeout
	WriteTimeout time.Duration

	// TLS contains TLS/SSL configuration
	TLS TLSConfig

	// Logger is an optional logger from std/v1/logger package
	Logger Logger
}

// FailoverConfig defines the configuration for Redis Sentinel (failover) mode.
// Use this when connecting to a Redis Sentinel setup for high availability.
type FailoverConfig struct {
	// MasterName is the name of the master instance as configured in Sentinel
	MasterName string

	// SentinelAddrs is a list of Sentinel node addresses
	// Example: []string{"localhost:26379", "localhost:26380", "localhost:26381"}
	SentinelAddrs []string

	// SentinelUsername is the username for Sentinel authentication (Redis 6.0+)
	SentinelUsername string

	// SentinelPassword is the password for Sentinel authentication
	SentinelPassword string

	// Username is the Redis username for ACL authentication (Redis 6.0+)
	Username string

	// Password is the Redis password for authentication
	Password string

	// DB is the Redis database number to use
	// Default: 0
	DB int

	// ReplicaOnly forces read-only queries to go to replica nodes
	// Default: false
	ReplicaOnly bool

	// UseDisconnectedReplicas allows using replicas that are disconnected from master
	// Default: false
	UseDisconnectedReplicas bool

	// PoolSize is the maximum number of socket connections
	// Default: 10 per CPU
	PoolSize int

	// MinIdleConns is the minimum number of idle connections
	// Default: 0
	MinIdleConns int

	// MaxConnAge is the maximum duration a connection can be reused
	// Default: 0 (no maximum age)
	MaxConnAge time.Duration

	// PoolTimeout is the amount of time to wait for a connection from the pool
	// Default: ReadTimeout + 1 second
	PoolTimeout time.Duration

	// IdleTimeout is the amount of time after which idle connections are closed
	// Default: 5 minutes
	IdleTimeout time.Duration

	// MaxRetries is the maximum number of retries before giving up
	// Default: 3
	MaxRetries int

	// MinRetryBackoff is the minimum backoff between each retry
	// Default: 8 milliseconds
	MinRetryBackoff time.Duration

	// MaxRetryBackoff is the maximum backoff between each retry
	// Default: 512 milliseconds
	MaxRetryBackoff time.Duration

	// DialTimeout is the timeout for establishing new connections
	// Default: 5 seconds
	DialTimeout time.Duration

	// ReadTimeout is the timeout for socket reads
	// Default: 3 seconds
	ReadTimeout time.Duration

	// WriteTimeout is the timeout for socket writes
	// Default: ReadTimeout
	WriteTimeout time.Duration

	// TLS contains TLS/SSL configuration
	TLS TLSConfig

	// Logger is an optional logger from std/v1/logger package
	Logger Logger
}

// Logger is an interface that matches the std/v1/logger.Logger
type Logger interface {
	Error(msg string, err error, fields ...map[string]interface{})
	Info(msg string, err error, fields ...map[string]interface{})
	Warn(msg string, err error, fields ...map[string]interface{})
}

// Default values for configuration
const (
	DefaultHost                = "localhost"
	DefaultPort                = 6379
	DefaultDB                  = 0
	DefaultPoolSize            = 0 // 10 per CPU (set by redis client)
	DefaultMinIdleConns        = 0
	DefaultMaxConnAge          = 0
	DefaultPoolTimeout         = 0 // ReadTimeout + 1 second (set by redis client)
	DefaultIdleTimeout         = 5 * time.Minute
	DefaultIdleCheckFrequency  = 1 * time.Minute
	DefaultMaxRetries          = 3
	DefaultMinRetryBackoff     = 8 * time.Millisecond
	DefaultMaxRetryBackoff     = 512 * time.Millisecond
	DefaultDialTimeout         = 5 * time.Second
	DefaultReadTimeout         = 3 * time.Second
	DefaultWriteTimeout        = 0 // ReadTimeout (set by redis client)
	DefaultClusterMaxRedirects = 3
)
