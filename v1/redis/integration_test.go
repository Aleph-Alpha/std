package redis

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/go-connections/nat"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"go.uber.org/fx"
)

// TestRedisBasicOperations verifies basic Redis operations work correctly.
func TestRedisBasicOperations(t *testing.T) {
	// Skip if running in short mode
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()

	// Initialize Redis container
	host, port, containerInstance := initializeRedis(ctx, t)
	defer func() {
		if err := containerInstance.Terminate(ctx); err != nil {
			t.Logf("failed to terminate container: %v", err)
		}
	}()

	var client *RedisClient

	cfg := Config{
		Host: host,
		Port: port,
	}

	app := fx.New(
		FXModule,
		fx.Provide(
			func() Config { return cfg },
		),
		fx.Populate(&client),
	)

	require.NoError(t, app.Start(ctx))
	defer app.Stop(ctx)

	// Test basic operations
	t.Run("Set and Get", func(t *testing.T) {
		err := client.Set(ctx, "test-key", "test-value", 0)
		require.NoError(t, err)

		value, err := client.Get(ctx, "test-key")
		require.NoError(t, err)
		assert.Equal(t, "test-value", value)
	})

	t.Run("Delete", func(t *testing.T) {
		err := client.Set(ctx, "delete-key", "value", 0)
		require.NoError(t, err)

		deleted, err := client.Delete(ctx, "delete-key")
		require.NoError(t, err)
		assert.Equal(t, int64(1), deleted)

		_, err = client.Get(ctx, "delete-key")
		assert.Error(t, err)
	})

	t.Run("Exists", func(t *testing.T) {
		err := client.Set(ctx, "exists-key", "value", 0)
		require.NoError(t, err)

		exists, err := client.Exists(ctx, "exists-key")
		require.NoError(t, err)
		assert.Equal(t, int64(1), exists)

		client.Delete(ctx, "exists-key")

		exists, err = client.Exists(ctx, "exists-key")
		require.NoError(t, err)
		assert.Equal(t, int64(0), exists)
	})

	t.Run("Increment", func(t *testing.T) {
		value, err := client.Incr(ctx, "counter")
		require.NoError(t, err)
		assert.Equal(t, int64(1), value)

		value, err = client.IncrBy(ctx, "counter", 5)
		require.NoError(t, err)
		assert.Equal(t, int64(6), value)
	})
}

// TestRedisHashOperations verifies Redis hash operations work correctly.
func TestRedisHashOperations(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()

	host, port, containerInstance := initializeRedis(ctx, t)
	defer func() {
		if err := containerInstance.Terminate(ctx); err != nil {
			t.Logf("failed to terminate container: %v", err)
		}
	}()

	var client *RedisClient

	cfg := Config{
		Host: host,
		Port: port,
	}

	app := fx.New(
		FXModule,
		fx.Provide(
			func() Config { return cfg },
		),
		fx.Populate(&client),
	)

	require.NoError(t, app.Start(ctx))
	defer app.Stop(ctx)

	t.Run("HSet and HGet", func(t *testing.T) {
		_, err := client.HSet(ctx, "user:1", "name", "John", "age", "30")
		require.NoError(t, err)

		name, err := client.HGet(ctx, "user:1", "name")
		require.NoError(t, err)
		assert.Equal(t, "John", name)

		age, err := client.HGet(ctx, "user:1", "age")
		require.NoError(t, err)
		assert.Equal(t, "30", age)
	})

	t.Run("HGetAll", func(t *testing.T) {
		user, err := client.HGetAll(ctx, "user:1")
		require.NoError(t, err)
		assert.Equal(t, "John", user["name"])
		assert.Equal(t, "30", user["age"])
	})

	t.Run("HDel", func(t *testing.T) {
		deleted, err := client.HDel(ctx, "user:1", "age")
		require.NoError(t, err)
		assert.Equal(t, int64(1), deleted)

		exists, err := client.HExists(ctx, "user:1", "age")
		require.NoError(t, err)
		assert.False(t, exists)
	})
}

