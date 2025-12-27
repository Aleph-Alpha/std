package postgres

import (
	"context"
	"time"
)

// Find retrieves records from the database that match the given conditions.
// It populates the dest parameter with the query results.
//
// Parameters:
//   - ctx: Context for the database operation
//   - dest: Pointer to a slice where the results will be stored
//   - conditions: Optional query conditions (follows GORM conventions)
//
// Returns a GORM error if the query fails or nil on success.
// Use TranslateError() to convert to standardized error types if needed.
//
// Example:
//
//	var users []User
//	err := db.Find(ctx, &users, "name LIKE ?", "%john%")
func (p *Postgres) Find(ctx context.Context, dest interface{}, conditions ...interface{}) error {
	start := time.Now()
	db := p.DB().WithContext(ctx)
	result := db.Find(dest, conditions...)
	table := ""
	if result != nil && result.Statement != nil {
		table = result.Statement.Table
	}
	p.observeOperation("find", table, "", time.Since(start), result.Error, result.RowsAffected, nil)
	return result.Error
}

// First retrieves the first record that matches the given conditions.
// It populates the dest parameter with the result or returns an error if no matching record exists.
//
// Parameters:
//   - ctx: Context for the database operation
//   - dest: Pointer to a struct where the result will be stored
//   - conditions: Optional query conditions (follows GORM conventions)
//
// Returns gorm.ErrRecordNotFound if no matching record exists, or another GORM error if the query fails.
// Use TranslateError() to convert to standardized error types if needed.
//
// Example:
//
//	var user User
//	err := db.First(ctx, &user, "email = ?", "user@example.com")
//	if errors.Is(err, gorm.ErrRecordNotFound) {
//	    // Handle not found
//	}
func (p *Postgres) First(ctx context.Context, dest interface{}, conditions ...interface{}) error {
	start := time.Now()
	db := p.DB().WithContext(ctx)
	result := db.First(dest, conditions...)
	table := ""
	if result != nil && result.Statement != nil {
		table = result.Statement.Table
	}
	p.observeOperation("first", table, "", time.Since(start), result.Error, result.RowsAffected, nil)
	return result.Error
}

// Create inserts a new record into the database.
// It processes the value parameter according to GORM conventions, performing hooks
// and validations defined on the model.
//
// Parameters:
//   - ctx: Context for the database operation
//   - value: The struct or slice of structs to be created
//
// Returns a GORM error if the creation fails or nil on success.
// Use TranslateError() to convert to standardized error types if needed.
//
// Example:
//
//	user := User{Name: "John", Email: "john@example.com"}
//	err := db.Create(ctx, &user)
func (p *Postgres) Create(ctx context.Context, value interface{}) error {
	start := time.Now()
	db := p.DB().WithContext(ctx)
	result := db.Create(value)
	table := ""
	if result != nil && result.Statement != nil {
		table = result.Statement.Table
	}
	p.observeOperation("create", table, "", time.Since(start), result.Error, result.RowsAffected, nil)
	return result.Error
}

// Save updates the database record if the primary key exists,
// otherwise it creates a new record. It performs a full update
// of all fields, not just changed fields.
//
// Parameters:
//   - ctx: Context for the database operation
//   - value: The struct to be saved
//
// Returns a GORM error if the operation fails or nil on success.
// Use TranslateError() to convert to standardized error types if needed.
//
// Example:
//
//	user.Name = "Updated Name"
//	err := db.Save(ctx, &user)
func (p *Postgres) Save(ctx context.Context, value interface{}) error {
	start := time.Now()
	db := p.DB().WithContext(ctx)
	result := db.Save(value)
	table := ""
	if result != nil && result.Statement != nil {
		table = result.Statement.Table
	}
	p.observeOperation("save", table, "", time.Since(start), result.Error, result.RowsAffected, nil)
	return result.Error
}

