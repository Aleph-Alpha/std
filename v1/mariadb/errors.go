package mariadb

import (
	"errors"
	"strings"

	"github.com/go-sql-driver/mysql"
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

	// ErrConnectionFailed is returned when database connection cannot be established
	ErrConnectionFailed = errors.New("database connection failed")

	// ErrTransactionFailed is returned when a transaction fails to commit or rollback
	ErrTransactionFailed = errors.New("transaction failed")

	// ErrQueryTimeout is returned when a query exceeds the allowed timeout
	ErrQueryTimeout = errors.New("query timeout exceeded")

	// ErrInvalidQuery is returned when the SQL query is malformed or invalid
	ErrInvalidQuery = errors.New("invalid query")

	// ErrPermissionDenied is returned when the user lacks necessary permissions
	ErrPermissionDenied = errors.New("permission denied")

	// ErrTableNotFound is returned when trying to access a non-existent table
	ErrTableNotFound = errors.New("table not found")

	// ErrColumnNotFound is returned when trying to access a non-existent column
	ErrColumnNotFound = errors.New("column not found")

	// ErrConstraintViolation is returned for general constraint violations
	ErrConstraintViolation = errors.New("constraint violation")

	// ErrCheckConstraintViolation is returned when a check constraint is violated
	ErrCheckConstraintViolation = errors.New("check constraint violation")

	// ErrNotNullViolation is returned when trying to insert null into a not-null column
	ErrNotNullViolation = errors.New("not null constraint violation")

	// ErrDataTooLong is returned when data exceeds column length limits
	ErrDataTooLong = errors.New("data too long for column")

	// ErrDeadlock is returned when a deadlock is detected during transaction
	ErrDeadlock = errors.New("deadlock detected")

	// ErrLockTimeout is returned when unable to acquire lock within timeout
	ErrLockTimeout = errors.New("lock acquisition timeout")

	// ErrInvalidDataType is returned when data type conversion fails
	ErrInvalidDataType = errors.New("invalid data type")

	// ErrDivisionByZero is returned for division by zero operations
	ErrDivisionByZero = errors.New("division by zero")

	// ErrNumericOverflow is returned when numeric operation causes overflow
	ErrNumericOverflow = errors.New("numeric value overflow")

	// ErrDiskFull is returned when database storage is full
	ErrDiskFull = errors.New("disk full")

	// ErrTooManyConnections is returned when connection pool is exhausted
	ErrTooManyConnections = errors.New("too many connections")

	// ErrInvalidJSON is returned when JSON data is malformed
	ErrInvalidJSON = errors.New("invalid JSON data")

	// ErrIndexCorruption is returned when database index is corrupted
	ErrIndexCorruption = errors.New("index corruption detected")

	// ErrConfigurationError is returned for database configuration issues
	ErrConfigurationError = errors.New("database configuration error")

	// ErrUnsupportedOperation is returned for operations not supported by the database
	ErrUnsupportedOperation = errors.New("unsupported operation")

	// ErrMigrationFailed is returned when database migration fails
	ErrMigrationFailed = errors.New("migration failed")

	// ErrBackupFailed is returned when database backup operation fails
	ErrBackupFailed = errors.New("backup operation failed")

	// ErrRestoreFailed is returned when database restore operation fails
	ErrRestoreFailed = errors.New("restore operation failed")

	// ErrSchemaValidation is returned when schema validation fails
	ErrSchemaValidation = errors.New("schema validation failed")

	// ErrSerializationFailure is returned when transaction serialization fails
	ErrSerializationFailure = errors.New("serialization failure")

	// ErrInsufficientPrivileges is returned when user lacks required privileges
	ErrInsufficientPrivileges = errors.New("insufficient privileges")

	// ErrInvalidPassword is returned for authentication failures
	ErrInvalidPassword = errors.New("invalid password")

	// ErrAccountLocked is returned when user account is locked
	ErrAccountLocked = errors.New("account locked")

	// ErrDatabaseNotFound is returned when specified database doesn't exist
	ErrDatabaseNotFound = errors.New("database not found")

	// ErrSchemaNotFound is returned when specified schema doesn't exist
	ErrSchemaNotFound = errors.New("schema not found")

	// ErrFunctionNotFound is returned when specified function doesn't exist
	ErrFunctionNotFound = errors.New("function not found")

	// ErrTriggerNotFound is returned when specified trigger doesn't exist
	ErrTriggerNotFound = errors.New("trigger not found")

	// ErrIndexNotFound is returned when specified index doesn't exist
	ErrIndexNotFound = errors.New("index not found")

	// ErrViewNotFound is returned when specified view doesn't exist
	ErrViewNotFound = errors.New("view not found")

	// ErrSequenceNotFound is returned when specified sequence doesn't exist
	ErrSequenceNotFound = errors.New("sequence not found")

	// ErrInvalidCursor is returned when cursor operation fails
	ErrInvalidCursor = errors.New("invalid cursor")

	// ErrCursorNotFound is returned when specified cursor doesn't exist
	ErrCursorNotFound = errors.New("cursor not found")

	// ErrStatementTimeout is returned when statement execution exceeds timeout
	ErrStatementTimeout = errors.New("statement timeout")

	// ErrIdleInTransaction is returned when transaction is idle too long
	ErrIdleInTransaction = errors.New("idle in transaction timeout")

	// ErrConnectionLost is returned when database connection is lost
	ErrConnectionLost = errors.New("connection lost")

	// ErrProtocolViolation is returned for database protocol violations
	ErrProtocolViolation = errors.New("protocol violation")

	// ErrInternalError is returned for internal database errors
	ErrInternalError = errors.New("internal database error")

	// ErrSystemError is returned for system-level database errors
	ErrSystemError = errors.New("system error")
)

