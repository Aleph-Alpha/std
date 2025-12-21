package redis

import "errors"

// Common Redis errors
var (
	// Nil is returned when a key does not exist.
	Nil = errors.New("redis: nil")

	// ErrClosed is returned when the client is closed.
	ErrClosed = errors.New("redis: client is closed")

	// ErrPoolTimeout is returned when all connections in the pool are busy
	// and PoolTimeout was reached.
	ErrPoolTimeout = errors.New("redis: connection pool timeout")

	// ErrLockNotAcquired is returned when a lock cannot be acquired.
	ErrLockNotAcquired = errors.New("redis: lock not acquired")

	// ErrLockNotHeld is returned when trying to release a lock that is not held.
	ErrLockNotHeld = errors.New("redis: lock not held")

	// ErrRateLimitExceeded is returned when a rate limit is exceeded.
	ErrRateLimitExceeded = errors.New("redis: rate limit exceeded")
)

// IsNilError checks if the error is a "key does not exist" error.
func IsNilError(err error) bool {
	return errors.Is(err, Nil)
}

// IsClosedError checks if the error is a "client is closed" error.
func IsClosedError(err error) bool {
	return errors.Is(err, ErrClosed)
}

// IsPoolTimeoutError checks if the error is a pool timeout error.
func IsPoolTimeoutError(err error) bool {
	return errors.Is(err, ErrPoolTimeout)
}
