package postgres

import (
	"errors"
	"gorm.io/gorm"
)

// Common database error types that can be used by consumers of this package
var (
	ErrRecordNotFound = errors.New("record not found")
	ErrDuplicateKey   = errors.New("duplicate key violation")
	ErrForeignKey     = errors.New("foreign key violation")
	ErrInvalidData    = errors.New("invalid data")
)

// TranslateError converts GORM/database-specific errors into application errors
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
	}

	// You can add more translations based on PostgresSQL error codes if needed

	return err
}