// TranslateError converts GORM/database-specific errors into standardized application errors.
// This function provides abstraction from the underlying database implementation details,
// allowing application code to handle errors in a database-agnostic way.
//
// It maps common database errors to the standardized error types defined above.
// If an error doesn't match any known type, it's returned unchanged.
func (m *MariaDB) TranslateError(err error) error {
	if err == nil {
		return nil
	}

	// First check GORM specific errors
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		return ErrRecordNotFound
	case errors.Is(err, gorm.ErrDuplicatedKey):
		return ErrDuplicateKey
	case errors.Is(err, gorm.ErrForeignKeyViolated):
		return ErrForeignKey
	case errors.Is(err, gorm.ErrInvalidData):
		return ErrInvalidData
	case errors.Is(err, gorm.ErrInvalidTransaction):
		return ErrTransactionFailed
	case errors.Is(err, gorm.ErrNotImplemented):
		return ErrUnsupportedOperation
	case errors.Is(err, gorm.ErrMissingWhereClause):
		return ErrInvalidQuery
	case errors.Is(err, gorm.ErrUnsupportedRelation):
		return ErrUnsupportedOperation
	case errors.Is(err, gorm.ErrPrimaryKeyRequired):
		return ErrConstraintViolation
	case errors.Is(err, gorm.ErrModelValueRequired):
		return ErrInvalidData
	case errors.Is(err, gorm.ErrInvalidField):
		return ErrColumnNotFound
	case errors.Is(err, gorm.ErrEmptySlice):
		return ErrInvalidData
	case errors.Is(err, gorm.ErrDryRunModeUnsupported):
		return ErrUnsupportedOperation
	case errors.Is(err, gorm.ErrInvalidDB):
		return ErrConfigurationError
	case errors.Is(err, gorm.ErrInvalidValue):
		return ErrInvalidData
	case errors.Is(err, gorm.ErrInvalidValueOfLength):
		return ErrDataTooLong
	}

	// Check for MySQL specific errors using go-sql-driver/mysql
	var mysqlErr *mysql.MySQLError
	if errors.As(err, &mysqlErr) {
		return m.translateMySQLError(mysqlErr)
	}

	// Check error message for common patterns (fallback for string matching)
	errMsg := strings.ToLower(err.Error())
	return m.translateByErrorMessage(errMsg, err)
}

