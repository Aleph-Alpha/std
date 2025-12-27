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
func (r *RedisClient) Ping(ctx context.Context) error {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.client.Ping(ctx).Err()
}

// PoolStats returns connection pool statistics.
// Useful for monitoring connection pool health.
func (r *RedisClient) PoolStats() *redis.PoolStats {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.client.PoolStats()
}

// Get retrieves the value associated with the given key.
// Returns ErrNil if the key does not exist.
func (r *RedisClient) Get(ctx context.Context, key string) (string, error) {
	start := time.Now()
	r.mu.RLock()
	defer r.mu.RUnlock()

	result, err := r.client.Get(ctx, key).Result()
	r.observeOperation("get", key, "", time.Since(start), err, int64(len(result)), nil)
	return result, err
}

// Set sets the value for the given key with an optional TTL.
// If ttl is 0, the key will not expire.
func (r *RedisClient) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	start := time.Now()
	r.mu.RLock()
	defer r.mu.RUnlock()

	err := r.client.Set(ctx, key, value, ttl).Err()
	metadata := map[string]interface{}{}
	if ttl > 0 {
		metadata["ttl"] = ttl.String()
	}
	r.observeOperation("set", key, "", time.Since(start), err, 0, metadata)
	return err
}

// SetNX sets the value for the given key only if the key does not exist.
// Returns true if the key was set, false if it already existed.
func (r *RedisClient) SetNX(ctx context.Context, key string, value interface{}, ttl time.Duration) (bool, error) {
	start := time.Now()
	r.mu.RLock()
	defer r.mu.RUnlock()

	result, err := r.client.SetNX(ctx, key, value, ttl).Result()
	metadata := map[string]interface{}{"was_set": result}
	if ttl > 0 {
		metadata["ttl"] = ttl.String()
	}
	r.observeOperation("setnx", key, "", time.Since(start), err, 0, metadata)
	return result, err
}

// SetEX sets the value for the given key with a TTL (shorthand for Set with TTL).
func (r *RedisClient) SetEX(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	return r.Set(ctx, key, value, ttl)
}

// GetSet sets the value for the given key and returns the old value.
func (r *RedisClient) GetSet(ctx context.Context, key string, value interface{}) (string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.client.GetSet(ctx, key, value).Result()
}

// MGet retrieves the values of multiple keys at once.
// Returns a slice of values in the same order as the keys.
// If a key doesn't exist, its value will be nil.
func (r *RedisClient) MGet(ctx context.Context, keys ...string) ([]interface{}, error) {
	start := time.Now()
	r.mu.RLock()
	defer r.mu.RUnlock()

	result, err := r.client.MGet(ctx, keys...).Result()
	resource := ""
	if len(keys) > 0 {
		resource = keys[0]
	}
	r.observeOperation("mget", resource, "", time.Since(start), err, int64(len(result)), map[string]interface{}{
		"key_count": len(keys),
	})
	return result, err
}

// MSet sets multiple key-value pairs at once.
// The values parameter should be in the format: key1, value1, key2, value2, ...
func (r *RedisClient) MSet(ctx context.Context, values ...interface{}) error {
	start := time.Now()
	r.mu.RLock()
	defer r.mu.RUnlock()

	err := r.client.MSet(ctx, values...).Err()
	r.observeOperation("mset", "", "", time.Since(start), err, 0, map[string]interface{}{
		"pair_count": len(values) / 2,
	})
	return err
}

// Delete deletes one or more keys.
// Returns the number of keys that were deleted.
func (r *RedisClient) Delete(ctx context.Context, keys ...string) (int64, error) {
	start := time.Now()
	r.mu.RLock()
	defer r.mu.RUnlock()

	result, err := r.client.Del(ctx, keys...).Result()
	resource := ""
	if len(keys) > 0 {
		resource = keys[0]
	}
	r.observeOperation("delete", resource, "", time.Since(start), err, result, map[string]interface{}{
		"key_count": len(keys),
	})
	return result, err
}

// Exists checks if one or more keys exist.
// Returns the number of keys that exist.
func (r *RedisClient) Exists(ctx context.Context, keys ...string) (int64, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.client.Exists(ctx, keys...).Result()
}

