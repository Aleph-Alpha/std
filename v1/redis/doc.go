// Package redis provides functionality for interacting with Redis.
//
// The redis package offers a simplified interface for working with Redis
// key-value store, providing connection management, caching operations,
// pub/sub capabilities, and advanced data structure operations with a focus
// on reliability and ease of use.
//
// # Architecture
//
// This package follows the "accept interfaces, return structs" design pattern:
//   - Client interface: Defines the contract for Redis operations
//   - RedisClient struct: Concrete implementation of the Client interface
//   - NewClient constructor: Returns *RedisClient (concrete type)
//   - FX module: Provides both *RedisClient and Client interface for dependency injection
//
// Core Features:
//   - Robust connection management with automatic reconnection
//   - Connection pooling for optimal performance
//   - Simple key-value operations (Get, Set, Delete)
//   - Advanced data structures (Lists, Sets, Sorted Sets, Hashes)
//   - Pub/Sub messaging support
//   - Pipeline and transaction support
//   - TTL and expiration management
//   - Integration with the Logger package for structured logging
//   - TLS/SSL support for secure connections
//   - Cluster and Sentinel support
//
// # Direct Usage (Without FX)
//
// For simple applications or tests, create a client directly:
//
//	import (
//		"github.com/Aleph-Alpha/std/v1/redis"
//		"context"
//		"time"
//	)
//
//	// Create a new Redis client (returns concrete *RedisClient)
//	client, err := redis.NewClient(redis.Config{
//		Host:     "localhost",
//		Port:     6379,
//		Password: "",
//		DB:       0,
//	})
//	if err != nil {
//		log.Fatal("Failed to connect to Redis", err, nil)
//	}
//	defer client.Close()
//
//	// Use the client
//	ctx := context.Background()
//	err = client.Set(ctx, "user:123", "John Doe", 5*time.Minute)
//
// # FX Module Integration
//
// For production applications using Uber's fx, use the FXModule which provides
// both the concrete type and interface:
//
//	import (
//		"github.com/Aleph-Alpha/std/v1/redis"
//		"github.com/Aleph-Alpha/std/v1/logger"
//		"go.uber.org/fx"
//	)
//
//	app := fx.New(
//		logger.FXModule, // Optional: provides std logger
//		redis.FXModule,  // Provides *RedisClient and redis.Client interface
//		fx.Provide(func() redis.Config {
//			return redis.Config{
//				Host: "localhost",
//				Port: 6379,
//			}
//		}),
//		fx.Invoke(func(client *redis.RedisClient) {
//			// Use concrete type directly
//			ctx := context.Background()
//			client.Set(ctx, "key", "value", 0)
//		}),
//		// ... other modules
//	)
//	app.Run()
//
// # Observability (Observer Hook)
//
// Redis supports optional observability through the Observer interface from the observability package.
// This allows external systems to track Redis operations without coupling the package to specific
// metrics/tracing implementations.
//
// Using WithObserver (non-FX usage):
//
//	client, err := redis.NewClient(config)
//	if err != nil {
//	    return err
//	}
//	client = client.WithObserver(myObserver).WithLogger(myLogger)
//	defer client.Close()
//
// Using FX (automatic injection):
//
//	app := fx.New(
//	    redis.FXModule,
//	    logger.FXModule,  // Optional: provides logger
//	    fx.Provide(
//	        func() redis.Config { return loadConfig() },
//	        func() observability.Observer { return myObserver },  // Optional
//	    ),
//	)
//
// The observer receives events for Redis operations:
//   - Component: "redis"
//   - Operations: "get", "set", "setnx", "delete", "mget", "mset",
//     "hget", "hset", "hgetall", "lpush", "lrange", "publish"
//   - Resource: key name (or first key for multi-key operations)
//   - SubResource: field name (for hash operations) or channel name
//   - Duration: operation duration
//   - Error: any error that occurred
//   - Size: bytes or count returned/affected
//   - Metadata: operation-specific details (e.g., ttl, key_count, field_count)
//
// # Type Aliases in Consumer Code
//
// To simplify your code and make it database-agnostic, use type aliases:
//
//	package myapp
//
//	import stdRedis "github.com/Aleph-Alpha/std/v1/redis"
//
//	// Use type alias to reference std's interface
//	type RedisClient = stdRedis.Client
//
//	// Now use RedisClient throughout your codebase
//	func MyFunction(client RedisClient) {
//		client.Get(ctx, "key")
//	}
//
// This eliminates the need for adapters and allows you to switch implementations
// by only changing the alias definition.
//
// # Basic Operations
//
//	// Set hash fields
//	err = client.HSet(ctx, "user:123", map[string]interface{}{
//		"name":  "John Doe",
//		"email": "john@example.com",
//		"age":   30,
//	})
//
//	// Get a single hash field
//	name, err := client.HGet(ctx, "user:123", "name")
//
//	// Get all hash fields
//	user, err := client.HGetAll(ctx, "user:123")
//
//	// Delete hash fields
//	err = client.HDel(ctx, "user:123", "age")
//
// # List Operations
//
//	// Push to list (left/right)
//	err = client.LPush(ctx, "tasks", "task1", "task2")
//	err = client.RPush(ctx, "tasks", "task3")
//
//	// Pop from list
//	task, err := client.LPop(ctx, "tasks")
//	task, err := client.RPop(ctx, "tasks")
//
//	// Get list range
//	tasks, err := client.LRange(ctx, "tasks", 0, -1)
//
//	// Get list length
//	length, err := client.LLen(ctx, "tasks")
//
// # Set Operations
//
//	// Add members to set
//	err = client.SAdd(ctx, "tags", "redis", "cache", "database")
//
//	// Get all members
//	tags, err := client.SMembers(ctx, "tags")
//
//	// Check membership
//	exists, err := client.SIsMember(ctx, "tags", "redis")
//
//	// Remove members
//	err = client.SRem(ctx, "tags", "cache")
//
// # Sorted Set Operations
//
//	// Add members with scores
//	err = client.ZAdd(ctx, "leaderboard", map[string]float64{
//		"player1": 100,
//		"player2": 200,
//		"player3": 150,
//	})
//
//	// Get range by rank
//	players, err := client.ZRange(ctx, "leaderboard", 0, -1)
//
//	// Get range by score
//	players, err := client.ZRangeByScore(ctx, "leaderboard", 100, 200)
//
//	// Get member score
//	score, err := client.ZScore(ctx, "leaderboard", "player1")
//
// # Pub/Sub Messaging
//
//	// Publisher
//	err = client.Publish(ctx, "events", "user.created")
//
//	// Subscriber
//	pubsub := client.Subscribe(ctx, "events")
//	defer pubsub.Close()
//
//	for msg := range pubsub.Channel() {
//		fmt.Println("Received:", msg.Channel, msg.Payload)
//	}
//
// # Pattern-based Subscription
//
//	// Subscribe to pattern
//	pubsub := client.PSubscribe(ctx, "user.*")
//	defer pubsub.Close()
//
//	for msg := range pubsub.Channel() {
//		fmt.Println("Received on pattern:", msg.Channel, msg.Payload)
//	}
//
// # Pipeline for Bulk Operations
//
//	// Create pipeline
//	pipe := client.Pipeline()
//
//	// Queue multiple commands
//	pipe.Set(ctx, "key1", "value1", 0)
//	pipe.Set(ctx, "key2", "value2", 0)
//	pipe.Incr(ctx, "counter")
//
//	// Execute all commands at once
//	_, err := pipe.Exec(ctx)
//	if err != nil {
//		log.Error("Pipeline failed", err, nil)
//	}
//
// # Transactions (MULTI/EXEC)
//
//	// Watch keys for optimistic locking
//	err = client.Watch(ctx, func(tx *redis.Tx) error {
//		// Get current value
//		val, err := tx.Get(ctx, "counter").Int()
//		if err != nil && err != redis.Nil {
//			return err
//		}
//
//		// Start transaction
//		_, err = tx.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
//			pipe.Set(ctx, "counter", val+1, 0)
//			return nil
//		})
//		return err
//	}, "counter")
//
// # TTL and Expiration
//
//	// Set expiration on existing key
//	err = client.Expire(ctx, "session:123", 30*time.Minute)
//
//	// Get TTL
//	ttl, err := client.TTL(ctx, "session:123")
//
//	// Persist (remove expiration)
//	err = client.Persist(ctx, "session:123")
//
// # Key Scanning
//
//	// Scan keys with pattern
//	keys, err := client.Keys(ctx, "user:*")
//
//	// Scan with cursor (for large datasets)
//	iter := client.Scan(ctx, 0, "user:*", 100)
//	for iter.Next(ctx) {
//		key := iter.Val()
//		fmt.Println("Key:", key)
//	}
//	if err := iter.Err(); err != nil {
//		log.Error("Scan error", err, nil)
//	}
//
// # JSON Operations
//
//	// Set JSON value
//	user := map[string]interface{}{
//		"name":  "John",
//		"email": "john@example.com",
//		"age":   30,
//	}
//	err = client.JSONSet(ctx, "user:123", "$", user)
//
//	// Get JSON value
//	var result map[string]interface{}
//	err = client.JSONGet(ctx, "user:123", &result, "$")
//
//	// Update JSON field
//	err = client.JSONSet(ctx, "user:123", "$.age", 31)
//
// # Distributed Locking
//
//	// Acquire lock
//	lock, err := client.AcquireLock(ctx, "resource:123", 10*time.Second)
//	if err != nil {
//		log.Error("Failed to acquire lock", err, nil)
//		return
//	}
//	defer lock.Release(ctx)
//
//	// Do work with exclusive access
//	// ...
//
// # Configuration
//
// The redis client can be configured via environment variables or explicitly:
//
//	REDIS_HOST=localhost
//	REDIS_PORT=6379
//	REDIS_PASSWORD=secret
//	REDIS_DB=0
//	REDIS_POOL_SIZE=10
//
// # Custom Logger Integration
//
// You can integrate the std/v1/logger for better error logging:
//
//	import (
//		"github.com/Aleph-Alpha/std/v1/logger"
//		"github.com/Aleph-Alpha/std/v1/redis"
//	)
//
//	// Create logger
//	log := logger.NewLoggerClient(logger.Config{
//		Level:       logger.Info,
//		ServiceName: "my-service",
//	})
//
//	// Create Redis client with logger
//	client, err := redis.NewClient(redis.Config{
//		Host:     "localhost",
//		Port:     6379,
//		Logger:   log, // Redis errors will use this logger
//	})
//
// # TLS/SSL Configuration
//
//	client, err := redis.NewClient(redis.Config{
//		Host:     "redis.example.com",
//		Port:     6380,
//		TLS: redis.TLSConfig{
//			Enabled:            true,
//			CACertPath:         "/path/to/ca.crt",
//			ClientCertPath:     "/path/to/client.crt",
//			ClientKeyPath:      "/path/to/client.key",
//			InsecureSkipVerify: false,
//		},
//	})
//
// # Cluster Configuration
//
//	client, err := redis.NewClusterClient(redis.ClusterConfig{
//		Addrs: []string{
//			"localhost:7000",
//			"localhost:7001",
//			"localhost:7002",
//		},
//		Password: "secret",
//		TLS:      tlsConfig,
//	})
//
// # Sentinel Configuration
//
//	client, err := redis.NewFailoverClient(redis.FailoverConfig{
//		MasterName: "mymaster",
//		SentinelAddrs: []string{
//			"localhost:26379",
//			"localhost:26380",
//			"localhost:26381",
//		},
//		Password: "secret",
//		DB:       0,
//	})
//
// # Connection Pooling
//
// The Redis client uses connection pooling by default for optimal performance:
//
//	client, err := redis.NewClient(redis.Config{
//		Host:         "localhost",
//		Port:         6379,
//		PoolSize:     10,                // Maximum number of connections
//		MinIdleConns: 5,                 // Minimum idle connections
//		MaxConnAge:   30 * time.Minute,  // Maximum connection age
//		PoolTimeout:  4 * time.Second,   // Timeout when getting connection
//		IdleTimeout:  5 * time.Minute,   // Close idle connections after timeout
//	})
//
// # Health Check
//
//	// Ping to check connection
//	err := client.Ping(ctx)
//	if err != nil {
//		log.Error("Redis is not healthy", err, nil)
//	}
//
//	// Get connection pool stats
//	stats := client.PoolStats()
//	fmt.Printf("Hits: %d, Misses: %d, Timeouts: %d\n",
//		stats.Hits, stats.Misses, stats.Timeouts)
//
// # Caching Pattern with Automatic Serialization
//
//	type User struct {
//		ID    int    `json:"id"`
//		Name  string `json:"name"`
//		Email string `json:"email"`
//	}
//
//	// Set with automatic JSON serialization
//	user := User{ID: 123, Name: "John", Email: "john@example.com"}
//	err := client.SetJSON(ctx, "user:123", user, 10*time.Minute)
//
//	// Get with automatic JSON deserialization
//	var cachedUser User
//	err = client.GetJSON(ctx, "user:123", &cachedUser)
//	if err == redis.Nil {
//		// Cache miss - fetch from database
//		cachedUser = fetchUserFromDB(123)
//		client.SetJSON(ctx, "user:123", cachedUser, 10*time.Minute)
//	}
//
// # Rate Limiting
//
//	// Simple rate limiter using Redis
//	allowed, err := client.RateLimit(ctx, "api:user:123", 100, time.Minute)
//	if err != nil {
//		log.Error("Rate limit check failed", err, nil)
//	}
//	if !allowed {
//		return errors.New("rate limit exceeded")
//	}
//
// # Thread Safety
//
// All methods on the Redis client are safe for concurrent use by multiple
// goroutines. The underlying connection pool handles concurrent access efficiently.
package redis
