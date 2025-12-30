// Package database provides a unified interface for SQL database operations.
//
// This package defines shared interfaces (Client, QueryBuilder) that work across
// different SQL databases including PostgreSQL, MariaDB/MySQL, and potentially others.
//
// # Philosophy
//
// The database package follows Go's "accept interfaces, return structs" principle:
//   - Applications depend on database.Client interface
//   - Implementations (postgres, mariadb) return concrete types
//   - Concrete types implement the interface
//
// This enables:
//   - True database-agnostic application code
//   - Easy switching between databases via configuration
//   - Simplified testing with mock implementations
//   - Future support for additional databases (SQLite, CockroachDB, etc.)
//
// # Basic Usage
//
// Import the database package and choose an implementation:
//
//	import (
//	    "github.com/Aleph-Alpha/std/v1/database"
//	    "github.com/Aleph-Alpha/std/v1/postgres"
//	    "github.com/Aleph-Alpha/std/v1/mariadb"
//	)
//
//	// Application code depends on the interface
//	type UserRepository struct {
//	    db database.Client
//	}
//
//	func NewUserRepository(db database.Client) *UserRepository {
//	    return &UserRepository{db: db}
//	}
//
//	// Configuration-driven database selection
//	func NewDatabase(config Config) (database.Client, error) {
//	    switch config.DBType {
//	    case "postgres":
//	        return postgres.NewPostgres(config.Postgres)
//	    case "mariadb":
//	        return mariadb.NewMariaDB(config.MariaDB)
//	    default:
//	        return nil, fmt.Errorf("unsupported database: %s", config.DBType)
//	    }
//	}
//
// # Using with Fx Dependency Injection
//
// For applications using Uber's fx framework:
//
//	import (
//	    "go.uber.org/fx"
//	    "github.com/Aleph-Alpha/std/v1/database"
//	    "github.com/Aleph-Alpha/std/v1/postgres"
//	)
//
//	app := fx.New(
//	    // Provide the implementation
//	    postgres.FXModule,
//
//	    // Annotate to provide as database.Client interface
//	    fx.Provide(
//	        fx.Annotate(
//	            func(pg *postgres.Postgres) database.Client {
//	                return pg
//	            },
//	            fx.As(new(database.Client)),
//	        ),
//	    ),
//
//	    // Application code receives database.Client
//	    fx.Invoke(func(db database.Client) {
//	        // Use db...
//	    }),
//	)
//
// # Database-Specific Behavior
//
// While the interface is unified, some operations have database-specific behavior:
//
// ## Row-Level Locking
//
// PostgreSQL:
//   - Row-level locks work in all contexts (even outside explicit transactions)
//   - Supports all locking modes: FOR UPDATE, FOR SHARE, FOR NO KEY UPDATE, FOR KEY SHARE
//   - Each statement is implicitly wrapped in a transaction
//
// MariaDB/MySQL:
//   - Row-level locks require InnoDB storage engine
//   - Row-level locks require explicit transactions (or autocommit=0)
//   - With autocommit=1 (default), locks have NO EFFECT in InnoDB
//   - Non-InnoDB engines (MyISAM, Aria) fall back to table-level locks
//   - Only supports FOR UPDATE and FOR SHARE
//   - FOR NO KEY UPDATE and FOR KEY SHARE are no-ops
//
// Best practice: Always use locking within transactions:
//
//	err := db.Transaction(ctx, func(tx database.Client) error {
//	    var user User
//	    // This works on both PostgreSQL and MariaDB
//	    err := tx.Query(ctx).
//	        Where("id = ?", userID).
//	        ForUpdate().
//	        First(&user)
//	    if err != nil {
//	        return err
//	    }
//	    // Update user...
//	    return tx.Save(ctx, &user)
//	})
//
// ## Error Handling
//
// Use TranslateError to normalize database-specific errors to std sentinels:
//
//	err := db.First(ctx, &user, "id = ?", id)
//	err = db.TranslateError(err) // Normalize to std errors
//
//	switch {
//	case errors.Is(err, postgres.ErrRecordNotFound):
//	    // Handle not found
//	case errors.Is(err, postgres.ErrDuplicateKey):
//	    // Handle duplicate
//	case db.IsRetryable(err):
//	    // Retry the operation
//	}
//
// ## Query Builder
//
// The QueryBuilder interface provides a fluent API for complex queries:
//
//	var users []User
//	err := db.Query(ctx).
//	    Select("id, name, email").
//	    Where("age > ?", 18).
//	    Where("status = ?", "active").
//	    Order("created_at DESC").
//	    Limit(100).
//	    Find(&users)
//
// ## Transactions
//
// Transactions work identically across databases:
//
//	err := db.Transaction(ctx, func(tx database.Client) error {
//	    // All operations use tx, not db
//	    if err := tx.Create(ctx, &user); err != nil {
//	        return err // Automatic rollback
//	    }
//	    if err := tx.Create(ctx, &profile); err != nil {
//	        return err // Automatic rollback
//	    }
//	    return nil // Automatic commit
//	})
//
// # Migration Between Databases
//
// To migrate from database-specific code to database-agnostic code:
//
// Before:
//
//	import "github.com/Aleph-Alpha/std/v1/postgres"
//
//	type Service struct {
//	    db *postgres.Postgres
//	}
//
// After:
//
//	import "github.com/Aleph-Alpha/std/v1/database"
//
//	type Service struct {
//	    db database.Client
//	}
//
// All methods remain the same - only the type changes.
//
// # Testing
//
// Mock the database.Client interface for unit tests:
//
//	type MockDB struct {
//	    database.Client
//	    FindFunc func(ctx context.Context, dest interface{}, conditions ...interface{}) error
//	}
//
//	func (m *MockDB) Find(ctx context.Context, dest interface{}, conditions ...interface{}) error {
//	    if m.FindFunc != nil {
//	        return m.FindFunc(ctx, dest, conditions...)
//	    }
//	    return nil
//	}
//
// # Future Databases
//
// This interface is designed to be implemented by additional databases:
//   - SQLite (for embedded use cases)
//   - CockroachDB (for distributed SQL)
//   - TiDB (for MySQL-compatible distributed SQL)
//   - Any other SQL database using GORM
//
// New implementations should:
//  1. Implement database.Client and database.QueryBuilder
//  2. Document database-specific behavior
//  3. Add compile-time checks: var _ database.Client = (*YourDB)(nil)
//
// For more details, see the individual database package documentation:
//   - docs/v1/postgres.md
//   - docs/v1/mariadb.md
package database