// Expire sets a timeout on a key.
// After the timeout has expired, the key will be automatically deleted.
func (r *RedisClient) Expire(ctx context.Context, key string, ttl time.Duration) (bool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.client.Expire(ctx, key, ttl).Result()
}

// ExpireAt sets an expiration timestamp on a key.
// The key will be deleted when the timestamp is reached.
func (r *RedisClient) ExpireAt(ctx context.Context, key string, tm time.Time) (bool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.client.ExpireAt(ctx, key, tm).Result()
}

// TTL returns the remaining time to live of a key that has a timeout.
// Returns -1 if the key exists but has no associated expire.
// Returns -2 if the key does not exist.
func (r *RedisClient) TTL(ctx context.Context, key string) (time.Duration, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.client.TTL(ctx, key).Result()
}

// Persist removes the expiration from a key.
func (r *RedisClient) Persist(ctx context.Context, key string) (bool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.client.Persist(ctx, key).Result()
}

// Incr increments the integer value of a key by one.
// If the key does not exist, it is set to 0 before performing the operation.
func (r *RedisClient) Incr(ctx context.Context, key string) (int64, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.client.Incr(ctx, key).Result()
}

// IncrBy increments the integer value of a key by the given amount.
func (r *RedisClient) IncrBy(ctx context.Context, key string, value int64) (int64, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.client.IncrBy(ctx, key, value).Result()
}

// IncrByFloat increments the float value of a key by the given amount.
func (r *RedisClient) IncrByFloat(ctx context.Context, key string, value float64) (float64, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.client.IncrByFloat(ctx, key, value).Result()
}

// Decr decrements the integer value of a key by one.
func (r *RedisClient) Decr(ctx context.Context, key string) (int64, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.client.Decr(ctx, key).Result()
}

// DecrBy decrements the integer value of a key by the given amount.
func (r *RedisClient) DecrBy(ctx context.Context, key string, value int64) (int64, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.client.DecrBy(ctx, key, value).Result()
}

// Keys returns all keys matching the given pattern.
// WARNING: Use with caution in production as it can be slow on large datasets.
// Consider using Scan instead.
func (r *RedisClient) Keys(ctx context.Context, pattern string) ([]string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.client.Keys(ctx, pattern).Result()
}

// Scan iterates over keys in the database using a cursor.
// This is safer than Keys for large datasets as it doesn't block.
func (r *RedisClient) Scan(ctx context.Context, cursor uint64, match string, count int64) *redis.ScanIterator {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.client.Scan(ctx, cursor, match, count).Iterator()
}

// Type returns the type of value stored at key.
func (r *RedisClient) Type(ctx context.Context, key string) (string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.client.Type(ctx, key).Result()
}

// --- Hash Operations ---

// HSet sets field in the hash stored at key to value.
// If the key doesn't exist, a new hash is created.
func (r *RedisClient) HSet(ctx context.Context, key string, values ...interface{}) (int64, error) {
	start := time.Now()
	r.mu.RLock()
	defer r.mu.RUnlock()

	result, err := r.client.HSet(ctx, key, values...).Result()
	r.observeOperation("hset", key, "", time.Since(start), err, result, map[string]interface{}{
		"field_count": len(values) / 2,
	})
	return result, err
}

// HGet returns the value associated with field in the hash stored at key.
func (r *RedisClient) HGet(ctx context.Context, key, field string) (string, error) {
	start := time.Now()
	r.mu.RLock()
	defer r.mu.RUnlock()

	result, err := r.client.HGet(ctx, key, field).Result()
	r.observeOperation("hget", key, field, time.Since(start), err, int64(len(result)), nil)
	return result, err
}

// HGetAll returns all fields and values in the hash stored at key.
func (r *RedisClient) HGetAll(ctx context.Context, key string) (map[string]string, error) {
	start := time.Now()
	r.mu.RLock()
	defer r.mu.RUnlock()

	result, err := r.client.HGetAll(ctx, key).Result()
	r.observeOperation("hgetall", key, "", time.Since(start), err, int64(len(result)), nil)
	return result, err
}

// HMGet returns the values associated with the specified fields in the hash.
func (r *RedisClient) HMGet(ctx context.Context, key string, fields ...string) ([]interface{}, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.client.HMGet(ctx, key, fields...).Result()
}