// Update updates records that match the given model's non-zero fields or primary key.
// It only updates the fields provided in attrs and only affects records that match
// the model's primary key or query conditions.
//
// Parameters:
//   - ctx: Context for the database operation
//   - model: The model instance with primary key set, or struct with query conditions
//   - attrs: Map, struct, or individual field values to update
//
// Returns:
//   - int64: Number of rows affected by the update operation
//   - error: GORM error if the update fails, nil on success
//
// Note: The current implementation has a bug where it executes the query twice.
// This should be fixed to execute only once and return both values properly.
//
// Example:
//
//	// Update user with ID=1
//	rowsAffected, err := db.Update(ctx, &User{ID: 1}, map[string]interface{}{
//	    "name": "New Name",
//	    "age": 30,
//	})
//	if err != nil {
//	    return err
//	}
//	fmt.Printf("Updated %d rows\n", rowsAffected)
func (p *Postgres) Update(ctx context.Context, model interface{}, attrs interface{}) (int64, error) {
	start := time.Now()
	db := p.DB().WithContext(ctx)
	result := db.Model(model).Updates(attrs)
	table := ""
	if result != nil && result.Statement != nil {
		table = result.Statement.Table
	}
	p.observeOperation("update", table, "", time.Since(start), result.Error, result.RowsAffected, nil)
	return result.RowsAffected, result.Error
}

// UpdateColumn updates a single column's value for records that match the given model.
// Unlike Update, it doesn't run hooks and can be used to update fields that are
// zero values (like setting a string to empty or a number to zero).
//
// Parameters:
//   - ctx: Context for the database operation
//   - model: The model instance with primary key set, or struct with query conditions
//   - columnName: Name of the column to update
//   - value: New value for the column
//
// Returns:
//   - int64: Number of rows affected by the update operation
//   - error: GORM error if the update fails, nil on success
//
// Example:
//
//	// Set status to "inactive" for user with ID=1
//	rowsAffected, err := db.UpdateColumn(ctx, &User{ID: 1}, "status", "inactive")
//	if err != nil {
//	    return err
//	}
//	fmt.Printf("Updated %d rows\n", rowsAffected)
func (p *Postgres) UpdateColumn(ctx context.Context, model interface{}, columnName string, value interface{}) (int64, error) {
	start := time.Now()
	db := p.DB().WithContext(ctx)
	result := db.Model(model).Update(columnName, value)
	table := ""
	if result != nil && result.Statement != nil {
		table = result.Statement.Table
	}
	p.observeOperation("update_column", table, "", time.Since(start), result.Error, result.RowsAffected, nil)
	return result.RowsAffected, result.Error
}

// UpdateColumns updates multiple columns with name/value pairs for records that match the given model.
// Like UpdateColumn, it doesn't run hooks and can update zero-value fields.
//
// Parameters:
//   - ctx: Context for the database operation
//   - model: The model instance with primary key set, or struct with query conditions
//   - columnValues: Map of column names to their new values
//
// Returns:
//   - int64: Number of rows affected by the update operation
//   - error: GORM error if the update fails, nil on success
//
// Example:
//
//	// Update multiple fields for user with ID=1
//	rowsAffected, err := db.UpdateColumns(ctx, &User{ID: 1}, map[string]interface{}{
//	    "status": "inactive",
//	    "last_login": time.Now(),
//	})
//	if err != nil {
//	    return err
//	}
//	fmt.Printf("Updated %d rows\n", rowsAffected)
func (p *Postgres) UpdateColumns(ctx context.Context, model interface{}, columnValues map[string]interface{}) (int64, error) {
	start := time.Now()
	db := p.DB().WithContext(ctx)
	result := db.Model(model).Updates(columnValues)
	table := ""
	if result != nil && result.Statement != nil {
		table = result.Statement.Table
	}
	p.observeOperation("update_columns", table, "", time.Since(start), result.Error, result.RowsAffected, nil)
	return result.RowsAffected, result.Error
}

// Delete removes records that match the given value and conditions from the database.
// It respects soft delete if implemented on the model.
//
// Parameters:
//   - ctx: Context for the database operation
//   - value: The model to delete or a slice for batch delete
//   - conditions: Additional conditions to filter records to delete
//
// Returns:
//   - int64: Number of rows affected by the delete operation
//   - error: GORM error if the deletion fails, nil on success
//
// Example:
//
//	// Delete user with ID=1
//	rowsAffected, err := db.Delete(ctx, &User{}, "id = ?", 1)
//	if err != nil {
//	    return err
//	}
//	fmt.Printf("Deleted %d rows\n", rowsAffected)
//
//	// Or with a model instance
//	user := User{ID: 1}
//	rowsAffected, err := db.Delete(ctx, &user)
func (p *Postgres) Delete(ctx context.Context, value interface{}, conditions ...interface{}) (int64, error) {
	start := time.Now()
	db := p.DB().WithContext(ctx)
	result := db.Delete(value, conditions...)
	table := ""
	if result != nil && result.Statement != nil {
		table = result.Statement.Table
	}
	p.observeOperation("delete", table, "", time.Since(start), result.Error, result.RowsAffected, nil)
	return result.RowsAffected, result.Error
}

