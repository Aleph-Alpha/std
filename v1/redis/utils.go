package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// Ping checks if the Redis server is reachable and responsive.
// It returns an error if the connection fails.
func (r *Redis) Ping(ctx context.Context) error {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.client.Ping(ctx).Err()
}

// PoolStats returns connection pool statistics.
// Useful for monitoring connection pool health.
func (r *Redis) PoolStats() *redis.PoolStats {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.client.PoolStats()
}

// Get retrieves the value associated with the given key.
// Returns ErrNil if the key does not exist.
func (r *Redis) Get(ctx context.Context, key string) (string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.client.Get(ctx, key).Result()
}

// Set sets the value for the given key with an optional TTL.
// If ttl is 0, the key will not expire.
func (r *Redis) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.client.Set(ctx, key, value, ttl).Err()
}

// SetNX sets the value for the given key only if the key does not exist.
// Returns true if the key was set, false if it already existed.
func (r *Redis) SetNX(ctx context.Context, key string, value interface{}, ttl time.Duration) (bool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.client.SetNX(ctx, key, value, ttl).Result()
}

// SetEX sets the value for the given key with a TTL (shorthand for Set with TTL).
func (r *Redis) SetEX(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	return r.Set(ctx, key, value, ttl)
}

// GetSet sets the value for the given key and returns the old value.
func (r *Redis) GetSet(ctx context.Context, key string, value interface{}) (string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.client.GetSet(ctx, key, value).Result()
}

// MGet retrieves the values of multiple keys at once.
// Returns a slice of values in the same order as the keys.
// If a key doesn't exist, its value will be nil.
func (r *Redis) MGet(ctx context.Context, keys ...string) ([]interface{}, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.client.MGet(ctx, keys...).Result()
}

// MSet sets multiple key-value pairs at once.
// The values parameter should be in the format: key1, value1, key2, value2, ...
func (r *Redis) MSet(ctx context.Context, values ...interface{}) error {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.client.MSet(ctx, values...).Err()
}

// Delete deletes one or more keys.
// Returns the number of keys that were deleted.
func (r *Redis) Delete(ctx context.Context, keys ...string) (int64, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.client.Del(ctx, keys...).Result()
}

// Exists checks if one or more keys exist.
// Returns the number of keys that exist.
func (r *Redis) Exists(ctx context.Context, keys ...string) (int64, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.client.Exists(ctx, keys...).Result()
}

// Expire sets a timeout on a key.
// After the timeout has expired, the key will be automatically deleted.
func (r *Redis) Expire(ctx context.Context, key string, ttl time.Duration) (bool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.client.Expire(ctx, key, ttl).Result()
}

// ExpireAt sets an expiration timestamp on a key.
// The key will be deleted when the timestamp is reached.
func (r *Redis) ExpireAt(ctx context.Context, key string, tm time.Time) (bool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.client.ExpireAt(ctx, key, tm).Result()
}

// TTL returns the remaining time to live of a key that has a timeout.
// Returns -1 if the key exists but has no associated expire.
// Returns -2 if the key does not exist.
func (r *Redis) TTL(ctx context.Context, key string) (time.Duration, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.client.TTL(ctx, key).Result()
}

// Persist removes the expiration from a key.
func (r *Redis) Persist(ctx context.Context, key string) (bool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.client.Persist(ctx, key).Result()
}

// Incr increments the integer value of a key by one.
// If the key does not exist, it is set to 0 before performing the operation.
func (r *Redis) Incr(ctx context.Context, key string) (int64, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.client.Incr(ctx, key).Result()
}

// IncrBy increments the integer value of a key by the given amount.
func (r *Redis) IncrBy(ctx context.Context, key string, value int64) (int64, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.client.IncrBy(ctx, key, value).Result()
}

// IncrByFloat increments the float value of a key by the given amount.
func (r *Redis) IncrByFloat(ctx context.Context, key string, value float64) (float64, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.client.IncrByFloat(ctx, key, value).Result()
}

