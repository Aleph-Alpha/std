// Package database provides a unified interface for SQL database operations.
//
// This package defines shared interfaces (Client, QueryBuilder) that work across
// different SQL databases including PostgreSQL, MariaDB/MySQL, and potentially others.
//
// # Implementations
//
// The Client interface is implemented by:
//   - postgres.Postgres (*Postgres)
//   - mariadb.MariaDB (*MariaDB)
//
// # Usage
//
// Applications can depend on the database.Client interface for true database-agnostic code:
//
//	type UserRepository struct {
//	    db database.Client
//	}
//
//	func NewUserRepository(db database.Client) *UserRepository {
//	    return &UserRepository{db: db}
//	}
//
// Then select the implementation via configuration:
//
//	var client database.Client
//	switch config.DBType {
//	case "postgres":
//	    client, err = postgres.NewClient(config.Postgres)
//	case "mariadb":
//	    client, err = mariadb.NewClient(config.MariaDB)
//	}
//
// # Database-Specific Behavior
//
// While the interface is unified, some methods have database-specific behavior:
//
// Row-Level Locking:
//   - PostgreSQL: Works in all contexts (even outside explicit transactions)
//   - MariaDB: Requires InnoDB storage engine AND explicit transactions (or autocommit=0)
//
// Lock Modes:
//   - ForNoKeyUpdate(), ForKeyShare(): PostgreSQL-only (no-op in MariaDB)
//   - ForUpdate(), ForShare(): Supported by both
//
// See individual database package documentation for details.
package database

import (
	"context"

	"gorm.io/gorm"
)

// Client is the main database client interface that provides CRUD operations,
// query building, and transaction management.
//
// This interface allows applications to:
//   - Switch between PostgreSQL and MariaDB without code changes
//   - Write database-agnostic business logic
//   - Mock database operations easily for testing
//   - Depend on abstractions rather than concrete implementations
//
// Implementations:
//   - postgres.Postgres implements this interface
//   - mariadb.MariaDB implements this interface
type Client interface {
	// Basic CRUD operations
	Find(ctx context.Context, dest interface{}, conditions ...interface{}) error
	First(ctx context.Context, dest interface{}, conditions ...interface{}) error
	Create(ctx context.Context, value interface{}) error
	Save(ctx context.Context, value interface{}) error
	Update(ctx context.Context, model interface{}, attrs interface{}) (int64, error)
	UpdateColumn(ctx context.Context, model interface{}, columnName string, value interface{}) (int64, error)
	UpdateColumns(ctx context.Context, model interface{}, columnValues map[string]interface{}) (int64, error)
	Delete(ctx context.Context, value interface{}, conditions ...interface{}) (int64, error)
	Count(ctx context.Context, model interface{}, count *int64, conditions ...interface{}) error
	UpdateWhere(ctx context.Context, model interface{}, attrs interface{}, condition string, args ...interface{}) (int64, error)
	Exec(ctx context.Context, sql string, values ...interface{}) (int64, error)

	// Query builder for complex queries
	// Returns the QueryBuilder interface for method chaining
	Query(ctx context.Context) QueryBuilder

	// Transaction support
	// The callback function receives a Client interface (not a concrete type)
	// This allows the same transaction code to work with any database implementation
	Transaction(ctx context.Context, fn func(tx Client) error) error

	// Raw GORM access for advanced use cases
	// Use this when you need direct access to GORM's functionality
	DB() *gorm.DB

	// Error translation / classification.
	//
	// std deliberately returns raw GORM/driver errors from CRUD/query methods.
	// Use TranslateError to normalize errors to std's exported sentinels (ErrRecordNotFound,
	// ErrDuplicateKey, ...), especially when working with the Client interface (e.g. inside
	// Transaction callbacks).
	//
	// Note: GetErrorCategory returns implementation-specific ErrorCategory type
	// (postgres.ErrorCategory or mariadb.ErrorCategory). Cast as needed.
	TranslateError(err error) error
	IsRetryable(err error) bool
	IsTemporary(err error) bool
	IsCritical(err error) bool

	// Lifecycle management
	GracefulShutdown() error
}