// translateMySQLError maps MySQL error codes to custom errors
func (m *MariaDB) translateMySQLError(mysqlErr *mysql.MySQLError) error {
	switch mysqlErr.Number {
	// Duplicate entry errors
	case 1062: // ER_DUP_ENTRY
		return ErrDuplicateKey
	case 1586: // ER_DUP_ENTRY_WITH_KEY_NAME
		return ErrDuplicateKey

	// Foreign key errors
	case 1216: // ER_NO_REFERENCED_ROW
		return ErrForeignKey
	case 1217: // ER_ROW_IS_REFERENCED
		return ErrForeignKey
	case 1451: // ER_ROW_IS_REFERENCED_2
		return ErrForeignKey
	case 1452: // ER_NO_REFERENCED_ROW_2
		return ErrForeignKey
	case 1557: // ER_FOREIGN_DUPLICATE_KEY_WITH_CHILD_INFO
		return ErrForeignKey
	case 1761: // ER_FOREIGN_DUPLICATE_KEY_WITHOUT_CHILD_INFO
		return ErrForeignKey

	// Table/column not found errors
	case 1051: // ER_BAD_TABLE_ERROR
		return ErrTableNotFound
	case 1054: // ER_BAD_FIELD_ERROR
		return ErrColumnNotFound
	case 1091: // ER_CANT_DROP_FIELD_OR_KEY
		return ErrColumnNotFound
	case 1146: // ER_NO_SUCH_TABLE
		return ErrTableNotFound

	// Data too long errors
	case 1406: // ER_DATA_TOO_LONG
		return ErrDataTooLong
	case 1264: // ER_WARN_DATA_OUT_OF_RANGE
		return ErrNumericOverflow

	// Constraint violations
	case 1048: // ER_BAD_NULL_ERROR
		return ErrNotNullViolation
	case 1364: // ER_NO_DEFAULT_FOR_FIELD - Missing NOT NULL field without default
		return ErrNotNullViolation
	case 1690: // ER_DATA_OUT_OF_RANGE
		return ErrConstraintViolation
	case 3819: // ER_CHECK_CONSTRAINT_VIOLATED (MySQL 8.0.16+)
		return ErrCheckConstraintViolation
	case 4025: // ER_CHECK_CONSTRAINT_VIOLATED (MariaDB 10.2+)
		return ErrCheckConstraintViolation

	// Deadlock and lock errors
	case 1205: // ER_LOCK_WAIT_TIMEOUT
		return ErrLockTimeout
	case 1213: // ER_LOCK_DEADLOCK
		return ErrDeadlock
	case 1637: // ER_TOO_MANY_CONCURRENT_TRXS
		return ErrDeadlock

	// Connection errors
	case 1040: // ER_CON_COUNT_ERROR
		return ErrTooManyConnections
	case 1158: // ER_NET_READ_ERROR
		return ErrConnectionLost
	case 1159: // ER_NET_READ_INTERRUPTED
		return ErrConnectionLost
	case 1160: // ER_NET_ERROR_ON_WRITE
		return ErrConnectionLost
	case 1161: // ER_NET_WRITE_INTERRUPTED
		return ErrConnectionLost
	case 2002: // CR_CONNECTION_ERROR
		return ErrConnectionFailed
	case 2003: // CR_CONN_HOST_ERROR
		return ErrConnectionFailed
	case 2006: // CR_SERVER_GONE_ERROR
		return ErrConnectionLost
	case 2013: // CR_SERVER_LOST
		return ErrConnectionLost
	case 2055: // CR_SERVER_LOST_EXTENDED
		return ErrConnectionLost

	// Syntax and query errors
	case 1064: // ER_PARSE_ERROR
		return ErrInvalidQuery
	case 1065: // ER_EMPTY_QUERY
		return ErrInvalidQuery
	case 1149: // ER_SYNTAX_ERROR
		return ErrInvalidQuery

	// Permission and access errors
	case 1044: // ER_DBACCESS_DENIED_ERROR
		return ErrPermissionDenied
	case 1045: // ER_ACCESS_DENIED_ERROR
		return ErrInvalidPassword
	case 1142: // ER_TABLEACCESS_DENIED_ERROR
		return ErrPermissionDenied
	case 1143: // ER_COLUMNACCESS_DENIED_ERROR
		return ErrPermissionDenied
	case 1227: // ER_SPECIFIC_ACCESS_DENIED_ERROR
		return ErrInsufficientPrivileges

	// Transaction errors
	case 1568: // ER_COMMIT_NOT_ALLOWED_IN_SF_OR_TRG
		return ErrTransactionFailed
	case 1792: // ER_CANT_EXECUTE_IN_READ_ONLY_TRANSACTION
		return ErrTransactionFailed

	// Division by zero
	case 1365: // ER_DIVISION_BY_ZERO
		return ErrDivisionByZero

	// JSON errors
	case 3140: // ER_INVALID_JSON_TEXT
		return ErrInvalidJSON
	case 3141: // ER_INVALID_JSON_TEXT_IN_PARAM
		return ErrInvalidJSON
	case 3143: // ER_INVALID_JSON_BINARY_DATA
		return ErrInvalidJSON

	// Database not found
	case 1049: // ER_BAD_DB_ERROR
		return ErrDatabaseNotFound
	case 1008: // ER_DB_DROP_EXISTS
		return ErrDatabaseNotFound

	// Function/procedure not found
	case 1305: // ER_SP_DOES_NOT_EXIST
		return ErrFunctionNotFound
	case 1370: // ER_PROC_AUTO_GRANT_FAIL
		return ErrFunctionNotFound

	// Index errors
	case 1061: // ER_DUP_KEYNAME
		return ErrDuplicateKey
	case 1176: // ER_KEY_DOES_NOT_EXISTS
		return ErrIndexNotFound

	// View errors - Note: 1347 ER_WRONG_OBJECT is handled above as table not found
	// case 1347: // ER_WRONG_OBJECT (when it's a view issue)
	//	return ErrViewNotFound

	// Disk full and system errors
	case 3: // ER_FILE_NOT_FOUND
		return ErrSystemError
	case 9: // ER_OUT_OF_RESOURCES
		return ErrSystemError
	case 1021: // ER_DISK_FULL
		return ErrDiskFull

	// Timeout errors
	case 1969: // ER_STATEMENT_TIMEOUT
		return ErrStatementTimeout

	// Data type errors
	case 1366: // ER_TRUNCATED_WRONG_VALUE_FOR_FIELD
		return ErrInvalidDataType
	case 1582: // ER_INCORRECT_PARAMETERS_TO_NATIVE_FCT
		return ErrInvalidDataType

	default:
		// If we don't recognize the error code, return a system error
		return ErrSystemError
	}
}