// Decr decrements the integer value of a key by one.
func (r *Redis) Decr(ctx context.Context, key string) (int64, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.client.Decr(ctx, key).Result()
}

// DecrBy decrements the integer value of a key by the given amount.
func (r *Redis) DecrBy(ctx context.Context, key string, value int64) (int64, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.client.DecrBy(ctx, key, value).Result()
}

// Keys returns all keys matching the given pattern.
// WARNING: Use with caution in production as it can be slow on large datasets.
// Consider using Scan instead.
func (r *Redis) Keys(ctx context.Context, pattern string) ([]string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.client.Keys(ctx, pattern).Result()
}

// Scan iterates over keys in the database using a cursor.
// This is safer than Keys for large datasets as it doesn't block.
func (r *Redis) Scan(ctx context.Context, cursor uint64, match string, count int64) *redis.ScanIterator {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.client.Scan(ctx, cursor, match, count).Iterator()
}

// Type returns the type of value stored at key.
func (r *Redis) Type(ctx context.Context, key string) (string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.client.Type(ctx, key).Result()
}

// --- Hash Operations ---

// HSet sets field in the hash stored at key to value.
// If the key doesn't exist, a new hash is created.
func (r *Redis) HSet(ctx context.Context, key string, values ...interface{}) (int64, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.client.HSet(ctx, key, values...).Result()
}

// HGet returns the value associated with field in the hash stored at key.
func (r *Redis) HGet(ctx context.Context, key, field string) (string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.client.HGet(ctx, key, field).Result()
}

// HGetAll returns all fields and values in the hash stored at key.
func (r *Redis) HGetAll(ctx context.Context, key string) (map[string]string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.client.HGetAll(ctx, key).Result()
}

// HMGet returns the values associated with the specified fields in the hash.
func (r *Redis) HMGet(ctx context.Context, key string, fields ...string) ([]interface{}, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.client.HMGet(ctx, key, fields...).Result()
}

// HExists checks if a field exists in the hash stored at key.
func (r *Redis) HExists(ctx context.Context, key, field string) (bool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.client.HExists(ctx, key, field).Result()
}

// HDel deletes one or more hash fields.
func (r *Redis) HDel(ctx context.Context, key string, fields ...string) (int64, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.client.HDel(ctx, key, fields...).Result()
}

// HLen returns the number of fields in the hash stored at key.
func (r *Redis) HLen(ctx context.Context, key string) (int64, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.client.HLen(ctx, key).Result()
}

// HKeys returns all field names in the hash stored at key.
func (r *Redis) HKeys(ctx context.Context, key string) ([]string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.client.HKeys(ctx, key).Result()
}

// HVals returns all values in the hash stored at key.
func (r *Redis) HVals(ctx context.Context, key string) ([]string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.client.HVals(ctx, key).Result()
}

// HIncrBy increments the integer value of a hash field by the given number.
func (r *Redis) HIncrBy(ctx context.Context, key, field string, incr int64) (int64, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.client.HIncrBy(ctx, key, field, incr).Result()
}

// HIncrByFloat increments the float value of a hash field by the given amount.
func (r *Redis) HIncrByFloat(ctx context.Context, key, field string, incr float64) (float64, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.client.HIncrByFloat(ctx, key, field, incr).Result()
}

// --- List Operations ---

// LPush inserts all the specified values at the head of the list stored at key.
func (r *Redis) LPush(ctx context.Context, key string, values ...interface{}) (int64, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.client.LPush(ctx, key, values...).Result()
}

// RPush inserts all the specified values at the tail of the list stored at key.
func (r *Redis) RPush(ctx context.Context, key string, values ...interface{}) (int64, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.client.RPush(ctx, key, values...).Result()
}

// LPop removes and returns the first element of the list stored at key.
func (r *Redis) LPop(ctx context.Context, key string) (string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.client.LPop(ctx, key).Result()
}

// RPop removes and returns the last element of the list stored at key.
func (r *Redis) RPop(ctx context.Context, key string) (string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.client.RPop(ctx, key).Result()
}