// HExists checks if a field exists in the hash stored at key.
func (r *RedisClient) HExists(ctx context.Context, key, field string) (bool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.client.HExists(ctx, key, field).Result()
}

// HDel deletes one or more hash fields.
func (r *RedisClient) HDel(ctx context.Context, key string, fields ...string) (int64, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.client.HDel(ctx, key, fields...).Result()
}

// HLen returns the number of fields in the hash stored at key.
func (r *RedisClient) HLen(ctx context.Context, key string) (int64, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.client.HLen(ctx, key).Result()
}

// HKeys returns all field names in the hash stored at key.
func (r *RedisClient) HKeys(ctx context.Context, key string) ([]string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.client.HKeys(ctx, key).Result()
}

// HVals returns all values in the hash stored at key.
func (r *RedisClient) HVals(ctx context.Context, key string) ([]string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.client.HVals(ctx, key).Result()
}

// HIncrBy increments the integer value of a hash field by the given number.
func (r *RedisClient) HIncrBy(ctx context.Context, key, field string, incr int64) (int64, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.client.HIncrBy(ctx, key, field, incr).Result()
}

// HIncrByFloat increments the float value of a hash field by the given amount.
func (r *RedisClient) HIncrByFloat(ctx context.Context, key, field string, incr float64) (float64, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.client.HIncrByFloat(ctx, key, field, incr).Result()
}

// --- List Operations ---

// LPush inserts all the specified values at the head of the list stored at key.
func (r *RedisClient) LPush(ctx context.Context, key string, values ...interface{}) (int64, error) {
	start := time.Now()
	r.mu.RLock()
	defer r.mu.RUnlock()

	result, err := r.client.LPush(ctx, key, values...).Result()
	r.observeOperation("lpush", key, "", time.Since(start), err, result, map[string]interface{}{
		"value_count": len(values),
	})
	return result, err
}

// RPush inserts all the specified values at the tail of the list stored at key.
func (r *RedisClient) RPush(ctx context.Context, key string, values ...interface{}) (int64, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.client.RPush(ctx, key, values...).Result()
}

// LPop removes and returns the first element of the list stored at key.
func (r *RedisClient) LPop(ctx context.Context, key string) (string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.client.LPop(ctx, key).Result()
}

// RPop removes and returns the last element of the list stored at key.
func (r *RedisClient) RPop(ctx context.Context, key string) (string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.client.RPop(ctx, key).Result()
}

// LRange returns the specified elements of the list stored at key.
// The offsets start and stop are zero-based indexes.
// Use -1 for the last element, -2 for the second last, etc.
func (r *RedisClient) LRange(ctx context.Context, key string, start, stop int64) ([]string, error) {
	startTime := time.Now()
	r.mu.RLock()
	defer r.mu.RUnlock()

	result, err := r.client.LRange(ctx, key, start, stop).Result()
	r.observeOperation("lrange", key, "", time.Since(startTime), err, int64(len(result)), map[string]interface{}{
		"start": start,
		"stop":  stop,
	})
	return result, err
}

// LLen returns the length of the list stored at key.
func (r *RedisClient) LLen(ctx context.Context, key string) (int64, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.client.LLen(ctx, key).Result()
}

// LRem removes the first count occurrences of elements equal to value from the list.
func (r *RedisClient) LRem(ctx context.Context, key string, count int64, value interface{}) (int64, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.client.LRem(ctx, key, count, value).Result()
}

// LTrim trims the list to the specified range.
func (r *RedisClient) LTrim(ctx context.Context, key string, start, stop int64) error {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.client.LTrim(ctx, key, start, stop).Err()
}

// LIndex returns the element at index in the list stored at key.
func (r *RedisClient) LIndex(ctx context.Context, key string, index int64) (string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.client.LIndex(ctx, key, index).Result()
}

// LSet sets the list element at index to value.
func (r *RedisClient) LSet(ctx context.Context, key string, index int64, value interface{}) error {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.client.LSet(ctx, key, index, value).Err()
}

// BLPop is a blocking version of LPop with a timeout.
// It blocks until an element is available or the timeout is reached.
func (r *RedisClient) BLPop(ctx context.Context, timeout time.Duration, keys ...string) ([]string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.client.BLPop(ctx, timeout, keys...).Result()
}

