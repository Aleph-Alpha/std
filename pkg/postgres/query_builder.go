package postgres

import (
	"context"
	"gorm.io/gorm"
)

// Query provides a flexible way to build complex queries.
// It returns a QueryBuilder which can be used to chain query methods in a fluent interface.
// The method acquires a read lock on the database connection that will be automatically
// released when a terminal method is called or Done() is invoked.
//
// Parameters:
//   - ctx: Context for the database operation
//
// Returns a QueryBuilder instance that can be used to construct the query.
//
// Example:
//
//	users := []User{}
//	err := db.Query(ctx).
//	    Where("age > ?", 18).
//	    Order("created_at DESC").
//	    Limit(10).
//	    Find(&users)
func (p *Postgres) Query(ctx context.Context) *QueryBuilder {
	p.mu.RLock() // Will be released when Done() is called
	return &QueryBuilder{
		db:      p.Client.WithContext(ctx),
		release: p.mu.RUnlock,
	}
}

// QueryBuilder provides a fluent interface for building complex database queries.
// It wraps GORM's query building capabilities with thread-safety and automatic resource cleanup.
// The builder maintains a chain of query modifiers that are applied when a terminal method is called.
type QueryBuilder struct {
	// db is the underlying GORM DB instance that handles the actual query execution
	db *gorm.DB

	// release is the function to call to release the mutex lock when done with the query
	release func()
}

// Select specifies fields to be selected in the query.
// It corresponds to the SQL SELECT clause.
//
// Parameters:
//   - query: Field selection string or raw SQL expression
//   - args: Arguments for any placeholders in the query
//
// Returns the QueryBuilder for method chaining.
//
// Example:
//
//	qb.Select("id, name, email")
//	qb.Select("COUNT(*) as user_count")
func (qb *QueryBuilder) Select(query interface{}, args ...interface{}) *QueryBuilder {
	qb.db = qb.db.Select(query, args...)
	return qb
}

// Where adds a WHERE condition to the query.
// Multiple Where calls are combined with AND logic.
//
// Parameters:
//   - query: Condition string with optional placeholders or a map of conditions
//   - args: Arguments for any placeholders in the query
//
// Returns the QueryBuilder for method chaining.
//
// Example:
//
//	qb.Where("age > ?", 18)
//	qb.Where("status = ?", "active")
func (qb *QueryBuilder) Where(query interface{}, args ...interface{}) *QueryBuilder {
	qb.db = qb.db.Where(query, args...)
	return qb
}

// Or adds an OR condition to the query.
// It combines with previous conditions using OR logic.
//
// Parameters:
//   - query: Condition string with optional placeholders or a map of conditions
//   - args: Arguments for any placeholders in the query
//
// Returns the QueryBuilder for method chaining.
//
// Example:
//
//	qb.Where("status = ?", "active").Or("status = ?", "pending")
func (qb *QueryBuilder) Or(query interface{}, args ...interface{}) *QueryBuilder {
	qb.db = qb.db.Or(query, args...)
	return qb
}

// Not adds a NOT condition to the query.
// It negates the specified condition.
//
// Parameters:
//   - query: Condition string with optional placeholders or a map of conditions
//   - args: Arguments for any placeholders in the query
//
// Returns the QueryBuilder for method chaining.
//
// Example:
//
//	qb.Not("status = ?", "deleted")
func (qb *QueryBuilder) Not(query interface{}, args ...interface{}) *QueryBuilder {
	qb.db = qb.db.Not(query, args...)
	return qb
}

// Joins add a JOIN clause to the query.
// It performs an INNER JOIN by default.
//
// Parameters:
//   - query: JOIN condition string
//   - args: Arguments for any placeholders in the query
//
// Returns the QueryBuilder for method chaining.
//
// Example:
//
//	qb.Joins("JOIN orders ON orders.user_id = users.id")
func (qb *QueryBuilder) Joins(query string, args ...interface{}) *QueryBuilder {
	qb.db = qb.db.Joins(query, args...)
	return qb
}

// LeftJoin adds a LEFT JOIN clause to the query.
// It retrieves all records from the left table and matching records from the right table.
//
// Parameters:
//   - query: JOIN condition string without the "LEFT JOIN" prefix
//   - args: Arguments for any placeholders in the query
//
// Returns the QueryBuilder for method chaining.
//
// Example:
//
//	qb.LeftJoin("orders ON orders.user_id = users.id")
func (qb *QueryBuilder) LeftJoin(query string, args ...interface{}) *QueryBuilder {
	joinClause := "LEFT JOIN " + query
	qb.db = qb.db.Joins(joinClause, args...)
	return qb
}