// Exec executes raw SQL directly against the database.
// This is useful for operations not easily expressed through GORM's API
// or for performance-critical code.
//
// Parameters:
//   - ctx: Context for the database operation
//   - sql: The SQL statement to execute
//   - values: Parameters for the SQL statement
//
// Returns:
//   - int64: Number of rows affected by the SQL execution
//   - error: GORM error if the execution fails, nil on success
//
// Example:
//
//	rowsAffected, err := db.Exec(ctx, "UPDATE users SET status = ? WHERE last_login < ?",
//	                             "inactive", time.Now().AddDate(0, -6, 0))
//	if err != nil {
//	    return err
//	}
//	fmt.Printf("Updated %d users\n", rowsAffected)
func (p *Postgres) Exec(ctx context.Context, sql string, values ...interface{}) (int64, error) {
	start := time.Now()
	db := p.DB().WithContext(ctx)
	result := db.Exec(sql, values...)
	table := ""
	if result != nil && result.Statement != nil {
		table = result.Statement.Table
	}
	p.observeOperation("exec", table, "", time.Since(start), result.Error, result.RowsAffected, map[string]interface{}{
		"sql": sql,
	})
	return result.RowsAffected, result.Error
}

// Count determines the number of records that match the given conditions.
// It populates the count parameter with the result.
//
// Parameters:
//   - ctx: Context for the database operation
//   - model: The model type to count
//   - count: Pointer to an int64 where the count will be stored
//   - conditions: Query conditions to filter the records to count
//
// Returns a GORM error if the query fails or nil on success.
// Use TranslateError() to convert to standardized error types if needed.
//
// Example:
//
//	var count int64
//	err := db.Count(ctx, &User{}, &count, "age > ?", 18)
func (p *Postgres) Count(ctx context.Context, model interface{}, count *int64, conditions ...interface{}) error {
	start := time.Now()
	db := p.DB().WithContext(ctx).Model(model)
	var err error
	if len(conditions) == 0 {
		err = db.Count(count).Error
	} else {
		err = db.Where(conditions[0], conditions[1:]...).Count(count).Error
	}
	table := ""
	if db != nil && db.Statement != nil {
		table = db.Statement.Table
	}
	p.observeOperation("count", table, "", time.Since(start), err, *count, nil)
	return err
}

// UpdateWhere updates records that match the specified WHERE condition.
// This method provides more flexibility than Update for complex conditions.
//
// Parameters:
//   - ctx: Context for the database operation
//   - model: The model type to update
//   - attrs: Fields to update (map, struct, or name/value pairs)
//   - condition: WHERE condition as a string
//   - args: Arguments for the WHERE condition
//
// Returns:
//   - int64: Number of rows affected by the update operation
//   - error: GORM error if the update fails, nil on success
//
// Example:
//
//	// Update all users who haven't logged in for 6 months
//	rowsAffected, err := db.UpdateWhere(ctx, &User{},
//	                                    map[string]interface{}{"status": "inactive"},
//	                                    "last_login < ?",
//	                                    time.Now().AddDate(0, -6, 0))
//	if err != nil {
//	    return err
//	}
//	fmt.Printf("Updated %d users to inactive status\n", rowsAffected)
func (p *Postgres) UpdateWhere(ctx context.Context, model interface{}, attrs interface{}, condition string, args ...interface{}) (int64, error) {
	start := time.Now()
	db := p.DB().WithContext(ctx)
	result := db.Model(model).Where(condition, args...).Updates(attrs)
	table := ""
	if result != nil && result.Statement != nil {
		table = result.Statement.Table
	}
	p.observeOperation("update_where", table, "", time.Since(start), result.Error, result.RowsAffected, nil)
	return result.RowsAffected, result.Error
}