// BRPop is a blocking version of RPop with a timeout.
func (r *RedisClient) BRPop(ctx context.Context, timeout time.Duration, keys ...string) ([]string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.client.BRPop(ctx, timeout, keys...).Result()
}

// --- Set Operations ---

// SAdd adds the specified members to the set stored at key.
func (r *RedisClient) SAdd(ctx context.Context, key string, members ...interface{}) (int64, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.client.SAdd(ctx, key, members...).Result()
}

// SRem removes the specified members from the set stored at key.
func (r *RedisClient) SRem(ctx context.Context, key string, members ...interface{}) (int64, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.client.SRem(ctx, key, members...).Result()
}

// SMembers returns all the members of the set stored at key.
func (r *RedisClient) SMembers(ctx context.Context, key string) ([]string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.client.SMembers(ctx, key).Result()
}

// SIsMember checks if member is a member of the set stored at key.
func (r *RedisClient) SIsMember(ctx context.Context, key string, member interface{}) (bool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.client.SIsMember(ctx, key, member).Result()
}

// SCard returns the number of elements in the set stored at key.
func (r *RedisClient) SCard(ctx context.Context, key string) (int64, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.client.SCard(ctx, key).Result()
}

// SPop removes and returns one or more random members from the set.
func (r *RedisClient) SPop(ctx context.Context, key string) (string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.client.SPop(ctx, key).Result()
}

// SPopN removes and returns count random members from the set.
func (r *RedisClient) SPopN(ctx context.Context, key string, count int64) ([]string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.client.SPopN(ctx, key, count).Result()
}

// SRandMember returns one or more random members from the set without removing them.
func (r *RedisClient) SRandMember(ctx context.Context, key string) (string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.client.SRandMember(ctx, key).Result()
}

// SRandMemberN returns count random members from the set without removing them.
func (r *RedisClient) SRandMemberN(ctx context.Context, key string, count int64) ([]string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.client.SRandMemberN(ctx, key, count).Result()
}

// SInter returns the members of the set resulting from the intersection of all the given sets.
func (r *RedisClient) SInter(ctx context.Context, keys ...string) ([]string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.client.SInter(ctx, keys...).Result()
}

// SUnion returns the members of the set resulting from the union of all the given sets.
func (r *RedisClient) SUnion(ctx context.Context, keys ...string) ([]string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.client.SUnion(ctx, keys...).Result()
}

// SDiff returns the members of the set resulting from the difference between the first set and all the successive sets.
func (r *RedisClient) SDiff(ctx context.Context, keys ...string) ([]string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.client.SDiff(ctx, keys...).Result()
}

// --- Sorted Set Operations ---

// ZAdd adds all the specified members with the specified scores to the sorted set stored at key.
func (r *RedisClient) ZAdd(ctx context.Context, key string, members ...redis.Z) (int64, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.client.ZAdd(ctx, key, members...).Result()
}

// ZRem removes the specified members from the sorted set stored at key.
func (r *RedisClient) ZRem(ctx context.Context, key string, members ...interface{}) (int64, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.client.ZRem(ctx, key, members...).Result()
}

// ZRange returns the specified range of elements in the sorted set stored at key.
// The elements are ordered from the lowest to the highest score.
func (r *RedisClient) ZRange(ctx context.Context, key string, start, stop int64) ([]string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.client.ZRange(ctx, key, start, stop).Result()
}

// ZRangeWithScores returns the specified range with scores.
func (r *RedisClient) ZRangeWithScores(ctx context.Context, key string, start, stop int64) ([]redis.Z, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.client.ZRangeWithScores(ctx, key, start, stop).Result()
}

// ZRevRange returns the specified range in reverse order (highest to lowest score).
func (r *RedisClient) ZRevRange(ctx context.Context, key string, start, stop int64) ([]string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.client.ZRevRange(ctx, key, start, stop).Result()
}

// ZRevRangeWithScores returns the specified range in reverse order with scores.
func (r *RedisClient) ZRevRangeWithScores(ctx context.Context, key string, start, stop int64) ([]redis.Z, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.client.ZRevRangeWithScores(ctx, key, start, stop).Result()
}