// RightJoin adds a RIGHT JOIN clause to the query.
// It retrieves all records from the right table and matching records from the left table.
//
// Parameters:
//   - query: JOIN condition string without the "RIGHT JOIN" prefix
//   - args: Arguments for any placeholders in the query
//
// Returns the QueryBuilder for method chaining.
//
// Example:
//
//	qb.RightJoin("orders ON orders.user_id = users.id")
func (qb *QueryBuilder) RightJoin(query string, args ...interface{}) *QueryBuilder {
	joinClause := "RIGHT JOIN " + query
	qb.db = qb.db.Joins(joinClause, args...)
	return qb
}

// Preload preloads associations for the query results.
// This is used to eagerly load related models to avoid N+1 query problems.
//
// Parameters:
//   - query: Name of the association to preload
//   - args: Optional conditions for the preloaded association
//
// Return the QueryBuilder for method chaining.
//
// Example:
//
//	qb.Preload("Orders")
//	qb.Preload("Orders", "state = ?", "paid")
func (qb *QueryBuilder) Preload(query string, args ...interface{}) *QueryBuilder {
	qb.db = qb.db.Preload(query, args...)
	return qb
}

// Group adds a GROUP BY clause to the query.
// It is used to group rows with the same values into summary rows.
//
// Parameters:
//   - query: GROUP BY expression
//
// Returns the QueryBuilder for method chaining.
//
// Example:
//
//	qb.Group("status")
//	qb.Group("department, location")
func (qb *QueryBuilder) Group(query string) *QueryBuilder {
	qb.db = qb.db.Group(query)
	return qb
}

// Having added a HAVING clause to the query.
// It is used to filter groups created by the GROUP BY clause.
//
// Parameters:
//   - query: HAVING condition with optional placeholders
//   - args: Arguments for any placeholders in the query
//
// Returns the QueryBuilder for method chaining.
//
// Example:
//
//	qb.Group("department").Having("COUNT(*) > ?", 3)
func (qb *QueryBuilder) Having(query interface{}, args ...interface{}) *QueryBuilder {
	qb.db = qb.db.Having(query, args...)
	return qb
}

// Order adds an ORDER BY clause to the query.
// It is used to sort the result set.
//
// Parameters:
//   - value: ORDER BY expression
//
// Returns the QueryBuilder for method chaining.
//
// Example:
//
//	qb.Order("created_at DESC")
//	qb.Order("age ASC, name DESC")
func (qb *QueryBuilder) Order(value interface{}) *QueryBuilder {
	qb.db = qb.db.Order(value)
	return qb
}

// Limit sets the maximum number of records to return.
//
// Parameters:
//   - limit: Maximum number of records
//
// Returns the QueryBuilder for method chaining.
//
// Example:
//
//	qb.Limit(10) // Return at most 10 records
func (qb *QueryBuilder) Limit(limit int) *QueryBuilder {
	qb.db = qb.db.Limit(limit)
	return qb
}

// Offset sets the number of records to skip.
// It is typically used with Limit for pagination.
//
// Parameters:
//   - offset: Number of records to skip
//
// Returns the QueryBuilder for method chaining.
//
// Example:
//
//	qb.Offset(20).Limit(10) // Skip 20 records and return the next 10
func (qb *QueryBuilder) Offset(offset int) *QueryBuilder {
	qb.db = qb.db.Offset(offset)
	return qb
}

// Raw executes raw SQL as part of the query.
// It provides full SQL flexibility when needed.
//
// Parameters:
//   - SQL: Raw SQL statement with optional placeholders
//   - values: Arguments for any placeholders in the SQL
//
// Returns the QueryBuilder for method chaining.
//
// Example:
//
//	qb.Raw("SELECT * FROM users WHERE created_at > ?", time.Now().AddDate(0, -1, 0))
func (qb *QueryBuilder) Raw(sql string, values ...interface{}) *QueryBuilder {
	qb.db = qb.db.Raw(sql, values...)
	return qb
}

// Model specifies the model to use for the query.
// This is useful when the model can't be inferred from other methods.
//
// Parameters:
//   - value: Pointer to the model struct or its instance
//
// Returns the QueryBuilder for method chaining.
//
// Example:
//
//	qb.Model(&User{}).Where("active = ?", true).Count(&count)
func (qb *QueryBuilder) Model(value interface{}) *QueryBuilder {
	qb.db = qb.db.Model(value)
	return qb
}

// Scan scans the result into the destination struct or slice.
// This is a terminal method that executes the query and releases the mutex lock.
//
// Parameters:
//   - dest: Pointer to the struct or slice where results will be stored
//
// Returns an error if the query fails or nil on success.
//
// Example:
//
//	var result struct{ Count int }
//	err := qb.Raw("SELECT COUNT(*) as count FROM users").Scan(&result)
func (qb *QueryBuilder) Scan(dest interface{}) error {
	defer qb.release()
	return qb.db.Scan(dest).Error
}

