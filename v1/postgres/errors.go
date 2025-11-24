package postgres

import (
	"errors"
	"strings"

	"github.com/jackc/pgx/v5/pgconn"
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
func (p *Postgres) TranslateError(err error) error {
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

	// Check for PostgreSQL specific errors using pgconn.PgError
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return p.translatePostgreSQLError(pgErr)
	}

	// Check error message for common patterns (fallback for string matching)
	errMsg := strings.ToLower(err.Error())
	return p.translateByErrorMessage(errMsg, err)
}

// translatePostgreSQLError maps PostgreSQL error codes to custom errors
func (p *Postgres) translatePostgreSQLError(pgErr *pgconn.PgError) error {
	switch pgErr.Code {
	// Class 00 — Successful Completion (shouldn't really happen in error context)

	// Class 01 — Warning
	case "01000": // warning
		return ErrInvalidData

	// Class 02 — No Data
	case "02000": // no_data
		return ErrRecordNotFound
	case "02001": // no_additional_dynamic_result_sets_returned
		return ErrRecordNotFound

	// Class 03 — SQL Statement Not Yet Complete
	case "03000": // sql_statement_not_yet_complete
		return ErrInvalidQuery

	// Class 08 — Connection Exception
	case "08000": // connection_exception
		return ErrConnectionFailed
	case "08001": // sqlclient_unable_to_establish_sqlconnection
		return ErrConnectionFailed
	case "08003": // connection_does_not_exist
		return ErrConnectionLost
	case "08004": // sqlserver_rejected_establishment_of_sqlconnection
		return ErrConnectionFailed
	case "08006": // connection_failure
		return ErrConnectionLost
	case "08007": // transaction_resolution_unknown
		return ErrTransactionFailed
	case "08P01": // protocol_violation
		return ErrProtocolViolation

	// Class 09 — Triggered Action Exception
	case "09000": // triggered_action_exception
		return ErrTriggerNotFound

	// Class 0A — Feature Not Supported
	case "0A000": // feature_not_supported
		return ErrUnsupportedOperation

	// Class 0B — Invalid Transaction Initiation
	case "0B000": // invalid_transaction_initiation
		return ErrTransactionFailed

	// Class 0F — Locator Exception
	case "0F000": // locator_exception
		return ErrInvalidCursor
	case "0F001": // invalid_locator_specification
		return ErrInvalidCursor

	// Class 0L — Invalid Grantor
	case "0L000": // invalid_grantor
		return ErrPermissionDenied
	case "0LP01": // invalid_grant_operation
		return ErrInsufficientPrivileges

	// Class 0P — Invalid Role Specification
	case "0P000": // invalid_role_specification
		return ErrPermissionDenied

	// Class 0Z — Diagnostics Exception
	case "0Z000": // diagnostics_exception
		return ErrSystemError
	case "0Z002": // stacked_diagnostics_accessed_without_active_handler
		return ErrSystemError

	// Class 20 — Case Not Found
	case "20000": // case_not_found
		return ErrRecordNotFound

	// Class 21 — Cardinality Violation
	case "21000": // cardinality_violation
		return ErrConstraintViolation

	// Class 22 — Data Exception
	case "22000": // data_exception
		return ErrInvalidData
	case "22001": // string_data_right_truncation
		return ErrDataTooLong
	case "22002": // null_value_no_indicator_parameter
		return ErrNotNullViolation
	case "22003": // numeric_value_out_of_range
		return ErrNumericOverflow
	case "22004": // null_value_not_allowed
		return ErrNotNullViolation
	case "22005": // error_in_assignment
		return ErrInvalidDataType
	case "22007": // invalid_datetime_format
		return ErrInvalidDataType
	case "22008": // datetime_field_overflow
		return ErrNumericOverflow
	case "22009": // invalid_time_zone_displacement_value
		return ErrInvalidData
	case "22012": // division_by_zero
		return ErrDivisionByZero
	case "22015": // interval_field_overflow
		return ErrNumericOverflow
	case "22018": // invalid_character_value_for_cast
		return ErrInvalidDataType
	case "22019": // invalid_escape_character
		return ErrInvalidData
	case "22021": // character_not_in_repertoire
		return ErrInvalidDataType
	case "22022": // indicator_overflow
		return ErrNumericOverflow
	case "22023": // invalid_parameter_value
		return ErrInvalidData
	case "22024": // unterminated_c_string
		return ErrInvalidData
	case "22025": // invalid_escape_sequence
		return ErrInvalidData
	case "22026": // string_data_length_mismatch
		return ErrDataTooLong
	case "22027": // trim_error
		return ErrInvalidData
	case "2202E": // array_subscript_error
		return ErrInvalidData
	case "2202F": // zero_length_character_string
		return ErrInvalidData
	case "22030": // duplicate_json_object_key_value
		return ErrInvalidJSON
	case "22032": // invalid_json_text
		return ErrInvalidJSON
	case "22033": // invalid_sql_json_subscript
		return ErrInvalidJSON
	case "22034": // more_than_one_sql_json_item
		return ErrInvalidJSON
	case "22035": // no_sql_json_item
		return ErrInvalidJSON
	case "22036": // non_numeric_sql_json_item
		return ErrInvalidJSON
	case "22037": // non_unique_keys_in_a_json_object
		return ErrInvalidJSON
	case "22038": // singleton_sql_json_item_required
		return ErrInvalidJSON
	case "22039": // sql_json_array_not_found
		return ErrInvalidJSON
	case "2203A": // sql_json_member_not_found
		return ErrInvalidJSON
	case "2203B": // sql_json_number_not_found
		return ErrInvalidJSON
	case "2203C": // sql_json_object_not_found
		return ErrInvalidJSON
	case "2203D": // too_many_json_array_elements
		return ErrInvalidJSON
	case "2203E": // too_many_json_object_members
		return ErrInvalidJSON
	case "2203F": // sql_json_scalar_required
		return ErrInvalidJSON
	case "22P01": // floating_point_exception
		return ErrNumericOverflow
	case "22P02": // invalid_text_representation
		return ErrInvalidDataType
	case "22P03": // invalid_binary_representation
		return ErrInvalidDataType
	case "22P04": // bad_copy_file_format
		return ErrInvalidData
	case "22P05": // untranslatable_character
		return ErrInvalidDataType
	case "22P06": // nonstandard_use_of_escape_character
		return ErrInvalidData

	// Class 23 — Integrity Constraint Violation
	case "23000": // integrity_constraint_violation
		return ErrConstraintViolation
	case "23001": // restrict_violation
		return ErrForeignKey
	case "23502": // not_null_violation
		return ErrNotNullViolation
	case "23503": // foreign_key_violation
		return ErrForeignKey
	case "23505": // unique_violation
		return ErrDuplicateKey
	case "23514": // check_violation
		return ErrCheckConstraintViolation
	case "23P01": // exclusion_violation
		return ErrConstraintViolation

	// Class 24 — Invalid Cursor State
	case "24000": // invalid_cursor_state
		return ErrInvalidCursor

	// Class 25 — Invalid Transaction State
	case "25000": // invalid_transaction_state
		return ErrTransactionFailed
	case "25001": // active_sql_transaction
		return ErrTransactionFailed
	case "25002": // branch_transaction_already_active
		return ErrTransactionFailed
	case "25003": // inappropriate_access_mode_for_branch_transaction
		return ErrTransactionFailed
	case "25004": // inappropriate_isolation_level_for_branch_transaction
		return ErrTransactionFailed
	case "25005": // no_active_sql_transaction_for_branch_transaction
		return ErrTransactionFailed
	case "25006": // read_only_sql_transaction
		return ErrTransactionFailed
	case "25007": // schema_and_data_statement_mixing_not_supported
		return ErrTransactionFailed
	case "25008": // held_cursor_requires_same_isolation_level
		return ErrTransactionFailed
	case "25P01": // no_active_sql_transaction
		return ErrTransactionFailed
	case "25P02": // in_failed_sql_transaction
		return ErrTransactionFailed
	case "25P03": // idle_in_transaction_session_timeout
		return ErrIdleInTransaction

	// Class 26 — Invalid SQL Statement Name
	case "26000": // invalid_sql_statement_name
		return ErrInvalidQuery

	// Class 27 — Triggered Data Change Violation
	case "27000": // triggered_data_change_violation
		return ErrTriggerNotFound

	// Class 28 — Invalid Authorization Specification
	case "28000": // invalid_authorization_specification
		return ErrInvalidPassword
	case "28P01": // invalid_password
		return ErrInvalidPassword

	// Class 2B — Dependent Privilege Descriptors Still Exist
	case "2B000": // dependent_privilege_descriptors_still_exist
		return ErrConstraintViolation
	case "2BP01": // dependent_objects_still_exist
		return ErrConstraintViolation

	// Class 2D — Invalid Transaction Termination
	case "2D000": // invalid_transaction_termination
		return ErrTransactionFailed

	// Class 2F — SQL Routine Exception
	case "2F000": // sql_routine_exception
		return ErrFunctionNotFound
	case "2F002": // modifying_sql_data_not_permitted
		return ErrPermissionDenied
	case "2F003": // prohibited_sql_statement_attempted
		return ErrPermissionDenied
	case "2F004": // reading_sql_data_not_permitted
		return ErrPermissionDenied
	case "2F005": // function_executed_no_return_statement
		return ErrFunctionNotFound

	// Class 34 — Invalid Cursor Name
	case "34000": // invalid_cursor_name
		return ErrCursorNotFound

	// Class 38 — External Routine Exception
	case "38000": // external_routine_exception
		return ErrFunctionNotFound
	case "38001": // containing_sql_not_permitted
		return ErrPermissionDenied
	case "38002": // modifying_sql_data_not_permitted
		return ErrPermissionDenied
	case "38003": // prohibited_sql_statement_attempted
		return ErrPermissionDenied
	case "38004": // reading_sql_data_not_permitted
		return ErrPermissionDenied

	// Class 39 — External Routine Invocation Exception
	case "39000": // external_routine_invocation_exception
		return ErrFunctionNotFound
	case "39001": // invalid_sqlstate_returned
		return ErrFunctionNotFound
	case "39004": // null_value_not_allowed
		return ErrNotNullViolation
	case "39P01": // trigger_protocol_violated
		return ErrTriggerNotFound
	case "39P02": // srf_protocol_violated
		return ErrFunctionNotFound
	case "39P03": // event_trigger_protocol_violated
		return ErrTriggerNotFound

	// Class 3B — Savepoint Exception
	case "3B000": // savepoint_exception
		return ErrTransactionFailed
	case "3B001": // invalid_savepoint_specification
		return ErrTransactionFailed

	// Class 3D — Invalid Catalog Name
	case "3D000": // invalid_catalog_name
		return ErrDatabaseNotFound

	// Class 3F — Invalid Schema Name
	case "3F000": // invalid_schema_name
		return ErrSchemaNotFound

	// Class 40 — Transaction Rollback
	case "40000": // transaction_rollback
		return ErrTransactionFailed
	case "40001": // serialization_failure
		return ErrSerializationFailure
	case "40002": // transaction_integrity_constraint_violation
		return ErrConstraintViolation
	case "40003": // statement_completion_unknown
		return ErrTransactionFailed
	case "40P01": // deadlock_detected
		return ErrDeadlock

	// Class 42 — Syntax Error or Access Rule Violation
	case "42000": // syntax_error_or_access_rule_violation
		return ErrInvalidQuery
	case "42501": // insufficient_privilege
		return ErrInsufficientPrivileges
	case "42601": // syntax_error
		return ErrInvalidQuery
	case "42602": // invalid_name
		return ErrInvalidQuery
	case "42611": // invalid_column_definition
		return ErrInvalidQuery
	case "42622": // name_too_long
		return ErrDataTooLong
	case "42701": // duplicate_column
		return ErrDuplicateKey
	case "42702": // ambiguous_column
		return ErrColumnNotFound
	case "42703": // undefined_column
		return ErrColumnNotFound
	case "42704": // undefined_object
		return ErrTableNotFound
	case "42710": // duplicate_object
		return ErrDuplicateKey
	case "42712": // duplicate_alias
		return ErrDuplicateKey
	case "42723": // duplicate_function
		return ErrDuplicateKey
	case "42725": // ambiguous_function
		return ErrFunctionNotFound
	case "42803": // grouping_error
		return ErrInvalidQuery
	case "42804": // datatype_mismatch
		return ErrInvalidDataType
	case "42809": // wrong_object_type
		return ErrInvalidQuery
	case "42830": // invalid_foreign_key
		return ErrForeignKey
	case "42846": // cannot_coerce
		return ErrInvalidDataType
	case "42883": // undefined_function
		return ErrFunctionNotFound
	case "42939": // reserved_name
		return ErrInvalidQuery
	case "42P01": // undefined_table
		return ErrTableNotFound
	case "42P02": // undefined_parameter
		return ErrInvalidQuery
	case "42P03": // duplicate_cursor
		return ErrInvalidCursor
	case "42P04": // duplicate_database
		return ErrDuplicateKey
	case "42P05": // duplicate_prepared_statement
		return ErrDuplicateKey
	case "42P06": // duplicate_schema
		return ErrDuplicateKey
	case "42P07": // duplicate_table
		return ErrDuplicateKey
	case "42P08": // ambiguous_parameter
		return ErrInvalidQuery
	case "42P09": // ambiguous_alias
		return ErrInvalidQuery
	case "42P10": // invalid_column_reference
		return ErrColumnNotFound
	case "42P11": // invalid_cursor_definition
		return ErrInvalidCursor
	case "42P12": // invalid_database_definition
		return ErrConfigurationError
	case "42P13": // invalid_function_definition
		return ErrFunctionNotFound
	case "42P14": // invalid_prepared_statement_definition
		return ErrInvalidQuery
	case "42P15": // invalid_schema_definition
		return ErrSchemaValidation
	case "42P16": // invalid_table_definition
		return ErrSchemaValidation
	case "42P17": // invalid_object_definition
		return ErrSchemaValidation
	case "42P18": // indeterminate_datatype
		return ErrInvalidDataType
	case "42P19": // invalid_recursion
		return ErrInvalidQuery
	case "42P20": // windowing_error
		return ErrInvalidQuery
	case "42P21": // collation_mismatch
		return ErrInvalidDataType
	case "42P22": // indeterminate_collation
		return ErrInvalidDataType

	// Class 44 — WITH CHECK OPTION Violation
	case "44000": // with_check_option_violation
		return ErrCheckConstraintViolation

	// Class 53 — Insufficient Resources
	case "53000": // insufficient_resources
		return ErrSystemError
	case "53100": // disk_full
		return ErrDiskFull
	case "53200": // out_of_memory
		return ErrSystemError
	case "53300": // too_many_connections
		return ErrTooManyConnections
	case "53400": // configuration_limit_exceeded
		return ErrConfigurationError

	// Class 54 — Program Limit Exceeded
	case "54000": // program_limit_exceeded
		return ErrSystemError
	case "54001": // statement_too_complex
		return ErrInvalidQuery
	case "54011": // too_many_columns
		return ErrSchemaValidation
	case "54023": // too_many_arguments
		return ErrInvalidQuery

	// Class 55 — Object Not In Prerequisite State
	case "55000": // object_not_in_prerequisite_state
		return ErrSystemError
	case "55006": // object_in_use
		return ErrSystemError
	case "55P02": // cant_change_runtime_param
		return ErrConfigurationError
	case "55P03": // lock_not_available
		return ErrLockTimeout
	case "55P04": // unsafe_new_enum_value_usage
		return ErrUnsupportedOperation

	// Class 57 — Operator Intervention
	case "57000": // operator_intervention
		return ErrSystemError
	case "57014": // query_canceled
		return ErrQueryTimeout
	case "57P01": // admin_shutdown
		return ErrSystemError
	case "57P02": // crash_shutdown
		return ErrSystemError
	case "57P03": // cannot_connect_now
		return ErrConnectionFailed
	case "57P04": // database_dropped
		return ErrDatabaseNotFound
	case "57P05": // idle_session_timeout
		return ErrQueryTimeout

	// Class 58 — System Error
	case "58000": // system_error
		return ErrSystemError
	case "58030": // io_error
		return ErrSystemError
	case "58P01": // undefined_file
		return ErrSystemError
	case "58P02": // duplicate_file
		return ErrSystemError

	// Class F0 — Configuration File Error
	case "F0000": // config_file_error
		return ErrConfigurationError
	case "F0001": // lock_file_exists
		return ErrConfigurationError

	// Class HV — Foreign Data Wrapper Error
	case "HV000": // fdw_error
		return ErrSystemError
	case "HV005": // fdw_column_name_not_found
		return ErrColumnNotFound
	case "HV007": // fdw_dynamic_parameter_value_needed
		return ErrInvalidData
	case "HV008": // fdw_function_sequence_error
		return ErrFunctionNotFound
	case "HV009": // fdw_inconsistent_descriptor_information
		return ErrSystemError
	case "HV00A": // fdw_invalid_attribute_value
		return ErrInvalidData
	case "HV00B": // fdw_invalid_column_name
		return ErrColumnNotFound
	case "HV00C": // fdw_invalid_column_number
		return ErrColumnNotFound
	case "HV00D": // fdw_invalid_data_type
		return ErrInvalidDataType
	case "HV00J": // fdw_invalid_descriptor_field_identifier
		return ErrInvalidData
	case "HV00K": // fdw_invalid_handle
		return ErrSystemError
	case "HV00L": // fdw_invalid_option_index
		return ErrInvalidData
	case "HV00M": // fdw_invalid_option_name
		return ErrInvalidData
	case "HV00N": // fdw_invalid_string_length_or_buffer_length
		return ErrDataTooLong
	case "HV00P": // fdw_invalid_string_format
		return ErrInvalidData
	case "HV00Q": // fdw_invalid_use_of_null_pointer
		return ErrInvalidData
	case "HV00R": // fdw_too_many_handles
		return ErrSystemError
	case "HV010": // fdw_out_of_memory
		return ErrSystemError
	case "HV014": // fdw_unable_to_create_execution
		return ErrSystemError
	case "HV016": // fdw_unable_to_create_reply
		return ErrSystemError
	case "HV017": // fdw_unable_to_establish_connection
		return ErrConnectionFailed
	case "HV018": // fdw_no_schemas
		return ErrSchemaNotFound
	case "HV019": // fdw_option_name_not_found
		return ErrConfigurationError
	case "HV01A": // fdw_reply_handle
		return ErrSystemError
	case "HV01B": // fdw_schema_not_found
		return ErrSchemaNotFound
	case "HV01C": // fdw_table_not_found
		return ErrTableNotFound
	case "HV01D": // fdw_unable_to_create_execution_state
		return ErrSystemError

	// Class P0 — PL/pgSQL Error
	case "P0000": // plpgsql_error
		return ErrFunctionNotFound
	case "P0001": // raise_exception
		return ErrFunctionNotFound
	case "P0002": // no_data_found
		return ErrRecordNotFound
	case "P0003": // too_many_rows
		return ErrConstraintViolation
	case "P0004": // assert_failure
		return ErrCheckConstraintViolation

	// Class XX — Internal Error
	case "XX000": // internal_error
		return ErrInternalError
	case "XX001": // data_corrupted
		return ErrIndexCorruption
	case "XX002": // index_corrupted
		return ErrIndexCorruption

	default:
		// If we don't recognize the error code, return the original error
		return ErrSystemError
	}
}

