package postgres

import (
	"gorm.io/gorm"
)

// RowScanner provides an interface for scanning a single row of data
type RowScanner interface {
	Scan(dest ...interface{}) error
}

// RowsScanner provides an interface for iterating through rows of data
type RowsScanner interface {
	Next() bool
	Scan(dest ...interface{}) error
	Close() error
	Err() error
}

// QueryRow executes a query that is expected to return a single row and returns a RowScanner for it
func (qb *QueryBuilder) QueryRow() RowScanner {
	defer qb.release()
	return qb.db.Row()
}

// QueryRows executes a query that returns rows and returns a RowsScanner for them
func (qb *QueryBuilder) QueryRows() (RowsScanner, error) {
	defer qb.release()
	return qb.db.Rows()
}

// ScanRow is a convenience method to scan a single row into a struct
// This is a higher-level alternative to QueryRow that populates a struct directly
func (qb *QueryBuilder) ScanRow(dest interface{}) error {
	defer qb.release()
	return qb.db.Scan(dest).Error
}

// MapRows executes a query and maps all rows into a destination slice
// This provides a higher-level abstraction than working with raw rows
func (qb *QueryBuilder) MapRows(destSlice interface{}, mapFn func(*gorm.DB) error) error {
	defer qb.release()
	return mapFn(qb.db)
}
