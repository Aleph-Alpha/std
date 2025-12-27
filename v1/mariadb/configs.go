package mariadb

import (
	"context"
	"time"
)

// Config represents the complete configuration for a MariaDB/MySQL database connection.
// It encapsulates both the basic connection parameters and detailed connection pool settings.
type Config struct {
	// Connection contains the essential parameters needed to establish a database connection
	Connection Connection

	// ConnectionDetails contains configuration for the connection pool behavior
	ConnectionDetails ConnectionDetails
}

// Connection holds the basic parameters required to connect to a MariaDB/MySQL database.
// These parameters are used to construct the database connection DSN.
type Connection struct {
	// Host specifies the database server hostname or IP address
	Host string

	// Port specifies the TCP port on which the database server is listening
	Port string

	// User specifies the database username for authentication
	User string

	// Password specifies the database user password for authentication
	Password string

	// DbName specifies the name of the database to connect to
	DbName string

	// Charset specifies the character set to use for the connection
	// Common values: "utf8mb4" (recommended), "utf8", "latin1"
	// Default: "utf8mb4"
	Charset string

	// ParseTime enables parsing of DATE and DATETIME values to time.Time
	// Recommended: true
	ParseTime bool

	// Loc specifies the location for parsing timestamps
	// Common values: "Local", "UTC"
	// Default: "Local"
	Loc string

	// TLS specifies the TLS/SSL configuration name
	// Common values: "true", "false", "skip-verify", "preferred", or a custom TLS config name
	// Default: "false"
	TLS string

	// Timeout specifies the connection timeout
	// Example: "10s", "30s"
	Timeout string

	// ReadTimeout specifies the I/O read timeout
	// Example: "30s"
	ReadTimeout string

	// WriteTimeout specifies the I/O write timeout
	// Example: "30s"
	WriteTimeout string
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
