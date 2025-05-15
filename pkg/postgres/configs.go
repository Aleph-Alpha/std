package postgres

import "time"

type Config struct {
	Connection        Connection
	ConnectionDetails ConnectionDetails
}

type Connection struct {
	Host     string
	Port     string
	User     string
	Password string
	DbName   string
	SSLMode  string
}

type ConnectionDetails struct {
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
}