// LRange returns the specified elements of the list stored at key.
// The offsets start and stop are zero-based indexes.
// Use -1 for the last element, -2 for the second last, etc.
func (r *Redis) LRange(ctx context.Context, key string, start, stop int64) ([]string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.client.LRange(ctx, key, start, stop).Result()
}

// LLen returns the length of the list stored at key.
func (r *Redis) LLen(ctx context.Context, key string) (int64, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.client.LLen(ctx, key).Result()
}

// LRem removes the first count occurrences of elements equal to value from the list.
func (r *Redis) LRem(ctx context.Context, key string, count int64, value interface{}) (int64, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.client.LRem(ctx, key, count, value).Result()
}

// LTrim trims the list to the specified range.
func (r *Redis) LTrim(ctx context.Context, key string, start, stop int64) error {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.client.LTrim(ctx, key, start, stop).Err()
}

// LIndex returns the element at index in the list stored at key.
func (r *Redis) LIndex(ctx context.Context, key string, index int64) (string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.client.LIndex(ctx, key, index).Result()
}

// LSet sets the list element at index to value.
func (r *Redis) LSet(ctx context.Context, key string, index int64, value interface{}) error {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.client.LSet(ctx, key, index, value).Err()
}

// BLPop is a blocking version of LPop with a timeout.
// It blocks until an element is available or the timeout is reached.
func (r *Redis) BLPop(ctx context.Context, timeout time.Duration, keys ...string) ([]string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.client.BLPop(ctx, timeout, keys...).Result()
}

// BRPop is a blocking version of RPop with a timeout.
func (r *Redis) BRPop(ctx context.Context, timeout time.Duration, keys ...string) ([]string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.client.BRPop(ctx, timeout, keys...).Result()
}

// --- Set Operations ---

// SAdd adds the specified members to the set stored at key.
func (r *Redis) SAdd(ctx context.Context, key string, members ...interface{}) (int64, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.client.SAdd(ctx, key, members...).Result()
}

// SRem removes the specified members from the set stored at key.
func (r *Redis) SRem(ctx context.Context, key string, members ...interface{}) (int64, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.client.SRem(ctx, key, members...).Result()
}

// SMembers returns all the members of the set stored at key.
func (r *Redis) SMembers(ctx context.Context, key string) ([]string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.client.SMembers(ctx, key).Result()
}

// SIsMember checks if member is a member of the set stored at key.
func (r *Redis) SIsMember(ctx context.Context, key string, member interface{}) (bool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.client.SIsMember(ctx, key, member).Result()
}

// SCard returns the number of elements in the set stored at key.
func (r *Redis) SCard(ctx context.Context, key string) (int64, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.client.SCard(ctx, key).Result()
}

// SPop removes and returns one or more random members from the set.
func (r *Redis) SPop(ctx context.Context, key string) (string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.client.SPop(ctx, key).Result()
}

// SPopN removes and returns count random members from the set.
func (r *Redis) SPopN(ctx context.Context, key string, count int64) ([]string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.client.SPopN(ctx, key, count).Result()
}

// SRandMember returns one or more random members from the set without removing them.
func (r *Redis) SRandMember(ctx context.Context, key string) (string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.client.SRandMember(ctx, key).Result()
}

// SRandMemberN returns count random members from the set without removing them.
func (r *Redis) SRandMemberN(ctx context.Context, key string, count int64) ([]string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.client.SRandMemberN(ctx, key, count).Result()
}

// SInter returns the members of the set resulting from the intersection of all the given sets.
func (r *Redis) SInter(ctx context.Context, keys ...string) ([]string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.client.SInter(ctx, keys...).Result()
}

// SUnion returns the members of the set resulting from the union of all the given sets.
func (r *Redis) SUnion(ctx context.Context, keys ...string) ([]string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.client.SUnion(ctx, keys...).Result()
}

// SDiff returns the members of the set resulting from the difference between the first set and all the successive sets.
func (r *Redis) SDiff(ctx context.Context, keys ...string) ([]string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.client.SDiff(ctx, keys...).Result()
}