// translateByErrorMessage translates errors based on error message patterns (fallback)
func (m *MariaDB) translateByErrorMessage(errMsg string, originalErr error) error {
	switch {
	// Connection related
	case strings.Contains(errMsg, "connection refused"):
		return ErrConnectionFailed
	case strings.Contains(errMsg, "connection reset"):
		return ErrConnectionLost
	case strings.Contains(errMsg, "too many connections"):
		return ErrTooManyConnections
	case strings.Contains(errMsg, "connection timed out"):
		return ErrConnectionFailed
	case strings.Contains(errMsg, "server has gone away"):
		return ErrConnectionLost
	case strings.Contains(errMsg, "broken pipe"):
		return ErrConnectionLost

	// Lock and deadlock
	case strings.Contains(errMsg, "deadlock"):
		return ErrDeadlock
	case strings.Contains(errMsg, "lock wait timeout"):
		return ErrLockTimeout
	case strings.Contains(errMsg, "try restarting transaction"):
		return ErrDeadlock

	// Data related
	case strings.Contains(errMsg, "data too long"):
		return ErrDataTooLong
	case strings.Contains(errMsg, "out of range"):
		return ErrNumericOverflow
	case strings.Contains(errMsg, "invalid json"):
		return ErrInvalidJSON
	case strings.Contains(errMsg, "division by zero"):
		return ErrDivisionByZero
	case strings.Contains(errMsg, "incorrect"):
		return ErrInvalidDataType

	// Constraint related
	case strings.Contains(errMsg, "duplicate entry"):
		return ErrDuplicateKey
	case strings.Contains(errMsg, "duplicate key"):
		return ErrDuplicateKey
	case strings.Contains(errMsg, "cannot be null"):
		return ErrNotNullViolation
	case strings.Contains(errMsg, "foreign key constraint"):
		return ErrForeignKey
	case strings.Contains(errMsg, "check constraint"):
		return ErrCheckConstraintViolation

	// Schema related
	case strings.Contains(errMsg, "table") && strings.Contains(errMsg, "doesn't exist"):
		return ErrTableNotFound
	case strings.Contains(errMsg, "unknown column"):
		return ErrColumnNotFound
	case strings.Contains(errMsg, "unknown database"):
		return ErrDatabaseNotFound
	case strings.Contains(errMsg, "no such table"):
		return ErrTableNotFound

	// Permission related
	case strings.Contains(errMsg, "access denied"):
		return ErrPermissionDenied
	case strings.Contains(errMsg, "command denied"):
		return ErrInsufficientPrivileges

	// System related
	case strings.Contains(errMsg, "disk full"):
		return ErrDiskFull
	case strings.Contains(errMsg, "out of memory"):
		return ErrSystemError

	default:
		// Return the original error if no pattern matches
		return originalErr
	}
}

// ErrorCategory represents different categories of database errors
type ErrorCategory int

const (
	CategoryUnknown ErrorCategory = iota
	CategoryConnection
	CategoryQuery
	CategoryData
	CategoryConstraint
	CategoryPermission
	CategoryTransaction
	CategoryResource
	CategorySystem
	CategorySchema
	CategoryOperation
)

