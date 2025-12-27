package redis

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log"
	"os"
	"sync"

	"github.com/Aleph-Alpha/std/v1/observability"
	"github.com/redis/go-redis/v9"
)

// RedisClient represents a client for interacting with Redis.
// It wraps the go-redis client and provides a simplified interface
// with connection management and helper methods.
//
// RedisClient implements the Client interface.
type RedisClient struct {
	// client is the underlying Redis client
	client redis.UniversalClient

	// cfg stores the configuration for this Redis client
	cfg interface{} // Can be Config, ClusterConfig, or FailoverConfig

	// logger is used for structured logging
	logger Logger

	// observer provides optional observability hooks for tracking operations
	observer observability.Observer

	// mu protects concurrent access to client
	mu sync.RWMutex

	// shutdownSignal is closed when the client is being shut down
	shutdownSignal chan struct{}

	closeShutdownOnce sync.Once
}

// NewClient creates and initializes a new Redis client with the provided configuration.
// This is for connecting to a standalone Redis instance.
//
// Parameters:
//   - cfg: Configuration for connecting to Redis
//
// Returns a new Redis client instance that is ready to use.
//
// Example:
//
//	client, err := redis.NewClient(redis.Config{
//		Host:     "localhost",
//		Port:     6379,
//		Password: "",
//		DB:       0,
//	})
//	if err != nil {
//		log.Printf("ERROR: failed to create Redis client: %v", err)
//		return nil, err
//	}
//	defer client.Close()
func NewClient(cfg Config) (*RedisClient, error) {
	// Apply defaults
	if cfg.Host == "" {
		cfg.Host = DefaultHost
	}
	if cfg.Port == 0 {
		cfg.Port = DefaultPort
	}
	if cfg.MaxRetries == 0 {
		cfg.MaxRetries = DefaultMaxRetries
	}
	if cfg.MinRetryBackoff == 0 {
		cfg.MinRetryBackoff = DefaultMinRetryBackoff
	}
	if cfg.MaxRetryBackoff == 0 {
		cfg.MaxRetryBackoff = DefaultMaxRetryBackoff
	}
	if cfg.DialTimeout == 0 {
		cfg.DialTimeout = DefaultDialTimeout
	}
	if cfg.ReadTimeout == 0 {
		cfg.ReadTimeout = DefaultReadTimeout
	}
	if cfg.IdleTimeout == 0 {
		cfg.IdleTimeout = DefaultIdleTimeout
	}
	if cfg.IdleCheckFrequency == 0 {
		cfg.IdleCheckFrequency = DefaultIdleCheckFrequency
	}

	// Set up TLS config if enabled
	var tlsConfig *tls.Config
	var err error
	if cfg.TLS.Enabled {
		tlsConfig, err = createTLSConfig(cfg.TLS, cfg.Host)
		if err != nil {
			return nil, fmt.Errorf("failed to create TLS config: %w", err)
		}
	}

	// Create Redis options
	opts := &redis.Options{
		Addr:            fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
		Username:        cfg.Username,
		Password:        cfg.Password,
		DB:              cfg.DB,
		PoolSize:        cfg.PoolSize,
		MinIdleConns:    cfg.MinIdleConns,
		ConnMaxLifetime: cfg.MaxConnAge,
		PoolTimeout:     cfg.PoolTimeout,
		ConnMaxIdleTime: cfg.IdleTimeout,
		MaxRetries:      cfg.MaxRetries,
		MinRetryBackoff: cfg.MinRetryBackoff,
		MaxRetryBackoff: cfg.MaxRetryBackoff,
		DialTimeout:     cfg.DialTimeout,
		ReadTimeout:     cfg.ReadTimeout,
		WriteTimeout:    cfg.WriteTimeout,
		TLSConfig:       tlsConfig,
	}

	client := redis.NewClient(opts)

	r := &RedisClient{
		client:         client,
		cfg:            cfg,
		logger:         cfg.Logger,
		shutdownSignal: make(chan struct{}),
	}

	log.Println("INFO: Redis client initialized")
	return r, nil
}

