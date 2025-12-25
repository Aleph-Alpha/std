package redis

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

// Client provides a high-level interface for interacting with Redis.
// It abstracts Redis operations with support for strings, hashes, lists, sets, sorted sets,
// pub/sub, transactions, and advanced features like distributed locks and rate limiting.
//
// This interface is implemented by the concrete *RedisClient type.
type Client interface {
	// Connection and lifecycle
	Ping(ctx context.Context) error
	PoolStats() *redis.PoolStats
	Client() redis.UniversalClient
	Close() error

	// String operations
	Get(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error
	SetNX(ctx context.Context, key string, value interface{}, ttl time.Duration) (bool, error)
	SetEX(ctx context.Context, key string, value interface{}, ttl time.Duration) error
	GetSet(ctx context.Context, key string, value interface{}) (string, error)
	MGet(ctx context.Context, keys ...string) ([]interface{}, error)
	MSet(ctx context.Context, values ...interface{}) error

	// Key operations
	Delete(ctx context.Context, keys ...string) (int64, error)
	Exists(ctx context.Context, keys ...string) (int64, error)
	Expire(ctx context.Context, key string, ttl time.Duration) (bool, error)
	ExpireAt(ctx context.Context, key string, tm time.Time) (bool, error)
	TTL(ctx context.Context, key string) (time.Duration, error)
	Persist(ctx context.Context, key string) (bool, error)
	Keys(ctx context.Context, pattern string) ([]string, error)
	Scan(ctx context.Context, cursor uint64, match string, count int64) *redis.ScanIterator
	Type(ctx context.Context, key string) (string, error)

	// Numeric operations
	Incr(ctx context.Context, key string) (int64, error)
	IncrBy(ctx context.Context, key string, value int64) (int64, error)
	IncrByFloat(ctx context.Context, key string, value float64) (float64, error)
	Decr(ctx context.Context, key string) (int64, error)
	DecrBy(ctx context.Context, key string, value int64) (int64, error)

	// Hash operations
	HSet(ctx context.Context, key string, values ...interface{}) (int64, error)
	HGet(ctx context.Context, key, field string) (string, error)
	HGetAll(ctx context.Context, key string) (map[string]string, error)
	HMGet(ctx context.Context, key string, fields ...string) ([]interface{}, error)
	HExists(ctx context.Context, key, field string) (bool, error)
	HDel(ctx context.Context, key string, fields ...string) (int64, error)
	HLen(ctx context.Context, key string) (int64, error)
	HKeys(ctx context.Context, key string) ([]string, error)
	HVals(ctx context.Context, key string) ([]string, error)
	HIncrBy(ctx context.Context, key, field string, incr int64) (int64, error)
	HIncrByFloat(ctx context.Context, key, field string, incr float64) (float64, error)

	// List operations
	LPush(ctx context.Context, key string, values ...interface{}) (int64, error)
	RPush(ctx context.Context, key string, values ...interface{}) (int64, error)
	LPop(ctx context.Context, key string) (string, error)
	RPop(ctx context.Context, key string) (string, error)
	LRange(ctx context.Context, key string, start, stop int64) ([]string, error)
	LLen(ctx context.Context, key string) (int64, error)
	LRem(ctx context.Context, key string, count int64, value interface{}) (int64, error)
	LTrim(ctx context.Context, key string, start, stop int64) error
	LIndex(ctx context.Context, key string, index int64) (string, error)
	LSet(ctx context.Context, key string, index int64, value interface{}) error
	BLPop(ctx context.Context, timeout time.Duration, keys ...string) ([]string, error)
	BRPop(ctx context.Context, timeout time.Duration, keys ...string) ([]string, error)

	// Set operations
	SAdd(ctx context.Context, key string, members ...interface{}) (int64, error)
	SRem(ctx context.Context, key string, members ...interface{}) (int64, error)
	SMembers(ctx context.Context, key string) ([]string, error)
	SIsMember(ctx context.Context, key string, member interface{}) (bool, error)
	SCard(ctx context.Context, key string) (int64, error)
	SPop(ctx context.Context, key string) (string, error)
	SPopN(ctx context.Context, key string, count int64) ([]string, error)
	SRandMember(ctx context.Context, key string) (string, error)
	SRandMemberN(ctx context.Context, key string, count int64) ([]string, error)
	SInter(ctx context.Context, keys ...string) ([]string, error)
	SUnion(ctx context.Context, keys ...string) ([]string, error)
	SDiff(ctx context.Context, keys ...string) ([]string, error)

	// Sorted set operations
	ZAdd(ctx context.Context, key string, members ...redis.Z) (int64, error)
	ZRem(ctx context.Context, key string, members ...interface{}) (int64, error)
	ZRange(ctx context.Context, key string, start, stop int64) ([]string, error)
	ZRangeWithScores(ctx context.Context, key string, start, stop int64) ([]redis.Z, error)
	ZRevRange(ctx context.Context, key string, start, stop int64) ([]string, error)
	ZRevRangeWithScores(ctx context.Context, key string, start, stop int64) ([]redis.Z, error)
	ZRangeByScore(ctx context.Context, key string, opt *redis.ZRangeBy) ([]string, error)
	ZRevRangeByScore(ctx context.Context, key string, opt *redis.ZRangeBy) ([]string, error)
	ZScore(ctx context.Context, key, member string) (float64, error)
	ZCard(ctx context.Context, key string) (int64, error)
	ZCount(ctx context.Context, key, min, max string) (int64, error)
	ZIncrBy(ctx context.Context, key string, increment float64, member string) (float64, error)
	ZRank(ctx context.Context, key, member string) (int64, error)
	ZRevRank(ctx context.Context, key, member string) (int64, error)

	// Pub/Sub operations
	Publish(ctx context.Context, channel string, message interface{}) (int64, error)
	Subscribe(ctx context.Context, channels ...string) *redis.PubSub
	PSubscribe(ctx context.Context, patterns ...string) *redis.PubSub

	// Transaction and pipeline operations
	Pipeline() redis.Pipeliner
	TxPipeline() redis.Pipeliner
	Watch(ctx context.Context, fn func(*redis.Tx) error, keys ...string) error

	// Advanced operations
	SetJSON(ctx context.Context, key string, value interface{}, ttl time.Duration) error
	GetJSON(ctx context.Context, key string, dest interface{}) error
	AcquireLock(ctx context.Context, key string, ttl time.Duration) (*Lock, error)
	RateLimit(ctx context.Context, key string, limit int64, window time.Duration) (bool, error)
}