// translateByErrorMessage translates errors based on error message patterns (fallback)
func (p *Postgres) translateByErrorMessage(errMsg string, originalErr error) error {
	switch {
	// Connection related
	case strings.Contains(errMsg, "connection refused"):
		return ErrConnectionFailed
	case strings.Contains(errMsg, "connection reset"):
		return ErrConnectionLost
	case strings.Contains(errMsg, "too many clients"):
		return ErrTooManyConnections
	case strings.Contains(errMsg, "connection timed out"):
		return ErrConnectionFailed
	case strings.Contains(errMsg, "server closed the connection"):
		return ErrConnectionLost

	// Timeout related
	case strings.Contains(errMsg, "statement timeout"):
		return ErrStatementTimeout
	case strings.Contains(errMsg, "canceling statement due to statement timeout"):
		return ErrStatementTimeout
	case strings.Contains(errMsg, "lock timeout"):
		return ErrLockTimeout
	case strings.Contains(errMsg, "timeout"):
		return ErrQueryTimeout
	case strings.Contains(errMsg, "idle in transaction"):
		return ErrIdleInTransaction

	// Lock related
	case strings.Contains(errMsg, "deadlock"):
		return ErrDeadlock
	case strings.Contains(errMsg, "could not obtain lock"):
		return ErrLockTimeout

	// Data related
	case strings.Contains(errMsg, "value too long"):
		return ErrDataTooLong
	case strings.Contains(errMsg, "invalid input syntax"):
		return ErrInvalidDataType
	case strings.Contains(errMsg, "invalid json"):
		return ErrInvalidJSON
	case strings.Contains(errMsg, "division by zero"):
		return ErrDivisionByZero
	case strings.Contains(errMsg, "numeric field overflow"):
		return ErrNumericOverflow

	// Constraint related - check these before general "violates" patterns
	case strings.Contains(errMsg, "violates check constraint"):
		return ErrCheckConstraintViolation
	case strings.Contains(errMsg, "duplicated key not allowed"):
		return ErrDuplicateKey
	case strings.Contains(errMsg, "duplicate key"):
		return ErrDuplicateKey
	case strings.Contains(errMsg, "violates not-null constraint"):
		return ErrNotNullViolation
	case strings.Contains(errMsg, "violates foreign key constraint"):
		return ErrForeignKey
	case strings.Contains(errMsg, "violates unique constraint"):
		return ErrDuplicateKey

	// Schema related
	case strings.Contains(errMsg, "relation") && strings.Contains(errMsg, "does not exist"):
		return ErrTableNotFound
	case strings.Contains(errMsg, "column") && strings.Contains(errMsg, "does not exist"):
		return ErrColumnNotFound
	case strings.Contains(errMsg, "function") && strings.Contains(errMsg, "does not exist"):
		return ErrFunctionNotFound
	case strings.Contains(errMsg, "database") && strings.Contains(errMsg, "does not exist"):
		return ErrDatabaseNotFound
	case strings.Contains(errMsg, "schema") && strings.Contains(errMsg, "does not exist"):
		return ErrSchemaNotFound
	case strings.Contains(errMsg, "sequence") && strings.Contains(errMsg, "does not exist"):
		return ErrSequenceNotFound
	case strings.Contains(errMsg, "index") && strings.Contains(errMsg, "does not exist"):
		return ErrIndexNotFound
	case strings.Contains(errMsg, "view") && strings.Contains(errMsg, "does not exist"):
		return ErrViewNotFound
	case strings.Contains(errMsg, "trigger") && strings.Contains(errMsg, "does not exist"):
		return ErrTriggerNotFound

	// Field/column related - GORM specific messages
	case strings.Contains(errMsg, "invalid field"):
		return ErrColumnNotFound

	// Permission related
	case strings.Contains(errMsg, "permission denied"):
		return ErrPermissionDenied
	case strings.Contains(errMsg, "insufficient privilege"):
		return ErrInsufficientPrivileges
	case strings.Contains(errMsg, "authentication failed"):
		return ErrInvalidPassword
	case strings.Contains(errMsg, "role") && strings.Contains(errMsg, "does not exist"):
		return ErrInvalidPassword

	// System related
	case strings.Contains(errMsg, "disk full"):
		return ErrDiskFull
	case strings.Contains(errMsg, "out of memory"):
		return ErrSystemError
	case strings.Contains(errMsg, "internal error"):
		return ErrInternalError
	case strings.Contains(errMsg, "index corruption"):
		return ErrIndexCorruption
	case strings.Contains(errMsg, "corrupted"):
		return ErrIndexCorruption

	// Cursor related
	case strings.Contains(errMsg, "cursor") && strings.Contains(errMsg, "does not exist"):
		return ErrCursorNotFound
	case strings.Contains(errMsg, "invalid cursor"):
		return ErrInvalidCursor

	// Migration/Backup related
	case strings.Contains(errMsg, "migration"):
		return ErrMigrationFailed
	case strings.Contains(errMsg, "backup"):
		return ErrBackupFailed
	case strings.Contains(errMsg, "restore"):
		return ErrRestoreFailed

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
func (p *Postgres) GetErrorCategory(err error) ErrorCategory {
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
func (p *Postgres) IsRetryable(err error) bool {
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
func (p *Postgres) IsTemporary(err error) bool {
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
func (p *Postgres) IsCritical(err error) bool {
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