// --- Sorted Set Operations ---

// ZAdd adds all the specified members with the specified scores to the sorted set stored at key.
func (r *Redis) ZAdd(ctx context.Context, key string, members ...redis.Z) (int64, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.client.ZAdd(ctx, key, members...).Result()
}

// ZRem removes the specified members from the sorted set stored at key.
func (r *Redis) ZRem(ctx context.Context, key string, members ...interface{}) (int64, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.client.ZRem(ctx, key, members...).Result()
}

// ZRange returns the specified range of elements in the sorted set stored at key.
// The elements are ordered from the lowest to the highest score.
func (r *Redis) ZRange(ctx context.Context, key string, start, stop int64) ([]string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.client.ZRange(ctx, key, start, stop).Result()
}

// ZRangeWithScores returns the specified range with scores.
func (r *Redis) ZRangeWithScores(ctx context.Context, key string, start, stop int64) ([]redis.Z, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.client.ZRangeWithScores(ctx, key, start, stop).Result()
}

// ZRevRange returns the specified range in reverse order (highest to lowest score).
func (r *Redis) ZRevRange(ctx context.Context, key string, start, stop int64) ([]string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.client.ZRevRange(ctx, key, start, stop).Result()
}

// ZRevRangeWithScores returns the specified range in reverse order with scores.
func (r *Redis) ZRevRangeWithScores(ctx context.Context, key string, start, stop int64) ([]redis.Z, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.client.ZRevRangeWithScores(ctx, key, start, stop).Result()
}

// ZRangeByScore returns all elements in the sorted set with a score between min and max.
func (r *Redis) ZRangeByScore(ctx context.Context, key string, opt *redis.ZRangeBy) ([]string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.client.ZRangeByScore(ctx, key, opt).Result()
}

// ZRevRangeByScore returns all elements in the sorted set with scores between max and min (in reverse order).
func (r *Redis) ZRevRangeByScore(ctx context.Context, key string, opt *redis.ZRangeBy) ([]string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.client.ZRevRangeByScore(ctx, key, opt).Result()
}

// ZScore returns the score of member in the sorted set stored at key.
func (r *Redis) ZScore(ctx context.Context, key, member string) (float64, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.client.ZScore(ctx, key, member).Result()
}

// ZCard returns the number of elements in the sorted set stored at key.
func (r *Redis) ZCard(ctx context.Context, key string) (int64, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.client.ZCard(ctx, key).Result()
}

// ZCount returns the number of elements in the sorted set with a score between min and max.
func (r *Redis) ZCount(ctx context.Context, key, min, max string) (int64, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.client.ZCount(ctx, key, min, max).Result()
}

// ZIncrBy increments the score of member in the sorted set by increment.
func (r *Redis) ZIncrBy(ctx context.Context, key string, increment float64, member string) (float64, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.client.ZIncrBy(ctx, key, increment, member).Result()
}

// ZRank returns the rank of member in the sorted set (0-based, lowest score first).
func (r *Redis) ZRank(ctx context.Context, key, member string) (int64, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.client.ZRank(ctx, key, member).Result()
}

// ZRevRank returns the rank of member in the sorted set (highest score first).
func (r *Redis) ZRevRank(ctx context.Context, key, member string) (int64, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.client.ZRevRank(ctx, key, member).Result()
}

// --- Pub/Sub Operations ---

// Publish posts a message to the given channel.
// Returns the number of clients that received the message.
func (r *Redis) Publish(ctx context.Context, channel string, message interface{}) (int64, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.client.Publish(ctx, channel, message).Result()
}

// Subscribe subscribes to the given channels.
// Returns a PubSub instance that can be used to receive messages.
func (r *Redis) Subscribe(ctx context.Context, channels ...string) *redis.PubSub {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.client.Subscribe(ctx, channels...)
}

// PSubscribe subscribes to channels matching the given patterns.
func (r *Redis) PSubscribe(ctx context.Context, patterns ...string) *redis.PubSub {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.client.PSubscribe(ctx, patterns...)
}