// TestRedisListOperations verifies Redis list operations work correctly.
func TestRedisListOperations(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()

	host, port, containerInstance := initializeRedis(ctx, t)
	defer func() {
		if err := containerInstance.Terminate(ctx); err != nil {
			t.Logf("failed to terminate container: %v", err)
		}
	}()

	var client *RedisClient

	cfg := Config{
		Host: host,
		Port: port,
	}

	app := fx.New(
		FXModule,
		fx.Provide(
			func() Config { return cfg },
		),
		fx.Populate(&client),
	)

	require.NoError(t, app.Start(ctx))
	defer app.Stop(ctx)

	t.Run("LPush and LRange", func(t *testing.T) {
		_, err := client.LPush(ctx, "tasks", "task3", "task2", "task1")
		require.NoError(t, err)

		tasks, err := client.LRange(ctx, "tasks", 0, -1)
		require.NoError(t, err)
		assert.Equal(t, []string{"task1", "task2", "task3"}, tasks)
	})

	t.Run("RPush and LPop", func(t *testing.T) {
		_, err := client.RPush(ctx, "queue", "item1", "item2")
		require.NoError(t, err)

		item, err := client.LPop(ctx, "queue")
		require.NoError(t, err)
		assert.Equal(t, "item1", item)
	})

	t.Run("LLen", func(t *testing.T) {
		length, err := client.LLen(ctx, "queue")
		require.NoError(t, err)
		assert.Equal(t, int64(1), length)
	})
}

// TestRedisSetOperations verifies Redis set operations work correctly.
func TestRedisSetOperations(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()

	host, port, containerInstance := initializeRedis(ctx, t)
	defer func() {
		if err := containerInstance.Terminate(ctx); err != nil {
			t.Logf("failed to terminate container: %v", err)
		}
	}()

	var client *RedisClient

	cfg := Config{
		Host: host,
		Port: port,
	}

	app := fx.New(
		FXModule,
		fx.Provide(
			func() Config { return cfg },
		),
		fx.Populate(&client),
	)

	require.NoError(t, app.Start(ctx))
	defer app.Stop(ctx)

	t.Run("SAdd and SMembers", func(t *testing.T) {
		_, err := client.SAdd(ctx, "tags", "redis", "cache", "database")
		require.NoError(t, err)

		members, err := client.SMembers(ctx, "tags")
		require.NoError(t, err)
		assert.Len(t, members, 3)
		assert.Contains(t, members, "redis")
		assert.Contains(t, members, "cache")
		assert.Contains(t, members, "database")
	})

	t.Run("SIsMember", func(t *testing.T) {
		isMember, err := client.SIsMember(ctx, "tags", "redis")
		require.NoError(t, err)
		assert.True(t, isMember)

		isMember, err = client.SIsMember(ctx, "tags", "notfound")
		require.NoError(t, err)
		assert.False(t, isMember)
	})

	t.Run("SCard", func(t *testing.T) {
		count, err := client.SCard(ctx, "tags")
		require.NoError(t, err)
		assert.Equal(t, int64(3), count)
	})
}

// TestRedisJSONOperations verifies JSON serialization works correctly.
func TestRedisJSONOperations(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()

	host, port, containerInstance := initializeRedis(ctx, t)
	defer func() {
		if err := containerInstance.Terminate(ctx); err != nil {
			t.Logf("failed to terminate container: %v", err)
		}
	}()

	var client *RedisClient

	cfg := Config{
		Host: host,
		Port: port,
	}

	app := fx.New(
		FXModule,
		fx.Provide(
			func() Config { return cfg },
		),
		fx.Populate(&client),
	)

	require.NoError(t, app.Start(ctx))
	defer app.Stop(ctx)

	type User struct {
		ID    int    `json:"id"`
		Name  string `json:"name"`
		Email string `json:"email"`
	}

	t.Run("SetJSON and GetJSON", func(t *testing.T) {
		user := User{ID: 123, Name: "John", Email: "john@example.com"}
		err := client.SetJSON(ctx, "user:123", user, 5*time.Minute)
		require.NoError(t, err)

		var cachedUser User
		err = client.GetJSON(ctx, "user:123", &cachedUser)
		require.NoError(t, err)
		assert.Equal(t, user, cachedUser)
	})
}

