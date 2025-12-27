package mariadb

import (
	"context"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Query provides a flexible way to build complex queries.
// It returns a QueryBuilder interface which can be used to chain query methods in a fluent interface.
// The builder snapshots the current `*gorm.DB` connection and does not hold any package-level
// locks while the chain is being built or executed.
//
// Parameters:
//   - ctx: Context for the database operation
//
// Returns a QueryBuilder interface instance that can be used to construct the query.
//
// Note: QueryBuilder methods return GORM errors directly. Use MariaDB.TranslateError()
// to convert them to standardized error types if needed.
//
// Example:
//
//	users := []User{}
//	err := db.Query(ctx).
//	    Where("age > ?", 18).
//	    Order("created_at DESC").
//	    Limit(10).
//	    Find(&users)
//	if err != nil {
//	    err = db.TranslateError(err) // Optional: translate to standardized error
//	}
func (m *MariaDB) Query(ctx context.Context) QueryBuilder {
	// Important: do NOT hold any package-level lock across the query-builder chain.
	// Long-held locks can stall reconnect/migrations and reduce throughput if terminal ops
	// are delayed or forgotten.
	return &mariadbQueryBuilder{
		db:      m.DB().WithContext(ctx), // snapshot current connection under an atomic load
		m:       m,                       // reference to parent for observer
		release: func() {},               // no-op; kept to preserve the builder structure
	}
}

// QueryBuilder provides a fluent interface for building complex database queries.
// It wraps GORM's query building capabilities with thread-safety and automatic resource cleanup.
// The builder maintains a chain of query modifiers that are applied when a terminal method is called.
//
// Note: All terminal methods (First, Find, Create, etc.) return GORM errors directly.
// This preserves the error chain and allows consumers to use errors.Is() with GORM error types.
// Use MariaDB.TranslateError() to convert errors to standardized types if needed.
type mariadbQueryBuilder struct {
	// db is the underlying GORM DB instance that handles the actual query execution
	db *gorm.DB

	// m is a reference to the parent MariaDB for observer calls
	m *MariaDB

	// release finalizes the builder (currently a no-op).
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
func (qb *mariadbQueryBuilder) Select(query interface{}, args ...interface{}) QueryBuilder {
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
func (qb *mariadbQueryBuilder) Where(query interface{}, args ...interface{}) QueryBuilder {
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
func (qb *mariadbQueryBuilder) Or(query interface{}, args ...interface{}) QueryBuilder {
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
func (qb *mariadbQueryBuilder) Not(query interface{}, args ...interface{}) QueryBuilder {
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
func (qb *mariadbQueryBuilder) Joins(query string, args ...interface{}) QueryBuilder {
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
func (qb *mariadbQueryBuilder) LeftJoin(query string, args ...interface{}) QueryBuilder {
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
func (qb *mariadbQueryBuilder) RightJoin(query string, args ...interface{}) QueryBuilder {
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
func (qb *mariadbQueryBuilder) Preload(query string, args ...interface{}) QueryBuilder {
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
func (qb *mariadbQueryBuilder) Group(query string) QueryBuilder {
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
func (qb *mariadbQueryBuilder) Having(query interface{}, args ...interface{}) QueryBuilder {
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
func (qb *mariadbQueryBuilder) Order(value interface{}) QueryBuilder {
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
func (qb *mariadbQueryBuilder) Limit(limit int) QueryBuilder {
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
func (qb *mariadbQueryBuilder) Offset(offset int) QueryBuilder {
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
func (qb *mariadbQueryBuilder) Raw(sql string, values ...interface{}) QueryBuilder {
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
func (qb *mariadbQueryBuilder) Model(value interface{}) QueryBuilder {
	qb.db = qb.db.Model(value)
	return qb
}

// Scan scans the result into the destination struct or slice.
// This is a terminal method that executes the query and finalizes the builder.
//
// Parameters:
//   - dest: Pointer to the struct or slice where results will be stored
//
// Returns a GORM error if the query fails or nil on success.
// Use MariaDB.TranslateError() to convert to standardized error types.
//
// Example:
//
//	var result struct{ Count int }
//	err := qb.Raw("SELECT COUNT(*) as count FROM users").Scan(&result)
func (qb *mariadbQueryBuilder) Scan(dest interface{}) error {
	defer qb.release()
	start := time.Now()
	result := qb.db.Scan(dest)
	table := ""
	if result != nil && result.Statement != nil {
		table = result.Statement.Table
	}
	if qb.m != nil {
		qb.m.observeOperation("scan", table, "", time.Since(start), result.Error, result.RowsAffected, nil)
	}
	return result.Error
}

// Find finds records that match the query conditions.
// This is a terminal method that executes the query and finalizes the builder.
//
// Parameters:
//   - dest: Pointer to a slice where results will be stored
//
// Returns a GORM error if the query fails or nil on success.
// Use MariaDB.TranslateError() to convert to standardized error types.
//
// Example:
//
//	var users []User
//	err := qb.Where("active = ?", true).Find(&users)
func (qb *mariadbQueryBuilder) Find(dest interface{}) error {
	defer qb.release()
	start := time.Now()
	result := qb.db.Find(dest)
	table := ""
	if result != nil && result.Statement != nil {
		table = result.Statement.Table
	}
	if qb.m != nil {
		qb.m.observeOperation("find", table, "", time.Since(start), result.Error, result.RowsAffected, nil)
	}
	return result.Error
}

// First finds the first record that matches the query conditions.
// This is a terminal method that executes the query and finalizes the builder.
//
// Parameters:
//   - dest: Pointer to a struct where the result will be stored
//
// Returns gorm.ErrRecordNotFound if no record is found, or another GORM error if the query fails, nil on success.
// Use MariaDB.TranslateError() to convert to standardized error types if needed.
//
// Example:
//
//	var user User
//	err := qb.Where("email = ?", "user@example.com").First(&user)
//	if err != nil {
//	    err = db.TranslateError(err) // Optional: convert to standardized error
//	}
func (qb *mariadbQueryBuilder) First(dest interface{}) error {
	defer qb.release()
	start := time.Now()
	result := qb.db.First(dest)
	table := ""
	if result != nil && result.Statement != nil {
		table = result.Statement.Table
	}
	if qb.m != nil {
		qb.m.observeOperation("first", table, "", time.Since(start), result.Error, result.RowsAffected, nil)
	}
	return result.Error
}

// Last finds the last record that matches the query conditions.
// This is a terminal method that executes the query and finalizes the builder.
//
// Parameters:
//   - dest: Pointer to a struct where the result will be stored
//
// Returns gorm.ErrRecordNotFound if no record is found, or another GORM error if the query fails, nil on success.
// Use MariaDB.TranslateError() to convert to standardized error types if needed.
//
// Example:
//
//	var user User
//	err := qb.Where("department = ?", "Engineering").Order("joined_at ASC").Last(&user)
func (qb *mariadbQueryBuilder) Last(dest interface{}) error {
	defer qb.release()
	start := time.Now()
	result := qb.db.Last(dest)
	table := ""
	if result != nil && result.Statement != nil {
		table = result.Statement.Table
	}
	if qb.m != nil {
		qb.m.observeOperation("last", table, "", time.Since(start), result.Error, result.RowsAffected, nil)
	}
	return result.Error
}

// Count counts records that match the query conditions.
// This is a terminal method that executes the query and finalizes the builder.
//
// Parameters:
//   - count: Pointer to an int64 where the count will be stored
//
// Returns a GORM error if the query fails or nil on success.
//
// Example:
//
//	var count int64
//	err := qb.Where("active = ?", true).Count(&count)
func (qb *mariadbQueryBuilder) Count(count *int64) error {
	defer qb.release()
	start := time.Now()
	result := qb.db.Count(count)
	size := int64(0)
	if count != nil {
		size = *count
	}
	table := ""
	if result != nil && result.Statement != nil {
		table = result.Statement.Table
	}
	if qb.m != nil {
		qb.m.observeOperation("count", table, "", time.Since(start), result.Error, size, nil)
	}
	return result.Error
}

// Updates updates records that match the query conditions.
// This is a terminal method that executes the query and finalizes the builder.
//
// Parameters:
//   - values: Map or struct with the fields to update
//
// Returns:
//   - int64: Number of rows affected by the update operation
//   - error: GORM error if the update fails, nil on success
//
// Example:
//
//	rowsAffected, err := qb.Where("expired = ?", true).Updates(map[string]interface{}{"active": false})
//	if err != nil {
//	    return err
//	}
//	fmt.Printf("Updated %d rows\n", rowsAffected)
func (qb *mariadbQueryBuilder) Updates(values interface{}) (int64, error) {
	defer qb.release()
	start := time.Now()
	result := qb.db.Updates(values)
	table := ""
	if result != nil && result.Statement != nil {
		table = result.Statement.Table
	}
	if qb.m != nil {
		qb.m.observeOperation("update", table, "", time.Since(start), result.Error, result.RowsAffected, nil)
	}
	return result.RowsAffected, result.Error
}

// Delete deletes records that match the query conditions.
// This is a terminal method that executes the query and finalizes the builder.
//
// Parameters:
//   - value: Model value or pointer to specify what to delete
//
// Returns:
//   - int64: Number of rows affected by the delete operation
//   - error: GORM error if the deletion fails, nil on success
//
// Example:
//
//	rowsAffected, err := qb.Where("created_at < ?", time.Now().AddDate(-1, 0, 0)).Delete(&User{})
//	if err != nil {
//	    return err
//	}
//	fmt.Printf("Deleted %d rows\n", rowsAffected)
func (qb *mariadbQueryBuilder) Delete(value interface{}) (int64, error) {
	defer qb.release()
	start := time.Now()
	result := qb.db.Delete(value)
	table := ""
	if result != nil && result.Statement != nil {
		table = result.Statement.Table
	}
	if qb.m != nil {
		qb.m.observeOperation("delete", table, "", time.Since(start), result.Error, result.RowsAffected, nil)
	}
	return result.RowsAffected, result.Error
}

// Pluck queries a single column and scans the results into a slice.
// This is a terminal method that executes the query and finalizes the builder.
//
// Parameters:
//   - column: Name of the column to query
//   - dest: Pointer to a slice where results will be stored
//
// Returns:
//   - int64: Number of rows found and processed
//   - error: GORM error if the query fails, nil on success
//
// Example:
//
//	var emails []string
//	rowsFound, err := qb.Where("department = ?", "Engineering").Pluck("email", &emails)
//	if err != nil {
//	    return err
//	}
//	fmt.Printf("Found %d email addresses\n", rowsFound)
func (qb *mariadbQueryBuilder) Pluck(column string, dest interface{}) (int64, error) {
	defer qb.release()
	start := time.Now()
	result := qb.db.Pluck(column, dest)
	table := ""
	if result != nil && result.Statement != nil {
		table = result.Statement.Table
	}
	if qb.m != nil {
		qb.m.observeOperation("pluck", table, "", time.Since(start), result.Error, result.RowsAffected, map[string]interface{}{
			"column": column,
		})
	}
	return result.RowsAffected, result.Error
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
func (qb *mariadbQueryBuilder) Distinct(args ...interface{}) QueryBuilder {
	qb.db = qb.db.Distinct(args...)
	return qb
}

// Table specifies the table name for the query.
// This overrides the default table name derived from the model.
//
// Parameters:
//   - name: Table name to use for the query
//
// Returns the QueryBuilder for method chaining.
//
// Example:
//
//	qb.Table("users_archive").Where("deleted_at IS NOT NULL").Find(&users)
//	qb.Table("custom_table_name").Count(&count)
//	qb.Table("user_stats").Select("department, COUNT(*) as count").Group("department").Scan(&stats)
func (qb *mariadbQueryBuilder) Table(name string) QueryBuilder {
	qb.db = qb.db.Table(name)
	return qb
}

// Unscoped disables the default scope for the query.
// This allows querying soft-deleted records or bypassing other default scopes.
// Commonly used with GORM's soft delete feature.
//
// Returns the QueryBuilder for method chaining.
//
// Example:
//
//	qb.Unscoped().Where("name = ?", "John").Find(&users) // Includes soft-deleted records
//	qb.Unscoped().Delete(&user) // Permanently deletes the record
//	qb.Unscoped().Count(&count) // Counts all records including soft-deleted
func (qb *mariadbQueryBuilder) Unscoped() QueryBuilder {
	qb.db = qb.db.Unscoped()
	return qb
}

// Scopes applies one or more scopes to the query.
// Scopes are reusable query conditions that can be applied to multiple queries.
//
// Parameters:
//   - funcs: One or more scope functions that modify the query
//
// Returns the QueryBuilder for method chaining.
//
// Example:
//
//	// Define scopes
//	func ActiveUsers(db *gorm.DB) *gorm.DB {
//	    return db.Where("active = ?", true)
//	}
//	func AdultUsers(db *gorm.DB) *gorm.DB {
//	    return db.Where("age >= ?", 18)
//	}
//
//	// Use scopes
//	qb.Scopes(ActiveUsers, AdultUsers).Find(&users)
//	qb.Scopes(ActiveUsers).Count(&count)
func (qb *mariadbQueryBuilder) Scopes(funcs ...func(*gorm.DB) *gorm.DB) QueryBuilder {
	qb.db = qb.db.Scopes(funcs...)
	return qb
}

// ForUpdate adds a FOR UPDATE clause to the query for exclusive row-level locking.
// This prevents other transactions from modifying the selected rows until the
// current transaction commits or rolls back.
//
// Returns the QueryBuilder for method chaining.
//
// Example:
//
//	qb.Where("id = ?", userID).ForUpdate().First(&user) // Locks the row for update
//	qb.ForUpdate().Where("status = ?", "pending").Find(&orders) // Locks all matching rows
func (qb *mariadbQueryBuilder) ForUpdate() QueryBuilder {
	qb.db = qb.db.Clauses(clause.Locking{Strength: "UPDATE"})
	return qb
}

// ForShare adds a FOR SHARE clause to the query for shared row-level locking.
// This allows other transactions to read the rows but prevents them from
// updating or deleting until the current transaction commits or rolls back.
//
// Note: MySQL/MariaDB uses LOCK IN SHARE MODE for this functionality.
//
// Returns the QueryBuilder for method chaining.
//
// Example:
//
//	qb.Where("id = ?", userID).ForShare().First(&user) // Shared lock for reading
//	qb.ForShare().Where("status = ?", "active").Find(&users) // Prevents updates but allows reads
func (qb *mariadbQueryBuilder) ForShare() QueryBuilder {
	qb.db = qb.db.Clauses(clause.Locking{Strength: "SHARE"})
	return qb
}

// ForUpdateSkipLocked adds a FOR UPDATE SKIP LOCKED clause to the query.
// This acquires exclusive locks but skips any rows that are already locked,
// making it ideal for job queue processing where you want to avoid blocking.
//
// Note: Requires MySQL 8.0+ or MariaDB 10.6+
//
// Returns the QueryBuilder for method chaining.
//
// Example:
//
//	qb.Where("status = ?", "pending").ForUpdateSkipLocked().Limit(10).Find(&jobs)
//	qb.ForUpdateSkipLocked().Where("processed = ?", false).First(&task)
func (qb *mariadbQueryBuilder) ForUpdateSkipLocked() QueryBuilder {
	qb.db = qb.db.Clauses(clause.Locking{
		Strength: "UPDATE",
		Options:  "SKIP LOCKED",
	})
	return qb
}

// ForShareSkipLocked adds a FOR SHARE SKIP LOCKED clause to the query.
// This acquires shared locks but skips any rows that are already exclusively locked.
//
// Note: Requires MySQL 8.0+ or MariaDB 10.6+
//
// Returns the QueryBuilder for method chaining.
//
// Example:
//
//	qb.Where("category = ?", "news").ForShareSkipLocked().Find(&articles)
func (qb *mariadbQueryBuilder) ForShareSkipLocked() QueryBuilder {
	qb.db = qb.db.Clauses(clause.Locking{
		Strength: "SHARE",
		Options:  "SKIP LOCKED",
	})
	return qb
}

// ForUpdateNoWait adds a FOR UPDATE NOWAIT clause to the query.
// This attempts to acquire exclusive locks but immediately fails if any
// target rows are already locked, instead of waiting.
//
// Note: Requires MySQL 8.0+ or MariaDB 10.3+
//
// Returns the QueryBuilder for method chaining.
//
// Example:
//
//	qb.Where("id = ?", accountID).ForUpdateNoWait().First(&account)
func (qb *mariadbQueryBuilder) ForUpdateNoWait() QueryBuilder {
	qb.db = qb.db.Clauses(clause.Locking{
		Strength: "UPDATE",
		Options:  "NOWAIT",
	})
	return qb
}

// ForNoKeyUpdate is a PostgreSQL-specific locking mode.
// MariaDB doesn't support this, so it's a no-op that returns the builder unchanged.
func (qb *mariadbQueryBuilder) ForNoKeyUpdate() QueryBuilder {
	// No-op for MariaDB - PostgreSQL-specific feature
	return qb
}

// ForKeyShare is a PostgreSQL-specific locking mode.
// MariaDB doesn't support this, so it's a no-op that returns the builder unchanged.
func (qb *mariadbQueryBuilder) ForKeyShare() QueryBuilder {
	// No-op for MariaDB - PostgreSQL-specific feature
	return qb
}

// Returning is used in PostgreSQL to return values from INSERT/UPDATE/DELETE.
// MariaDB has limited support, so this is a no-op that returns the builder unchanged.
func (qb *mariadbQueryBuilder) Returning(columns ...string) QueryBuilder {
	// No-op for MariaDB - limited support compared to PostgreSQL
	return qb
}

// OnConflict adds an ON DUPLICATE KEY UPDATE clause for UPSERT operations.
// This handles conflicts during INSERT operations by updating existing records.
//
// Parameters:
//   - onConflict: ON CONFLICT clause configuration
//
// Returns the QueryBuilder for method chaining.
//
// Example:
//
//	qb.OnConflict(clause.OnConflict{
//	    Columns:   []clause.Column{{Name: "email"}},
//	    DoUpdates: clause.AssignmentColumns([]string{"name", "updated_at"}),
//	}).Create(&user)
func (qb *mariadbQueryBuilder) OnConflict(onConflict interface{}) QueryBuilder {
	if oc, ok := onConflict.(clause.OnConflict); ok {
		qb.db = qb.db.Clauses(oc)
	}
	return qb
}

// Clauses adds custom clauses to the query.
// This is a generic method for adding any GORM clause type.
//
// Parameters:
//   - conds: One or more clause expressions
//
// Returns the QueryBuilder for method chaining.
//
// Example:
//
//	qb.Clauses(clause.OrderBy{
//	    Expression: clause.Expr{SQL: "RAND()"},
//	}).Find(&users) // Random order (MySQL uses RAND())
//
//	qb.Clauses(clause.GroupBy{
//	    Columns: []clause.Column{{Name: "department"}},
//	}).Find(&users)
func (qb *mariadbQueryBuilder) Clauses(conds ...interface{}) QueryBuilder {
	// Convert interface{} to clause.Expression
	var clauseExprs []clause.Expression
	for _, cond := range conds {
		if expr, ok := cond.(clause.Expression); ok {
			clauseExprs = append(clauseExprs, expr)
		}
	}
	qb.db = qb.db.Clauses(clauseExprs...)
	return qb
}

// Create inserts a new record into the database.
// This is a terminal method that executes the operation and finalizes the builder.
// It can be combined with OnConflict() for UPSERT operations and other query builder methods.
//
// Parameters:
//   - value: Pointer to the struct or slice of structs to create
//
// Returns:
//   - int64: Number of rows affected (records created)
//   - error: GORM error if the operation fails, nil on success
//
// Example:
//
//	user := User{Name: "John", Email: "john@example.com"}
//	rowsAffected, err := qb.Create(&user)
//	if err != nil {
//	    return err
//	}
//	fmt.Printf("Created %d record, ID: %d\n", rowsAffected, user.ID)
//
// With OnConflict for idempotent create:
//
//	rowsAffected, err := qb.OnConflict(clause.OnConflict{DoNothing: true}).Create(&stage)
//	if rowsAffected == 0 {
//	    // Record already exists
//	}
func (qb *mariadbQueryBuilder) Create(value interface{}) (int64, error) {
	defer qb.release()
	start := time.Now()
	result := qb.db.Create(value)
	table := ""
	if result != nil && result.Statement != nil {
		table = result.Statement.Table
	}
	if qb.m != nil {
		qb.m.observeOperation("create", table, "", time.Since(start), result.Error, result.RowsAffected, nil)
	}
	return result.RowsAffected, result.Error
}

// CreateInBatches creates records in batches to avoid memory issues with large datasets.
// This is a terminal method that executes the operation and finalizes the builder.
//
// Parameters:
//   - value: Slice of records to create
//   - batchSize: Number of records to process in each batch
//
// Returns:
//   - int64: Number of rows affected (records created)
//   - error: GORM error if the operation fails, nil on success
//
// Example:
//
//	users := []User{{Name: "John"}, {Name: "Jane"}, {Name: "Bob"}}
//	rowsAffected, err := qb.CreateInBatches(&users, 100)
//	if err != nil {
//	    return err
//	}
//	fmt.Printf("Created %d records\n", rowsAffected)
func (qb *mariadbQueryBuilder) CreateInBatches(value interface{}, batchSize int) (int64, error) {
	defer qb.release()
	start := time.Now()
	result := qb.db.CreateInBatches(value, batchSize)
	table := ""
	if result != nil && result.Statement != nil {
		table = result.Statement.Table
	}
	if qb.m != nil {
		qb.m.observeOperation("create_in_batches", table, "", time.Since(start), result.Error, result.RowsAffected, map[string]interface{}{
			"batch_size": batchSize,
		})
	}
	return result.RowsAffected, result.Error
}

// FirstOrInit finds the first record matching the conditions, or initializes
// a new one if not found. This is a terminal method.
//
// Parameters:
//   - dest: Pointer to the struct where the result will be stored
//   - conds: Optional conditions for the query
//
// Returns a GORM error if the operation fails or nil on success.
//
// Example:
//
//	var user User
//	err := qb.Where("email = ?", "user@example.com").FirstOrInit(&user)
func (qb *mariadbQueryBuilder) FirstOrInit(dest interface{}, conds ...interface{}) error {
	defer qb.release()
	start := time.Now()
	result := qb.db.FirstOrInit(dest, conds...)
	table := ""
	if result != nil && result.Statement != nil {
		table = result.Statement.Table
	}
	if qb.m != nil {
		qb.m.observeOperation("first_or_init", table, "", time.Since(start), result.Error, result.RowsAffected, nil)
	}
	return result.Error
}

// FirstOrCreate finds the first record matching the conditions, or creates
// a new one if not found. This is a terminal method.
//
// Parameters:
//   - dest: Pointer to the struct where the result will be stored
//   - conds: Optional conditions for the query
//
// Returns a GORM error if the operation fails or nil on success.
//
// Example:
//
//	var user User
//	err := qb.Where("email = ?", "user@example.com").FirstOrCreate(&user)
func (qb *mariadbQueryBuilder) FirstOrCreate(dest interface{}, conds ...interface{}) error {
	defer qb.release()
	start := time.Now()
	result := qb.db.FirstOrCreate(dest, conds...)
	table := ""
	if result != nil && result.Statement != nil {
		table = result.Statement.Table
	}
	if qb.m != nil {
		qb.m.observeOperation("first_or_create", table, "", time.Since(start), result.Error, result.RowsAffected, nil)
	}
	return result.Error
}

// Done finalizes the builder without executing a terminal operation.
// This exists mostly for symmetry and future-proofing; currently it is a no-op.
//
// Example:
//
//	qb := db.Query(ctx)
//	if someCondition {
//	    err := qb.Where(...).Find(&results)
//	} else {
//	    qb.Done() // Release the lock without executing
//	}
func (qb *mariadbQueryBuilder) Done() {
	qb.release()
}

// ToSubquery returns the underlying GORM DB for use as a subquery and finalizes the builder.
// This method is specifically designed for creating subqueries that can be passed to
// Where(), Having(), or other clauses that accept subqueries.
//
// Important: This method finalizes the builder immediately, so the returned *gorm.DB should
// be used as a subquery argument right away.
//
// Returns:
//   - *gorm.DB: The underlying GORM DB instance configured with the query chain
//
// Example:
//
//	// Find users whose email is in a subquery of active accounts
//	activeEmails := db.Query(ctx).
//	    Model(&Account{}).
//	    Select("email").
//	    Where("status = ?", "active").
//	    ToSubquery()
//
//	var users []User
//	err := db.Query(ctx).
//	    Where("email IN (?)", activeEmails).
//	    Find(&users)
//
// Complex example with multiple subqueries:
//
//	// Find stages with no files
//	stageIDsWithFiles := db.Query(ctx).
//	    Model(&File{}).
//	    Select("DISTINCT stage_id").
//	    ToSubquery()
//
//	err := db.Query(ctx).
//	    Model(&Stage{}).
//	    Where("stage_id NOT IN (?)", stageIDsWithFiles).
//	    Find(&stages)
func (qb *mariadbQueryBuilder) ToSubquery() *gorm.DB {
	defer qb.release()
	return qb.db
}
