package postgres

import (
	"errors"
	"gorm.io/gorm"
)

// Common database error types that can be used by consumers of this package.
// These provide a standardized set of errors that abstract away the
// underlying database-specific error details.
var (
	// ErrRecordNotFound is returned when a query doesn't find any matching records
	ErrRecordNotFound = errors.New("record not found")

	// ErrDuplicateKey is returned when an insert or update violates a unique constraint
	ErrDuplicateKey = errors.New("duplicate key violation")

	// ErrForeignKey is returned when an operation violates a foreign key constraint
	ErrForeignKey = errors.New("foreign key violation")

	// ErrInvalidData is returned when the data being saved doesn't meet validation rules
	ErrInvalidData = errors.New("invalid data")
)

// TranslateError converts GORM/database-specific errors into standardized application errors.
// This function provides abstraction from the underlying database implementation details,
// allowing application code to handle errors in a database-agnostic way.
//
// It maps common database errors to the standardized error types defined above.
// If an error doesn't match any known type, it's returned unchanged.
func TranslateError(err error) error {
	if err == nil {
		return nil
	}

	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		return ErrRecordNotFound
	case errors.Is(err, gorm.ErrDuplicatedKey):
		return ErrDuplicateKey
	case errors.Is(err, gorm.ErrForeignKeyViolated):
		return ErrForeignKey
	case errors.Is(err, gorm.ErrInvalidData):
		return ErrInvalidData

	}

	// You can add more translations based on PostgresSQL error codes if needed

	return err
}