// --- Pipeline Operations ---

// Pipeline returns a new pipeline.
// Pipelines allow sending multiple commands in a single request.
func (r *Redis) Pipeline() redis.Pipeliner {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.client.Pipeline()
}

// TxPipeline returns a new transaction pipeline.
// Commands in a transaction pipeline are wrapped in MULTI/EXEC.
func (r *Redis) TxPipeline() redis.Pipeliner {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.client.TxPipeline()
}

// Watch watches the given keys for changes.
// If any of the watched keys are modified before EXEC, the transaction will fail.
func (r *Redis) Watch(ctx context.Context, fn func(*redis.Tx) error, keys ...string) error {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.client.Watch(ctx, fn, keys...)
}

// --- JSON Helper Methods ---

// SetJSON serializes the value to JSON and stores it in Redis.
func (r *Redis) SetJSON(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}

	return r.Set(ctx, key, data, ttl)
}

// GetJSON retrieves the value from Redis and deserializes it from JSON.
func (r *Redis) GetJSON(ctx context.Context, key string, dest interface{}) error {
	data, err := r.Get(ctx, key)
	if err != nil {
		return err
	}

	if err := json.Unmarshal([]byte(data), dest); err != nil {
		return fmt.Errorf("failed to unmarshal JSON: %w", err)
	}

	return nil
}

// --- Lock Operations ---

// Lock represents a distributed lock.
type Lock struct {
	client *Redis
	key    string
	value  string
	ttl    time.Duration
}

// AcquireLock attempts to acquire a distributed lock.
// Returns a Lock instance if successful, or an error if the lock is already held.
func (r *Redis) AcquireLock(ctx context.Context, key string, ttl time.Duration) (*Lock, error) {
	// Use a unique value to ensure only the lock holder can release it
	value := fmt.Sprintf("%d", time.Now().UnixNano())

	acquired, err := r.SetNX(ctx, key, value, ttl)
	if err != nil {
		return nil, fmt.Errorf("failed to acquire lock: %w", err)
	}

	if !acquired {
		return nil, fmt.Errorf("lock already held")
	}

	return &Lock{
		client: r,
		key:    key,
		value:  value,
		ttl:    ttl,
	}, nil
}

// Release releases the distributed lock.
func (l *Lock) Release(ctx context.Context) error {
	// Use a Lua script to ensure we only delete the lock if we own it
	script := `
		if redis.call("get", KEYS[1]) == ARGV[1] then
			return redis.call("del", KEYS[1])
		else
			return 0
		end
	`

	l.client.mu.RLock()
	defer l.client.mu.RUnlock()

	_, err := l.client.client.Eval(ctx, script, []string{l.key}, l.value).Result()
	return err
}

// Refresh extends the TTL of the lock.
func (l *Lock) Refresh(ctx context.Context) error {
	// Use a Lua script to refresh only if we own the lock
	script := `
		if redis.call("get", KEYS[1]) == ARGV[1] then
			return redis.call("expire", KEYS[1], ARGV[2])
		else
			return 0
		end
	`

	l.client.mu.RLock()
	defer l.client.mu.RUnlock()

	_, err := l.client.client.Eval(ctx, script, []string{l.key}, l.value, int(l.ttl.Seconds())).Result()
	return err
}

// --- Rate Limiting ---

// RateLimit implements a simple rate limiter using Redis.
// Returns true if the operation is allowed, false if rate limit is exceeded.
func (r *Redis) RateLimit(ctx context.Context, key string, limit int64, window time.Duration) (bool, error) {
	// Use a Lua script for atomic rate limiting
	script := `
		local current = redis.call("incr", KEYS[1])
		if current == 1 then
			redis.call("expire", KEYS[1], ARGV[1])
		end
		return current
	`

	r.mu.RLock()
	defer r.mu.RUnlock()

	result, err := r.client.Eval(ctx, script, []string{key}, int(window.Seconds())).Int64()
	if err != nil {
		return false, err
	}

	return result <= limit, nil
}