// TestRedisLocking verifies distributed locking works correctly.
func TestRedisLocking(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()

	host, port, containerInstance := initializeRedis(ctx, t)
	defer func() {
		if err := containerInstance.Terminate(ctx); err != nil {
			t.Logf("failed to terminate container: %v", err)
		}
	}()

	var client *RedisClient

	cfg := Config{
		Host: host,
		Port: port,
	}

	app := fx.New(
		FXModule,
		fx.Provide(
			func() Config { return cfg },
		),
		fx.Populate(&client),
	)

	require.NoError(t, app.Start(ctx))
	defer app.Stop(ctx)

	t.Run("AcquireLock and Release", func(t *testing.T) {
		lock, err := client.AcquireLock(ctx, "resource:123", 5*time.Second)
		require.NoError(t, err)
		assert.NotNil(t, lock)

		// Try to acquire the same lock should fail
		_, err = client.AcquireLock(ctx, "resource:123", 5*time.Second)
		assert.Error(t, err)

		// Release the lock
		err = lock.Release(ctx)
		require.NoError(t, err)

		// Now we should be able to acquire it again
		lock2, err := client.AcquireLock(ctx, "resource:123", 5*time.Second)
		require.NoError(t, err)
		assert.NotNil(t, lock2)
		lock2.Release(ctx)
	})
}

// TestRedisTTL verifies TTL operations work correctly.
func TestRedisTTL(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()

	host, port, containerInstance := initializeRedis(ctx, t)
	defer func() {
		if err := containerInstance.Terminate(ctx); err != nil {
			t.Logf("failed to terminate container: %v", err)
		}
	}()

	var client *RedisClient

	cfg := Config{
		Host: host,
		Port: port,
	}

	app := fx.New(
		FXModule,
		fx.Provide(
			func() Config { return cfg },
		),
		fx.Populate(&client),
	)

	require.NoError(t, app.Start(ctx))
	defer app.Stop(ctx)

	t.Run("Set with TTL", func(t *testing.T) {
		err := client.Set(ctx, "expiring-key", "value", 2*time.Second)
		require.NoError(t, err)

		// Key should exist
		exists, err := client.Exists(ctx, "expiring-key")
		require.NoError(t, err)
		assert.Equal(t, int64(1), exists)

		// Check TTL
		ttl, err := client.TTL(ctx, "expiring-key")
		require.NoError(t, err)
		assert.Greater(t, ttl, time.Duration(0))

		// Wait for expiration
		time.Sleep(3 * time.Second)

		// Key should be gone
		exists, err = client.Exists(ctx, "expiring-key")
		require.NoError(t, err)
		assert.Equal(t, int64(0), exists)
	})

	t.Run("Expire and Persist", func(t *testing.T) {
		err := client.Set(ctx, "persist-key", "value", 0)
		require.NoError(t, err)

		// Set expiration
		success, err := client.Expire(ctx, "persist-key", 10*time.Second)
		require.NoError(t, err)
		assert.True(t, success)

		// Check TTL exists
		ttl, err := client.TTL(ctx, "persist-key")
		require.NoError(t, err)
		assert.Greater(t, ttl, time.Duration(0))

		// Remove expiration
		success, err = client.Persist(ctx, "persist-key")
		require.NoError(t, err)
		assert.True(t, success)

		// Check no TTL
		ttl, err = client.TTL(ctx, "persist-key")
		require.NoError(t, err)
		assert.Equal(t, time.Duration(-1), ttl)
	})
}

