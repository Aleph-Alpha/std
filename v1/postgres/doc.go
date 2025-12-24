// Package postgres provides a PostgreSQL client built on top of GORM.
//
// The package exposes a small, database-agnostic interface (`Client`) plus a fluent
// query builder (`QueryBuilder`). The concrete implementation (`*Postgres`) wraps a
// `*gorm.DB` and adds:
//   - Connection establishment + pool configuration
//   - Periodic health checks and automatic reconnection (via `MonitorConnection` + `RetryConnection`)
//   - Standardized CRUD and query-builder helpers
//   - Optional error normalization (`TranslateError`)
//
// # Concurrency model
//
// The active `*gorm.DB` connection pointer is stored in an `atomic.Pointer`. Calls that
// need a DB snapshot simply load the pointer and run the operation without holding any
// package-level locks. Reconnection swaps the pointer atomically.
//
// Basic usage
//
//	cfg := postgres.Config{
//	    Connection: postgres.Connection{
//	        Host:     "localhost",
//	        Port:     "5432",
//	        User:     "postgres",
//	        Password: "password",
//	        DbName:   "mydb",
//	        SSLMode:  "disable",
//	    },
//	}
//
//	pg, err := postgres.NewPostgres(cfg)
//	if err != nil {
//	    // handle
//	}
//	defer pg.GracefulShutdown()
//
//	var users []User
//	if err := pg.Find(ctx, &users, "active = ?", true); err != nil {
//	    // handle
//	}
//
// Transaction usage
//
//	err := pg.Transaction(ctx, func(tx postgres.Client) error {
//	    if err := tx.Create(ctx, &User{Name: "Alice"}); err != nil {
//	        return err
//	    }
//	    return nil
//	})
//
// # Fx integration
//
// The package provides `FXModule` which constructs `*Postgres` and also exposes it as
// the `Client` interface, plus lifecycle hooks for starting/stopping monitoring.
//
//	app := fx.New(
//	    postgres.FXModule,
//	    fx.Provide(loadPostgresConfig),
//	)
package postgres