// Find finds records that match the query conditions.
// This is a terminal method that executes the query and releases the mutex lock.
//
// Parameters:
//   - dest: Pointer to a slice where results will be stored
//
// Returns an error if the query fails or nil on success.
//
// Example:
//
//	var users []User
//	err := qb.Where("active = ?", true).Find(&users)
func (qb *QueryBuilder) Find(dest interface{}) error {
	defer qb.release()
	return qb.db.Find(dest).Error
}

// First finds the first record that matches the query conditions.
// This is a terminal method that executes the query and releases the mutex lock.
//
// Parameters:
//   - dest: Pointer to a struct where the result will be stored
//
// Returns an error if no record is found or the query fails, nil on success.
//
// Example:
//
//	var user User
//	err := qb.Where("email = ?", "user@example.com").First(&user)
func (qb *QueryBuilder) First(dest interface{}) error {
	defer qb.release()
	return qb.db.First(dest).Error
}

// Last finds the last record that matches the query conditions.
// This is a terminal method that executes the query and releases the mutex lock.
//
// Parameters:
//   - dest: Pointer to a struct where the result will be stored
//
// Returns an error if no record is found or the query fails, nil on success.
//
// Example:
//
//	var user User
//	err := qb.Where("department = ?", "Engineering").Order("joined_at ASC").Last(&user)
func (qb *QueryBuilder) Last(dest interface{}) error {
	defer qb.release()
	return qb.db.Last(dest).Error
}

// Count counts records that match the query conditions.
// This is a terminal method that executes the query and releases the mutex lock.
//
// Parameters:
//   - count: Pointer to an int64 where the count will be stored
//
// Returns an error if the query fails or nil on success.
//
// Example:
//
//	var count int64
//	err := qb.Where("active = ?", true).Count(&count)
func (qb *QueryBuilder) Count(count *int64) error {
	defer qb.release()
	return qb.db.Count(count).Error
}

// Updates updates records that match the query conditions.
// This is a terminal method that executes the query and releases the mutex lock.
//
// Parameters:
//   - values: Map or struct with the fields to update
//
// Returns an error if the update fails or nil on success.
//
// Example:
//
//	err := qb.Where("expired = ?", true).Updates(map[string]interface{}{"active": false})
func (qb *QueryBuilder) Updates(values interface{}) error {
	defer qb.release()
	return qb.db.Updates(values).Error
}

// Delete deletes records that match the query conditions.
// This is a terminal method that executes the query and releases the mutex lock.
//
// Parameters:
//   - value: Model value or pointer to specify what to delete
//
// Returns an error if the deletion fails or nil on success.
//
// Example:
//
//	err := qb.Where("created_at < ?", time.Now().AddDate(-1, 0, 0)).Delete(&User{})
func (qb *QueryBuilder) Delete(value interface{}) error {
	defer qb.release()
	return qb.db.Delete(value).Error
}

// Pluck queries a single column and scans the results into a slice.
// This is a terminal method that executes the query and releases the mutex lock.
//
// Parameters:
//   - column: Name of the column to query
//   - dest: Pointer to a slice where results will be stored
//
// Returns an error if the query fails or nil on success.
//
// Example:
//
//	var emails []string
//	err := qb.Where("department = ?", "Engineering").Pluck("email", &emails)
func (qb *QueryBuilder) Pluck(column string, dest interface{}) error {
	defer qb.release()
	return qb.db.Pluck(column, dest).Error
}

// Distinct specifies that the query should return distinct results.
// It eliminates duplicate rows from the result set.
//
// Parameters:
//   - args: Optional columns to apply DISTINCT to
//
// Returns the QueryBuilder for method chaining.
//
// Example:
//
//	qb.Distinct("department").Find(&departments)
//	qb.Distinct().Where("age > ?", 18).Find(&users) // SELECT DISTINCT * FROM users WHERE age > 18
func (qb *QueryBuilder) Distinct(args ...interface{}) *QueryBuilder {
	qb.db = qb.db.Distinct(args...)
	return qb
}

// Done releases the mutex lock without executing the query.
// This method should be called when you want to cancel a query building chain
// without executing any terminal operation.
//
// Example:
//
//	qb := db.Query(ctx)
//	if someCondition {
//	    err := qb.Where(...).Find(&results)
//	} else {
//	    qb.Done() // Release the lock without executing
//	}
func (qb *QueryBuilder) Done() {
	qb.release()
}
