// Package postgres provides PostgreSQL database operations with an interface-first design.
//
// This package implements the shared database.Client interface defined in v1/database.
// For database-agnostic code, depend on database.Client instead of postgres.Client.
//
// The postgres.Postgres type implements both postgres.Client (deprecated) and database.Client.
package postgres

import (
	"context"

	"gorm.io/gorm"
)

// Client is the PostgreSQL-specific client interface.
//
// DEPRECATED: Use database.Client instead for database-agnostic code.
//
// This interface is kept for backward compatibility. The Postgres type implements
// both this interface and database.Client.
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
	TranslateError(err error) error
	GetErrorCategory(err error) ErrorCategory
	IsRetryable(err error) bool
	IsTemporary(err error) bool
	IsCritical(err error) bool

	// Lifecycle management
	GracefulShutdown() error
}

// QueryBuilder provides a fluent interface for building complex database queries.
//
// DEPRECATED: Use database.QueryBuilder instead for database-agnostic code.
//
// This interface is kept for backward compatibility. The postgresQueryBuilder type
// implements both this interface and database.QueryBuilder.
//
// Example:
//
//	var users []User
//	err := db.Query(ctx).
//	    Where("age > ?", 18).
//	    Order("created_at DESC").
//	    Limit(10).
//	    Find(&users)
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
	ForUpdate() QueryBuilder
	ForShare() QueryBuilder
	ForUpdateSkipLocked() QueryBuilder
	ForShareSkipLocked() QueryBuilder
	ForUpdateNoWait() QueryBuilder
	ForNoKeyUpdate() QueryBuilder // PostgreSQL-specific
	ForKeyShare() QueryBuilder    // PostgreSQL-specific

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
