package postgres

import (
	"time"

	"gorm.io/gorm"
)

// RowScanner provides an interface for scanning a single row of data.
// It abstracts the process of parsing column values into Go variables,
// allowing for efficient handling of individual rows returned from a query.
type RowScanner interface {
	// Scan copies the column values from the current row into the values pointed to by dest.
	// The number of values in dest must match the number of columns in the row.
	Scan(dest ...interface{}) error
}

// RowsScanner provides an interface for iterating through rows of data returned by a query.
// It extends RowScanner functionality with methods for navigation and error handling,
// allowing for efficient processing of result sets with multiple rows.
type RowsScanner interface {
	// Next prepares the next row for reading. It returns false when there are no more rows.
	Next() bool

	// Scan copies column values from the current row into the provided destination variables.
	Scan(dest ...interface{}) error

	// Close closes the rows iterator, releasing any associated resources.
	Close() error

	// Err returns any error encountered during iteration.
	Err() error
}

// QueryRow executes a query expected to return a single row and returns a RowScanner for it.
// This method is optimized for queries that return exactly one row of data and provides
// a simplified interface for scanning the values from that row.
func (qb *postgresQueryBuilder) QueryRow() RowScanner {
	defer qb.release()
	return qb.db.Row()
}

// QueryRows executes a query that returns multiple rows and returns a RowsScanner for them.
// This method provides an iterator-style interface for processing multiple rows
// returned by a query, allowing for efficient traversal of large result sets.
//
// Returns a RowsScanner and a GORM error if the query fails.
func (qb *postgresQueryBuilder) QueryRows() (RowsScanner, error) {
	defer qb.release()
	start := time.Now()
	rows, err := qb.db.Rows()
	table := ""
	if qb.db != nil && qb.db.Statement != nil {
		table = qb.db.Statement.Table
	}
	if qb.pg != nil {
		qb.pg.observeOperation("rows", table, "", time.Since(start), err, 0, nil)
	}
	return rows, err
}

// ScanRow is a convenience method to scan a single row directly into a struct.
// This is a higher-level alternative to QueryRow that automatically maps
// column values to struct fields based on naming conventions or field tags.
// It's useful when you need to map a row to a predefined data structure.
//
// Returns a GORM error if the scan fails or nil on success.
func (qb *postgresQueryBuilder) ScanRow(dest interface{}) error {
	defer qb.release()
	start := time.Now()
	result := qb.db.Scan(dest)
	table := ""
	if result != nil && result.Statement != nil {
		table = result.Statement.Table
	}
	if qb.pg != nil {
		qb.pg.observeOperation("scan_row", table, "", time.Since(start), result.Error, result.RowsAffected, nil)
	}
	return result.Error
}

// MapRows executes a query and maps all rows into a destination slice using the provided mapping function.
// This provides a higher-level abstraction than working with raw rows, allowing
// for custom mapping logic while still handling the query execution and
// resource management automatically.
//
// Parameters:
//   - destSlice: The slice to populate with mapped rows (should be a pointer to a slice)
//   - mapFn: A function that defines how to map rows from the database to your slice items
//
// Returns a GORM error if the mapping fails or nil on success.
func (qb *postgresQueryBuilder) MapRows(destSlice interface{}, mapFn func(*gorm.DB) error) error {
	defer qb.release()
	return mapFn(qb.db)
}