// NewClusterClient creates and initializes a new Redis Cluster client.
// This is for connecting to a Redis Cluster deployment.
//
// Parameters:
//   - cfg: Configuration for connecting to Redis Cluster
//
// Returns a new Redis client instance that is ready to use.
//
// Example:
//
//	client, err := redis.NewClusterClient(redis.ClusterConfig{
//		Addrs: []string{
//			"localhost:7000",
//			"localhost:7001",
//			"localhost:7002",
//		},
//		Password: "",
//	})
func NewClusterClient(cfg ClusterConfig) (*RedisClient, error) {
	// Apply defaults
	if cfg.MaxRedirects == 0 {
		cfg.MaxRedirects = DefaultClusterMaxRedirects
	}
	if cfg.MaxRetries == 0 {
		cfg.MaxRetries = DefaultMaxRetries
	}
	if cfg.MinRetryBackoff == 0 {
		cfg.MinRetryBackoff = DefaultMinRetryBackoff
	}
	if cfg.MaxRetryBackoff == 0 {
		cfg.MaxRetryBackoff = DefaultMaxRetryBackoff
	}
	if cfg.DialTimeout == 0 {
		cfg.DialTimeout = DefaultDialTimeout
	}
	if cfg.ReadTimeout == 0 {
		cfg.ReadTimeout = DefaultReadTimeout
	}
	if cfg.IdleTimeout == 0 {
		cfg.IdleTimeout = DefaultIdleTimeout
	}

	// Set up TLS config if enabled
	var tlsConfig *tls.Config
	var err error
	if cfg.TLS.Enabled {
		tlsConfig, err = createTLSConfig(cfg.TLS, "")
		if err != nil {
			return nil, fmt.Errorf("failed to create TLS config: %w", err)
		}
	}

	// Create Redis Cluster options
	opts := &redis.ClusterOptions{
		Addrs:           cfg.Addrs,
		Username:        cfg.Username,
		Password:        cfg.Password,
		MaxRedirects:    cfg.MaxRedirects,
		ReadOnly:        cfg.ReadOnly,
		RouteByLatency:  cfg.RouteByLatency,
		RouteRandomly:   cfg.RouteRandomly,
		PoolSize:        cfg.PoolSize,
		MinIdleConns:    cfg.MinIdleConns,
		ConnMaxLifetime: cfg.MaxConnAge,
		PoolTimeout:     cfg.PoolTimeout,
		ConnMaxIdleTime: cfg.IdleTimeout,
		MaxRetries:      cfg.MaxRetries,
		MinRetryBackoff: cfg.MinRetryBackoff,
		MaxRetryBackoff: cfg.MaxRetryBackoff,
		DialTimeout:     cfg.DialTimeout,
		ReadTimeout:     cfg.ReadTimeout,
		WriteTimeout:    cfg.WriteTimeout,
		TLSConfig:       tlsConfig,
	}

	client := redis.NewClusterClient(opts)

	r := &RedisClient{
		client:         client,
		cfg:            cfg,
		logger:         cfg.Logger,
		shutdownSignal: make(chan struct{}),
	}

	log.Println("INFO: Redis Cluster client initialized")
	return r, nil
}