// TestRedisPipeline verifies pipeline operations work correctly.
func TestRedisPipeline(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()

	host, port, containerInstance := initializeRedis(ctx, t)
	defer func() {
		if err := containerInstance.Terminate(ctx); err != nil {
			t.Logf("failed to terminate container: %v", err)
		}
	}()

	var client *RedisClient

	cfg := Config{
		Host: host,
		Port: port,
	}

	app := fx.New(
		FXModule,
		fx.Provide(
			func() Config { return cfg },
		),
		fx.Populate(&client),
	)

	require.NoError(t, app.Start(ctx))
	defer app.Stop(ctx)

	t.Run("Pipeline execution", func(t *testing.T) {
		pipe := client.Pipeline()

		pipe.Set(ctx, "pipe-key1", "value1", 0)
		pipe.Set(ctx, "pipe-key2", "value2", 0)
		pipe.Incr(ctx, "pipe-counter")

		_, err := pipe.Exec(ctx)
		require.NoError(t, err)

		// Verify values were set
		value, err := client.Get(ctx, "pipe-key1")
		require.NoError(t, err)
		assert.Equal(t, "value1", value)

		value, err = client.Get(ctx, "pipe-key2")
		require.NoError(t, err)
		assert.Equal(t, "value2", value)
	})
}

// TestRedisConcurrency verifies concurrent operations work correctly.
func TestRedisConcurrency(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()

	host, port, containerInstance := initializeRedis(ctx, t)
	defer func() {
		if err := containerInstance.Terminate(ctx); err != nil {
			t.Logf("failed to terminate container: %v", err)
		}
	}()

	var client *RedisClient

	cfg := Config{
		Host: host,
		Port: port,
	}

	app := fx.New(
		FXModule,
		fx.Provide(
			func() Config { return cfg },
		),
		fx.Populate(&client),
	)

	require.NoError(t, app.Start(ctx))
	defer app.Stop(ctx)

	t.Run("Concurrent increments", func(t *testing.T) {
		var wg sync.WaitGroup
		concurrency := 100

		for i := 0; i < concurrency; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_, err := client.Incr(ctx, "concurrent-counter")
				assert.NoError(t, err)
			}()
		}

		wg.Wait()

		// Check final value
		value, err := client.Get(ctx, "concurrent-counter")
		require.NoError(t, err)
		assert.Equal(t, "100", value)
	})
}

// Helper functions

func initializeRedis(ctx context.Context, t *testing.T) (string, int, testcontainers.Container) {
	hostPort, err := getFreePort()
	require.NoError(t, err)

	containerInstance, err := createRedisContainer(ctx, hostPort)
	require.NoError(t, err)

	port, err := containerInstance.MappedPort(ctx, "6379")
	require.NoError(t, err)

	host, err := containerInstance.Host(ctx)
	require.NoError(t, err)

	// Wait for Redis to be ready
	require.Eventually(t, func() bool {
		conn, err := net.DialTimeout("tcp", net.JoinHostPort(host, port.Port()), 2*time.Second)
		if err != nil {
			return false
		}
		_ = conn.Close()
		return true
	}, 30*time.Second, 500*time.Millisecond, "Redis port not ready")

	return host, port.Int(), containerInstance
}

func createRedisContainer(ctx context.Context, hostPort string) (testcontainers.Container, error) {
	portBindings := nat.PortMap{
		"6379/tcp": []nat.PortBinding{{HostPort: hostPort}},
	}

	req := testcontainers.ContainerRequest{
		Image: "redis:7-alpine",
		ExposedPorts: []string{
			"6379/tcp",
		},
		HostConfigModifier: func(cfg *container.HostConfig) {
			cfg.PortBindings = portBindings
		},
		WaitingFor: wait.ForAll(
			wait.ForListeningPort("6379/tcp").WithStartupTimeout(30*time.Second),
			wait.ForLog("Ready to accept connections").WithStartupTimeout(30*time.Second),
		),
	}

	var containerInstance testcontainers.Container
	var lastErr error

	for attempt := 0; attempt < 3; attempt++ {
		containerInstance, lastErr = testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
			ContainerRequest: req,
			Started:          true,
		})
		if lastErr == nil {
			return containerInstance, nil
		}

		if strings.Contains(lastErr.Error(), "docker.sock") {
			time.Sleep(time.Duration(attempt+1) * time.Second)
			continue
		}

		break
	}

	return nil, fmt.Errorf("failed to start Redis container after 3 attempts: %w", lastErr)
}

func getFreePort() (string, error) {
	l, err := net.Listen("tcp", ":0")
	if err != nil {
		return "", err
	}
	defer l.Close()
	addr := l.Addr().(*net.TCPAddr)
	return strconv.Itoa(addr.Port), nil
}