// QueryBuilder provides a fluent interface for building complex database queries.
// All chainable methods return the QueryBuilder interface, allowing method chaining.
// Terminal operations (like Find, First, Create) execute the query and return results.
//
// Example:
//
//	var users []User
//	err := db.Query(ctx).
//	    Where("age > ?", 18).
//	    Order("created_at DESC").
//	    Limit(10).
//	    Find(&users)
//
// # Database-Specific Behavior
//
// Some methods have database-specific behavior or limitations:
//
//   - ForNoKeyUpdate(), ForKeyShare(): PostgreSQL-only (no-op in MariaDB)
//   - Row-level locking (ForUpdate, ForShare, etc.):
//   - PostgreSQL: Works in all contexts
//   - MariaDB: Requires InnoDB storage engine AND explicit transactions
type QueryBuilder interface {
	// Query modifiers - these return QueryBuilder for chaining
	Select(query interface{}, args ...interface{}) QueryBuilder
	Where(query interface{}, args ...interface{}) QueryBuilder
	Or(query interface{}, args ...interface{}) QueryBuilder
	Not(query interface{}, args ...interface{}) QueryBuilder
	Joins(query string, args ...interface{}) QueryBuilder
	LeftJoin(query string, args ...interface{}) QueryBuilder
	RightJoin(query string, args ...interface{}) QueryBuilder
	Preload(query string, args ...interface{}) QueryBuilder
	Group(query string) QueryBuilder
	Having(query interface{}, args ...interface{}) QueryBuilder
	Order(value interface{}) QueryBuilder
	Limit(limit int) QueryBuilder
	Offset(offset int) QueryBuilder
	Raw(sql string, values ...interface{}) QueryBuilder
	Model(value interface{}) QueryBuilder
	Distinct(args ...interface{}) QueryBuilder
	Table(name string) QueryBuilder
	Unscoped() QueryBuilder
	Scopes(funcs ...func(*gorm.DB) *gorm.DB) QueryBuilder

	// Locking methods
	//
	// Note on row-level locking:
	//   - PostgreSQL: All locking methods work in any context
	//   - MariaDB: Row-level locks require:
	//     * InnoDB storage engine (MyISAM/Aria use table-level locks)
	//     * Explicit transaction (or autocommit=0)
	//     * With autocommit=1, locks have NO EFFECT in InnoDB
	//
	// Always use locking within Transaction() for MariaDB:
	//   db.Transaction(ctx, func(tx Client) error {
	//       return tx.Query(ctx).ForUpdate().First(&user)
	//   })
	ForUpdate() QueryBuilder
	ForShare() QueryBuilder
	ForUpdateSkipLocked() QueryBuilder
	ForShareSkipLocked() QueryBuilder
	ForUpdateNoWait() QueryBuilder
	ForNoKeyUpdate() QueryBuilder // PostgreSQL-only (no-op in MariaDB)
	ForKeyShare() QueryBuilder    // PostgreSQL-only (no-op in MariaDB)

	// Conflict handling and returning
	OnConflict(onConflict interface{}) QueryBuilder
	Returning(columns ...string) QueryBuilder

	// Custom clauses
	Clauses(conds ...interface{}) QueryBuilder

	// Terminal operations - these execute the query
	Scan(dest interface{}) error
	Find(dest interface{}) error
	First(dest interface{}) error
	Last(dest interface{}) error
	Count(count *int64) error
	Updates(values interface{}) (int64, error)
	Delete(value interface{}) (int64, error)
	Pluck(column string, dest interface{}) (int64, error)
	Create(value interface{}) (int64, error)
	CreateInBatches(value interface{}, batchSize int) (int64, error)
	FirstOrInit(dest interface{}, conds ...interface{}) error
	FirstOrCreate(dest interface{}, conds ...interface{}) error

	// Utility methods
	Done()                // Finalize builder (currently a no-op)
	ToSubquery() *gorm.DB // Convert to GORM subquery
}