// ZRangeByScore returns all elements in the sorted set with a score between min and max.
func (r *RedisClient) ZRangeByScore(ctx context.Context, key string, opt *redis.ZRangeBy) ([]string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.client.ZRangeByScore(ctx, key, opt).Result()
}

// ZRevRangeByScore returns all elements in the sorted set with scores between max and min (in reverse order).
func (r *RedisClient) ZRevRangeByScore(ctx context.Context, key string, opt *redis.ZRangeBy) ([]string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.client.ZRevRangeByScore(ctx, key, opt).Result()
}

// ZScore returns the score of member in the sorted set stored at key.
func (r *RedisClient) ZScore(ctx context.Context, key, member string) (float64, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.client.ZScore(ctx, key, member).Result()
}

// ZCard returns the number of elements in the sorted set stored at key.
func (r *RedisClient) ZCard(ctx context.Context, key string) (int64, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.client.ZCard(ctx, key).Result()
}

// ZCount returns the number of elements in the sorted set with a score between min and max.
func (r *RedisClient) ZCount(ctx context.Context, key, min, max string) (int64, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.client.ZCount(ctx, key, min, max).Result()
}

// ZIncrBy increments the score of member in the sorted set by increment.
func (r *RedisClient) ZIncrBy(ctx context.Context, key string, increment float64, member string) (float64, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.client.ZIncrBy(ctx, key, increment, member).Result()
}

// ZRank returns the rank of member in the sorted set (0-based, lowest score first).
func (r *RedisClient) ZRank(ctx context.Context, key, member string) (int64, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.client.ZRank(ctx, key, member).Result()
}

// ZRevRank returns the rank of member in the sorted set (highest score first).
func (r *RedisClient) ZRevRank(ctx context.Context, key, member string) (int64, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.client.ZRevRank(ctx, key, member).Result()
}

// --- Pub/Sub Operations ---

// Publish posts a message to the given channel.
// Returns the number of clients that received the message.
func (r *RedisClient) Publish(ctx context.Context, channel string, message interface{}) (int64, error) {
	start := time.Now()
	r.mu.RLock()
	defer r.mu.RUnlock()

	result, err := r.client.Publish(ctx, channel, message).Result()
	r.observeOperation("publish", channel, "", time.Since(start), err, result, nil)
	return result, err
}

// Subscribe subscribes to the given channels.
// Returns a PubSub instance that can be used to receive messages.
func (r *RedisClient) Subscribe(ctx context.Context, channels ...string) *redis.PubSub {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.client.Subscribe(ctx, channels...)
}

// PSubscribe subscribes to channels matching the given patterns.
func (r *RedisClient) PSubscribe(ctx context.Context, patterns ...string) *redis.PubSub {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.client.PSubscribe(ctx, patterns...)
}

// --- Pipeline Operations ---

// Pipeline returns a new pipeline.
// Pipelines allow sending multiple commands in a single request.
func (r *RedisClient) Pipeline() redis.Pipeliner {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.client.Pipeline()
}

// TxPipeline returns a new transaction pipeline.
// Commands in a transaction pipeline are wrapped in MULTI/EXEC.
func (r *RedisClient) TxPipeline() redis.Pipeliner {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.client.TxPipeline()
}

// Watch watches the given keys for changes.
// If any of the watched keys are modified before EXEC, the transaction will fail.
func (r *RedisClient) Watch(ctx context.Context, fn func(*redis.Tx) error, keys ...string) error {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.client.Watch(ctx, fn, keys...)
}

// --- JSON Helper Methods ---

// SetJSON serializes the value to JSON and stores it in Redis.
func (r *RedisClient) SetJSON(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}

	return r.Set(ctx, key, data, ttl)
}

// GetJSON retrieves the value from Redis and deserializes it from JSON.
func (r *RedisClient) GetJSON(ctx context.Context, key string, dest interface{}) error {
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
	client *RedisClient
	key    string
	value  string
	ttl    time.Duration
}

// AcquireLock attempts to acquire a distributed lock.
// Returns a Lock instance if successful, or an error if the lock is already held.
func (r *RedisClient) AcquireLock(ctx context.Context, key string, ttl time.Duration) (*Lock, error) {
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
func (r *RedisClient) RateLimit(ctx context.Context, key string, limit int64, window time.Duration) (bool, error) {
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