// GetErrorCategory returns the category of the given error
func (m *MariaDB) GetErrorCategory(err error) ErrorCategory {
	switch {
	case errors.Is(err, ErrConnectionFailed), errors.Is(err, ErrConnectionLost), errors.Is(err, ErrTooManyConnections):
		return CategoryConnection
	case errors.Is(err, ErrInvalidQuery), errors.Is(err, ErrQueryTimeout), errors.Is(err, ErrStatementTimeout), errors.Is(err, ErrInvalidCursor), errors.Is(err, ErrCursorNotFound):
		return CategoryQuery
	case errors.Is(err, ErrInvalidData), errors.Is(err, ErrDataTooLong), errors.Is(err, ErrInvalidDataType), errors.Is(err, ErrInvalidJSON), errors.Is(err, ErrDivisionByZero), errors.Is(err, ErrNumericOverflow):
		return CategoryData
	case errors.Is(err, ErrDuplicateKey), errors.Is(err, ErrForeignKey), errors.Is(err, ErrConstraintViolation), errors.Is(err, ErrCheckConstraintViolation), errors.Is(err, ErrNotNullViolation):
		return CategoryConstraint
	case errors.Is(err, ErrPermissionDenied), errors.Is(err, ErrInsufficientPrivileges), errors.Is(err, ErrInvalidPassword), errors.Is(err, ErrAccountLocked):
		return CategoryPermission
	case errors.Is(err, ErrTransactionFailed), errors.Is(err, ErrDeadlock), errors.Is(err, ErrSerializationFailure), errors.Is(err, ErrIdleInTransaction):
		return CategoryTransaction
	case errors.Is(err, ErrDiskFull), errors.Is(err, ErrLockTimeout):
		return CategoryResource
	case errors.Is(err, ErrInternalError), errors.Is(err, ErrSystemError), errors.Is(err, ErrIndexCorruption), errors.Is(err, ErrProtocolViolation), errors.Is(err, ErrConfigurationError):
		return CategorySystem
	case errors.Is(err, ErrTableNotFound), errors.Is(err, ErrColumnNotFound), errors.Is(err, ErrDatabaseNotFound), errors.Is(err, ErrSchemaNotFound), errors.Is(err, ErrFunctionNotFound), errors.Is(err, ErrTriggerNotFound), errors.Is(err, ErrIndexNotFound), errors.Is(err, ErrViewNotFound), errors.Is(err, ErrSequenceNotFound):
		return CategorySchema
	case errors.Is(err, ErrUnsupportedOperation), errors.Is(err, ErrMigrationFailed), errors.Is(err, ErrBackupFailed), errors.Is(err, ErrRestoreFailed), errors.Is(err, ErrSchemaValidation), errors.Is(err, ErrRecordNotFound):
		return CategoryOperation
	default:
		return CategoryUnknown
	}
}

// IsRetryable returns true if the error might be resolved by retrying the operation
func (m *MariaDB) IsRetryable(err error) bool {
	retryableErrors := []error{
		ErrConnectionFailed,
		ErrConnectionLost,
		ErrQueryTimeout,
		ErrStatementTimeout,
		ErrDeadlock,
		ErrLockTimeout,
		ErrSerializationFailure,
		ErrTooManyConnections,
		ErrIdleInTransaction,
		ErrProtocolViolation,
		ErrInternalError, // Sometimes internal errors are transient
	}

	for _, retryableErr := range retryableErrors {
		if errors.Is(err, retryableErr) {
			return true
		}
	}
	return false
}

// IsTemporary returns true if the error is likely temporary and might resolve itself
func (m *MariaDB) IsTemporary(err error) bool {
	temporaryErrors := []error{
		ErrConnectionLost,
		ErrQueryTimeout,
		ErrStatementTimeout,
		ErrLockTimeout,
		ErrTooManyConnections,
		ErrDiskFull,
		ErrIdleInTransaction,
		ErrSerializationFailure,
	}

	for _, tempErr := range temporaryErrors {
		if errors.Is(err, tempErr) {
			return true
		}
	}
	return false
}

// IsCritical returns true if the error indicates a serious system problem
func (m *MariaDB) IsCritical(err error) bool {
	criticalErrors := []error{
		ErrIndexCorruption,
		ErrSystemError,
		ErrConfigurationError,
		ErrProtocolViolation,
		ErrDiskFull,
		ErrInternalError,
	}

	for _, criticalErr := range criticalErrors {
		if errors.Is(err, criticalErr) {
			return true
		}
	}
	return false
}
