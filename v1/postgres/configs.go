package postgres

import (
	"context"
	"time"
)

// Config represents the complete configuration for a PostgresSQL database connection.
// It encapsulates both the basic connection parameters and detailed connection pool settings.
type Config struct {
	// Connection contains the essential parameters needed to establish a database connection
	Connection Connection

	// ConnectionDetails contains configuration for the connection pool behavior
	ConnectionDetails ConnectionDetails
}

// Connection holds the basic parameters required to connect to a PostgresSQL database.
// These parameters are used to construct the database connection string.
type Connection struct {
	// Host specifies the database server hostname or IP address
	Host string

	// Port specifies the TCP port on which the database server is listening to
	Port string

	// User specifies the database username for authentication
	User string

	// Password specifies the database user password for authentication
	Password string

	// DbName specifies the name of the database to connect to
	DbName string

	// SSLMode specifies the SSL mode for the connection (e.g., "disable", "require", "verify-ca", "verify-full")
	// For production environments, it's recommended to use at least "require"
	SSLMode string
}

// ConnectionDetails holds configuration settings for the database connection pool.
// These settings help optimize performance and resource usage by controlling
// how database connections are created, reused, and expired.
type ConnectionDetails struct {
	// MaxOpenConns controls the maximum number of open connections to the database.
	// Setting this appropriately helps prevent overwhelming the database with too many connections.
	// If set to 0, the package default is used.
	MaxOpenConns int

	// MaxIdleConns controls the maximum number of connections in the idle connection pool.
	// A higher value can improve performance under a concurrent load but consumes more resources.
	// If set to 0, the package default is used.
	MaxIdleConns int

	// ConnMaxLifetime is the maximum amount of time a connection may be reused.
	// Expired connections are closed and removed from the pool during connection acquisition.
	// This helps ensure database-enforced timeouts are respected.
	// If set to 0, the package default is used.
	ConnMaxLifetime time.Duration
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
