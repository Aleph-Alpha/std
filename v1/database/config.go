package database

import (
	"github.com/Aleph-Alpha/std/v1/mariadb"
	"github.com/Aleph-Alpha/std/v1/postgres"
)

// Config contains configuration for database client creation.
// Use one of the helper functions (PostgresConfig, MariaDBConfig) to create it.
type Config struct {
	// Type is the database type ("postgres" or "mariadb")
	Type string

	// Postgres configuration (used when Type = "postgres")
	Postgres *postgres.Config

	// MariaDB configuration (used when Type = "mariadb")
	MariaDB *mariadb.Config
}

// PostgresConfig creates a database.Config for PostgreSQL.
// Use this in your fx.Provide function.
//
// Example:
//
//	fx.Provide(func() database.Config {
//	    return database.PostgresConfig(postgres.Config{
//	        Connection: postgres.Connection{
//	            Host: "localhost",
//	            Port: 5432,
//	            // ...
//	        },
//	    })
//	})
func PostgresConfig(cfg postgres.Config) Config {
	return Config{
		Type:     "postgres",
		Postgres: &cfg,
	}
}

// MariaDBConfig creates a database.Config for MariaDB/MySQL.
// Use this in your fx.Provide function.
//
// Example:
//
//	fx.Provide(func() database.Config {
//	    return database.MariaDBConfig(mariadb.Config{
//	        Connection: mariadb.Connection{
//	            Host: "localhost",
//	            Port: 3306,
//	            // ...
//	        },
//	    })
//	})
func MariaDBConfig(cfg mariadb.Config) Config {
	return Config{
		Type:    "mariadb",
		MariaDB: &cfg,
	}
}