// NewFailoverClient creates and initializes a new Redis Sentinel (failover) client.
// This is for connecting to a Redis Sentinel setup for high availability.
//
// Parameters:
//   - cfg: Configuration for connecting to Redis Sentinel
//
// Returns a new Redis client instance that is ready to use.
//
// Example:
//
//	client, err := redis.NewFailoverClient(redis.FailoverConfig{
//		MasterName: "mymaster",
//		SentinelAddrs: []string{
//			"localhost:26379",
//			"localhost:26380",
//			"localhost:26381",
//		},
//		Password: "",
//		DB:       0,
//	})
func NewFailoverClient(cfg FailoverConfig) (*RedisClient, error) {
	// Apply defaults
	if cfg.MaxRetries == 0 {
		cfg.MaxRetries = DefaultMaxRetries
	}
	if cfg.MinRetryBackoff == 0 {
		cfg.MinRetryBackoff = DefaultMinRetryBackoff
	}
	if cfg.MaxRetryBackoff == 0 {
		cfg.MaxRetryBackoff = DefaultMaxRetryBackoff
	}
	if cfg.DialTimeout == 0 {
		cfg.DialTimeout = DefaultDialTimeout
	}
	if cfg.ReadTimeout == 0 {
		cfg.ReadTimeout = DefaultReadTimeout
	}
	if cfg.IdleTimeout == 0 {
		cfg.IdleTimeout = DefaultIdleTimeout
	}

	// Set up TLS config if enabled
	var tlsConfig *tls.Config
	var err error
	if cfg.TLS.Enabled {
		tlsConfig, err = createTLSConfig(cfg.TLS, "")
		if err != nil {
			return nil, fmt.Errorf("failed to create TLS config: %w", err)
		}
	}

	// Create Redis Failover options
	opts := &redis.FailoverOptions{
		MasterName:              cfg.MasterName,
		SentinelAddrs:           cfg.SentinelAddrs,
		SentinelUsername:        cfg.SentinelUsername,
		SentinelPassword:        cfg.SentinelPassword,
		Username:                cfg.Username,
		Password:                cfg.Password,
		DB:                      cfg.DB,
		ReplicaOnly:             cfg.ReplicaOnly,
		UseDisconnectedReplicas: cfg.UseDisconnectedReplicas,
		PoolSize:                cfg.PoolSize,
		MinIdleConns:            cfg.MinIdleConns,
		ConnMaxLifetime:         cfg.MaxConnAge,
		PoolTimeout:             cfg.PoolTimeout,
		ConnMaxIdleTime:         cfg.IdleTimeout,
		MaxRetries:              cfg.MaxRetries,
		MinRetryBackoff:         cfg.MinRetryBackoff,
		MaxRetryBackoff:         cfg.MaxRetryBackoff,
		DialTimeout:             cfg.DialTimeout,
		ReadTimeout:             cfg.ReadTimeout,
		WriteTimeout:            cfg.WriteTimeout,
		TLSConfig:               tlsConfig,
	}

	client := redis.NewFailoverClient(opts)

	r := &RedisClient{
		client:         client,
		cfg:            cfg,
		logger:         cfg.Logger,
		shutdownSignal: make(chan struct{}),
	}

	log.Println("INFO: Redis Failover client initialized")
	return r, nil
}

// createTLSConfig creates a TLS configuration from the provided config
func createTLSConfig(cfg TLSConfig, defaultServerName string) (*tls.Config, error) {
	tlsConfig := &tls.Config{
		InsecureSkipVerify: cfg.InsecureSkipVerify,
	}

	// Set server name for TLS verification
	if cfg.ServerName != "" {
		tlsConfig.ServerName = cfg.ServerName
	} else if defaultServerName != "" {
		tlsConfig.ServerName = defaultServerName
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

// Client returns the underlying go-redis client for advanced operations.
// This allows users to access the full go-redis API when needed.
func (r *RedisClient) Client() redis.UniversalClient {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.client
}

// Close closes the Redis client and releases all resources.
// This should be called when the client is no longer needed.
func (r *RedisClient) Close() error {
	r.closeShutdownOnce.Do(func() {
		close(r.shutdownSignal)
	})

	r.mu.Lock()
	defer r.mu.Unlock()

	log.Println("INFO: Closing Redis client")

	if r.client != nil {
		if err := r.client.Close(); err != nil {
			log.Printf("WARN: Failed to close Redis client: %v", err)
			return err
		}
	}

	return nil
}

// WithObserver sets the observer for this client and returns the client for method chaining.
// The observer receives events about Redis operations (e.g., get, set, delete).
//
// Example:
//
//	client := client.WithObserver(myObserver).WithLogger(myLogger)
func (r *RedisClient) WithObserver(observer observability.Observer) *RedisClient {
	r.observer = observer
	return r
}

// WithLogger sets the logger for this client and returns the client for method chaining.
// The logger is used for structured logging of client operations and errors.
//
// Example:
//
//	client := client.WithObserver(myObserver).WithLogger(myLogger)
func (r *RedisClient) WithLogger(logger Logger) *RedisClient {
	r.logger = logger
	return r
}
