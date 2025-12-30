package database

import (
	"context"
	"fmt"
	"log"

	"github.com/Aleph-Alpha/std/v1/mariadb"
	"github.com/Aleph-Alpha/std/v1/postgres"
	"go.uber.org/fx"
)

// FXModule provides database.Client via dependency injection.
// It selects the implementation (postgres or mariadb) based on the provided Config.
//
// Usage:
//
//	app := fx.New(
//	    database.FXModule,
//	    fx.Provide(func() database.Config {
//	        return database.PostgresConfig(postgres.Config{...})
//	    }),
//	    fx.Invoke(func(db *postgres.Postgres) {
//	        // Receives *postgres.Postgres (concrete type)
//	        // Has all database.Client methods
//	    }),
//	)
//
// Or inject as specific type for database-agnostic code:
//
//	fx.Invoke(func(db any) {
//	    // Use type assertion or interface duck typing
//	    type clientInterface interface {
//	        Find(ctx context.Context, dest interface{}, conditions ...interface{}) error
//	        // ... other methods
//	    }
//	    client := db.(clientInterface)
//	})
var FXModule = fx.Module("database",
	fx.Provide(NewClientWithDI),
	fx.Invoke(RegisterDatabaseLifecycle),
)

// DatabaseParams groups the dependencies needed to create a database client
type DatabaseParams struct {
	fx.In

	Config Config
}

// DatabaseLifecycleParams groups the dependencies needed for database lifecycle management
type DatabaseLifecycleParams struct {
	fx.In

	Lifecycle fx.Lifecycle
	Client    Client
}

// NewClientWithDI creates a database client using dependency injection.
// The concrete implementation (postgres or mariadb) is selected based on Config.Type.
//
// Returns database.Client interface.
//
// Note: Due to Go's type system limitations with covariant return types,
// this returns any. The concrete types (*postgres.Postgres and *mariadb.MariaDB)
// have all the required methods and will satisfy database.Client at runtime.
func NewClientWithDI(params DatabaseParams) (any, error) {
	switch params.Config.Type {
	case "postgres":
		if params.Config.Postgres == nil {
			return nil, fmt.Errorf("postgres config is required when type=postgres")
		}
		return postgres.NewPostgres(*params.Config.Postgres)

	case "mariadb":
		if params.Config.MariaDB == nil {
			return nil, fmt.Errorf("mariadb config is required when type=mariadb")
		}
		return mariadb.NewMariaDB(*params.Config.MariaDB)

	default:
		return nil, fmt.Errorf("unsupported database type: %s (must be 'postgres' or 'mariadb')", params.Config.Type)
	}
}

// RegisterDatabaseLifecycle registers the database client with the fx lifecycle system.
// This ensures proper initialization and graceful shutdown.
func RegisterDatabaseLifecycle(params DatabaseLifecycleParams) {
	params.Lifecycle.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			log.Println("INFO: Database client initialized")
			return nil
		},
		OnStop: func(ctx context.Context) error {
			log.Println("INFO: Shutting down database client")
			return params.Client.GracefulShutdown()
		},
	})
}
