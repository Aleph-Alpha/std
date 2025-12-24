package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net"
	"os"
	"testing"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/go-connections/nat"
	"github.com/jackc/pgx/v5/pgconn"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"go.uber.org/fx"
	"go.uber.org/fx/fxtest"
	"go.uber.org/mock/gomock"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// TestUser is a sample model for testing GORM operations
type TestUser struct {
	gorm.Model
	Name  string
	Email string `gorm:"uniqueIndex"`
	Age   int
}

// PostgresContainer represents a Postgres container for testing
type PostgresContainer struct {
	testcontainers.Container
	ConnectionString string
	Config           Config
	Host             string
	Port             string
}

// setupPostgresContainer sets up a Postgres container for testing
func setupPostgresContainer(ctx context.Context) (*PostgresContainer, error) {
	// Get a random free port
	port, err := getFreePort()
	if err != nil {
		return nil, fmt.Errorf("could not get free port: %w", err)
	}

	portStr := fmt.Sprintf("%d", port)
	portBindings := nat.PortMap{
		"5432/tcp": []nat.PortBinding{{HostPort: portStr}},
	}

	// Define container request
	req := testcontainers.ContainerRequest{
		Image: "postgres:15",
		Env: map[string]string{
			"POSTGRES_USER":     "testuser",
			"POSTGRES_PASSWORD": "testpass",
			"POSTGRES_DB":       "testdb",
		},
		ExposedPorts: []string{"5432/tcp"},
		HostConfigModifier: func(cfg *container.HostConfig) {
			cfg.PortBindings = portBindings
		},
		WaitingFor: wait.ForLog("database system is ready to accept connections").WithStartupTimeout(30 * time.Second),
	}

	// Start container
	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to start postgres container: %w", err)
	}

	// Get host
	host, err := container.Host(ctx)
	if err != nil {
		_ = container.Terminate(ctx)
		return nil, fmt.Errorf("failed to get host: %w", err)
	}

	// Double-check port mapping (could be different from requested)
	mappedPort, err := container.MappedPort(ctx, "5432")
	if err != nil {
		_ = container.Terminate(ctx)
		return nil, fmt.Errorf("failed to get mapped port: %w", err)
	}

	// Update port to actual mapped port
	portStr = mappedPort.Port()

	// Wait for PostgreSQL to be fully ready for connections
	fmt.Printf("Waiting for PostgreSQL to be ready on %s:%s...\n", host, portStr)
	err = waitForPostgresReady(host, portStr, "testuser", "testpass", "testdb", 30*time.Second)
	if err != nil {
		_ = container.Terminate(ctx)
		return nil, fmt.Errorf("postgres container not ready: %w", err)
	}
	fmt.Printf("PostgreSQL is ready on %s:%s\n", host, portStr)

	// Create connection config
	config := Config{
		Connection: struct {
			Host     string
			Port     string
			User     string
			Password string
			DbName   string
			SSLMode  string
		}{
			Host:     host,
			Port:     portStr,
			User:     "testuser",
			Password: "testpass",
			DbName:   "testdb",
			SSLMode:  "disable",
		},
	}

	return &PostgresContainer{
		Container:        container,
		ConnectionString: fmt.Sprintf("host=%s port=%s user=testuser password=testpass dbname=testdb sslmode=disable", host, portStr),
		Config:           config,
		Host:             host,
		Port:             portStr,
	}, nil
}

// getFreePort gets a free port from the OS
func getFreePort() (int, error) {
	addr, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		return 0, err
	}
	defer func(addr net.Listener) {
		err := addr.Close()
		if err != nil {
			fmt.Printf("Failed to close listener: %v", err)
		}
	}(addr)

	return addr.Addr().(*net.TCPAddr).Port, nil
}

// TestMain sets up the testing environment
func TestMain(m *testing.M) {
	// Run tests
	code := m.Run()
	os.Exit(code)
}

// waitForPostgresReady attempts to connect to PostgresSQL until it's ready or times out
func waitForPostgresReady(host, port, user, password, dbname string, timeout time.Duration) error {
	// Use standard PostgresSQL connection string
	connStr := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		host, port, user, password, dbname)

	startTime := time.Now()
	for {
		if time.Since(startTime) > timeout {
			return fmt.Errorf("timed out waiting for PostgreSQL to be ready after %s", timeout)
		}

		// Try to establish a connection
		db, err := sql.Open("postgres", connStr)
		if err != nil {
			time.Sleep(500 * time.Millisecond)
			continue
		}

		// Try a simple ping
		err = db.Ping()
		if err == nil {
			// Close the connection and return success
			err = db.Close()
			if err != nil {
				return fmt.Errorf("error closing database connection: %w", err)
			}
			return nil
		}

		// Close the connection even if ping failed
		_ = db.Close()
		time.Sleep(500 * time.Millisecond)
	}
}

// TestPostgresWithFXModule tests the postgres package using the existing FX module
func TestPostgresWithFXModule(t *testing.T) {
	// Skip if running in short mode
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Setup PostgresSQL containerInstance
	ctx := context.Background()
	containerInstance, err := setupPostgresContainer(ctx)
	require.NoError(t, err)
	defer func() {
		if err := containerInstance.Terminate(ctx); err != nil {
			t.Fatalf("failed to terminate containerInstance: %s", err)
		}
	}()

	// Create mock controller and Logger
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Print connection details for debugging
	t.Logf("Using PostgreSQL on %s:%s", containerInstance.Host, containerInstance.Port)

	// Get the Postgres instance to test it
	var postgres *Postgres

	// Create a test app using the existing FXModule
	app := fxtest.New(t,
		// Provide dependencies with the correct containerInstance config
		fx.Provide(
			func() Config {
				return containerInstance.Config
			},
		),
		// Use the existing FXModule
		FXModule,
		fx.Populate(&postgres),
	)

	// Start the application
	err = app.Start(ctx)
	require.NoError(t, err)

	// Check if postgres was populated
	if postgres == nil || postgres.DB() == nil {
		t.Fatal("Failed to initialize Postgres client - connection likely failed")
	}

	// Verify the connection is working using the DB() method
	db := postgres.DB()
	require.NotNil(t, db)

	// Test DB connection with a simple query using DB()
	var result int
	err = db.Raw("SELECT 1").Scan(&result).Error
	assert.NoError(t, err)
	assert.Equal(t, 1, result)

	// Create the test table for our test models
	err = db.AutoMigrate(&TestUser{})
	require.NoError(t, err)

	// Test CRUD Operations using the public methods
	t.Run("CRUDOperations", func(t *testing.T) {
		ctx := context.Background()

		// Create
		user := TestUser{
			Name:  "John Doe",
			Email: "john@example.com",
			Age:   30,
		}

		err := postgres.Create(ctx, &user)
		assert.NoError(t, err)
		assert.Greater(t, user.ID, uint(0))

		// Find
		var users []TestUser
		err = postgres.Find(ctx, &users, "age = ?", 30)
		assert.NoError(t, err)
		assert.Len(t, users, 1)
		assert.Equal(t, "John Doe", users[0].Name)

		// First
		var retrievedUser TestUser
		err = postgres.First(ctx, &retrievedUser, "name = ?", "John Doe")
		assert.NoError(t, err)
		assert.Equal(t, "John Doe", retrievedUser.Name)
		assert.Equal(t, "john@example.com", retrievedUser.Email)
		assert.Equal(t, 30, retrievedUser.Age)

		// Update
		retrievedUser.Age = 31
		err = postgres.Save(ctx, &retrievedUser)
		assert.NoError(t, err)

		// Verify update with First
		var updatedUser TestUser
		err = postgres.First(ctx, &updatedUser, retrievedUser.ID)
		assert.NoError(t, err)
		assert.Equal(t, 31, updatedUser.Age)

		// UpdateWhere
		_, err = postgres.UpdateWhere(ctx, &TestUser{}, map[string]interface{}{
			"Age": 32,
		}, "name = ?", "John Doe")
		assert.NoError(t, err)

		// Verify UpdateWhere
		err = postgres.First(ctx, &updatedUser, "name = ?", "John Doe")
		assert.NoError(t, err)
		assert.Equal(t, 32, updatedUser.Age)

		// Update specific model
		_, err = postgres.Update(ctx, &updatedUser, map[string]interface{}{
			"Age": 33,
		})
		assert.NoError(t, err)

		// Verify Update
		err = postgres.First(ctx, &updatedUser, "name = ?", "John Doe")
		assert.NoError(t, err)
		assert.Equal(t, 33, updatedUser.Age)

		// UpdateColumn
		_, err = postgres.UpdateColumn(ctx, &updatedUser, "Age", 34)
		assert.NoError(t, err)

		// Verify UpdateColumn
		err = postgres.First(ctx, &updatedUser, "name = ?", "John Doe")
		assert.NoError(t, err)
		assert.Equal(t, 34, updatedUser.Age)

		// UpdateColumns
		_, err = postgres.UpdateColumns(ctx, &updatedUser, map[string]interface{}{
			"Age":   35,
			"Email": "john.updated@example.com",
		})
		assert.NoError(t, err)

		// Verify UpdateColumns
		err = postgres.First(ctx, &updatedUser, "name = ?", "John Doe")
		assert.NoError(t, err)
		assert.Equal(t, 35, updatedUser.Age)
		assert.Equal(t, "john.updated@example.com", updatedUser.Email)

		// Count
		var count int64
		err = postgres.Count(ctx, &TestUser{}, &count, "age > ?", 30)
		assert.NoError(t, err)
		assert.Equal(t, int64(1), count)

		// Delete
		_, err = postgres.Delete(ctx, &TestUser{}, "name = ?", "John Doe")
		assert.NoError(t, err)

		// Verify deletion with Count
		err = postgres.Count(ctx, &TestUser{}, &count, "name = ?", "John Doe")
		assert.NoError(t, err)
		assert.Equal(t, int64(0), count)
	})

	// Test Exec (raw SQL) method
	t.Run("ExecRawSQL", func(t *testing.T) {
		ctx := context.Background()

		// Create a test table using Exec
		_, err := postgres.Exec(ctx, `
			CREATE TABLE IF NOT EXISTS test_items (
				id SERIAL PRIMARY KEY,
				name TEXT NOT NULL,
				value INTEGER
			)
		`)
		assert.NoError(t, err)

		// Insert data using Exec
		_, err = postgres.Exec(ctx, `
			INSERT INTO test_items (name, value) VALUES ('item1', 100), ('item2', 200)
		`)
		assert.NoError(t, err)

		// Query data using the DB method since there's no direct query method exposed
		type Item struct {
			Name  string
			Value int
		}

		var items []Item
		err = postgres.DB().Raw(`SELECT name, value FROM test_items ORDER BY value`).Scan(&items).Error
		assert.NoError(t, err)
		assert.Len(t, items, 2)
		assert.Equal(t, "item1", items[0].Name)
		assert.Equal(t, 100, items[0].Value)
		assert.Equal(t, "item2", items[1].Name)
		assert.Equal(t, 200, items[1].Value)
	})

	// Test error handling - GORM errors and translation
	t.Run("ErrorHandling", func(t *testing.T) {
		ctx := context.Background()

		// Test 1: Direct GORM error checking (recommended pattern)
		t.Run("DirectGORMErrorChecking", func(t *testing.T) {
			var user TestUser
			err := postgres.First(ctx, &user, "name = ?", "NonExistentUser")
			assert.Error(t, err)
			assert.ErrorIs(t, err, gorm.ErrRecordNotFound, "Should return gorm.ErrRecordNotFound directly")

			// Same pattern works with QueryBuilder
			err = postgres.Query(ctx).Where("name = ?", "NonExistentUser").First(&user)
			assert.Error(t, err)
			assert.ErrorIs(t, err, gorm.ErrRecordNotFound, "QueryBuilder should also return gorm.ErrRecordNotFound")
		})

		// Test 2: Helper function pattern (recommended for consistency)
		t.Run("HelperFunctionPattern", func(t *testing.T) {
			// Define helper function (would typically be in your repository layer)
			isRecordNotFound := func(err error) bool {
				return errors.Is(err, gorm.ErrRecordNotFound)
			}

			var user TestUser
			err := postgres.First(ctx, &user, "name = ?", "NonExistentUser")
			assert.True(t, isRecordNotFound(err), "Helper function should identify record not found")

			// Works consistently with QueryBuilder too
			err = postgres.Query(ctx).Where("name = ?", "NonExistentUser").First(&user)
			assert.True(t, isRecordNotFound(err), "Helper function works with QueryBuilder")
		})

		// Test 3: Error translation (optional, when standardized errors are needed)
		t.Run("ErrorTranslation", func(t *testing.T) {
			var user TestUser
			err := postgres.First(ctx, &user, "name = ?", "NonExistentUser")
			translatedErr := postgres.TranslateError(err)
			assert.ErrorIs(t, translatedErr, ErrRecordNotFound, "TranslateError should convert to ErrRecordNotFound")

			// Translation works with any GORM error
			err = postgres.Query(ctx).Where("name = ?", "NonExistentUser").First(&user)
			translatedErr = postgres.TranslateError(err)
			assert.ErrorIs(t, translatedErr, ErrRecordNotFound, "TranslateError works with QueryBuilder errors")
		})

		// Test 4: Constraint violation errors
		t.Run("ConstraintViolations", func(t *testing.T) {
			// Create a table with a unique constraint
			_, err := postgres.Exec(ctx, `
				CREATE TABLE IF NOT EXISTS unique_test (
					id SERIAL PRIMARY KEY,
					email TEXT UNIQUE NOT NULL
				)
			`)
			assert.NoError(t, err)

			// Create a record
			_, err = postgres.Exec(ctx, `INSERT INTO unique_test (email) VALUES ('test@example.com')`)
			assert.NoError(t, err)

			// Try to create a duplicate (will fail due to unique constraint)
			_, err = postgres.Exec(ctx, `INSERT INTO unique_test (email) VALUES ('test@example.com')`)
			assert.Error(t, err)

			// Check with GORM error
			assert.ErrorIs(t, err, gorm.ErrDuplicatedKey, "Should return gorm.ErrDuplicatedKey for unique violation")

			// Or translate to standardized error
			translatedErr := postgres.TranslateError(err)
			assert.ErrorIs(t, translatedErr, ErrDuplicateKey, "Should translate to ErrDuplicateKey")
		})
	})

	// Stop the application
	require.NoError(t, app.Stop(ctx))
}

// TestPostgresConnectionFailureRecovery tests connection failure and recovery
func TestPostgresConnectionFailureRecovery(t *testing.T) {
	// Skip if running in short mode
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// This test is more complex and requires stopping and starting containers
	ctx := context.Background()
	containerInstance, err := setupPostgresContainer(ctx)
	require.NoError(t, err)
	defer func() {
		if err := containerInstance.Terminate(ctx); err != nil {
			t.Fatalf("failed to terminate containerInstance: %s", err)
		}
	}()

	// Print connection details for debugging
	t.Logf("Using PostgreSQL on %s:%s", containerInstance.Host, containerInstance.Port)

	// Create mock controller and Logger
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Get the Postgres instance to test it
	var postgres *Postgres

	// Create a test app using the existing FXModule
	app := fxtest.New(t,
		// Provide dependencies with the correct containerInstance config
		fx.Provide(
			func() Config {
				return containerInstance.Config
			},
		),
		// Use the existing FXModule
		FXModule,
		fx.Populate(&postgres),
	)

	// Start the application
	err = app.Start(ctx)
	require.NoError(t, err)

	// Verify connection works using the public DB() method
	db := postgres.DB()
	require.NotNil(t, db)

	var result int
	err = db.Raw("SELECT 1").Scan(&result).Error
	assert.NoError(t, err)

	// Simulate a connection error by sending a signal to the retry channel
	postgres.retryChanSignal <- fmt.Errorf("test connection error")

	// Give time for a reconnection attempt
	time.Sleep(100 * time.Millisecond)

	// Connection should still work since the containerInstance is running
	db = postgres.DB() // Get the DB again in case it was updated
	err = db.Raw("SELECT 1").Scan(&result).Error
	assert.NoError(t, err)

	err = app.Stop(ctx)
	require.NoError(t, err)
}

// TestErrorHandling tests the error handling and translation
func TestErrorHandling(t *testing.T) {
	ctx := context.Background()
	containerInstance, err := setupPostgresContainer(ctx)
	require.NoError(t, err)
	defer func() {
		if err := containerInstance.Terminate(ctx); err != nil {
			t.Fatalf("failed to terminate containerInstance: %s", err)
		}
	}()

	// Create mock controller and Logger
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Get the Postgres instance to test it
	var postgres *Postgres

	// Create a test app using the existing FXModule
	app := fxtest.New(t,
		// Provide dependencies with the correct containerInstance config
		fx.Provide(
			func() Config {
				return containerInstance.Config
			},
		),
		// Use the existing FXModule
		FXModule,
		fx.Populate(&postgres),
	)

	// Start the application
	err = app.Start(ctx)
	require.NoError(t, err)

	t.Run("TranslateError", func(t *testing.T) {
		// Test nil error
		assert.Equal(t, nil, postgres.TranslateError(nil))

		// Test GORM errors
		assert.Equal(t, ErrRecordNotFound, postgres.TranslateError(gorm.ErrRecordNotFound))
		assert.Equal(t, ErrDuplicateKey, postgres.TranslateError(gorm.ErrDuplicatedKey))
		assert.Equal(t, ErrForeignKey, postgres.TranslateError(gorm.ErrForeignKeyViolated))
		assert.Equal(t, ErrInvalidData, postgres.TranslateError(gorm.ErrInvalidData))
		assert.Equal(t, ErrTransactionFailed, postgres.TranslateError(gorm.ErrInvalidTransaction))
		assert.Equal(t, ErrUnsupportedOperation, postgres.TranslateError(gorm.ErrNotImplemented))
		assert.Equal(t, ErrInvalidQuery, postgres.TranslateError(gorm.ErrMissingWhereClause))
		assert.Equal(t, ErrUnsupportedOperation, postgres.TranslateError(gorm.ErrUnsupportedRelation))
		assert.Equal(t, ErrConstraintViolation, postgres.TranslateError(gorm.ErrPrimaryKeyRequired))
		assert.Equal(t, ErrInvalidData, postgres.TranslateError(gorm.ErrModelValueRequired))
		assert.Equal(t, ErrColumnNotFound, postgres.TranslateError(gorm.ErrInvalidField))
		assert.Equal(t, ErrInvalidData, postgres.TranslateError(gorm.ErrEmptySlice))
		assert.Equal(t, ErrUnsupportedOperation, postgres.TranslateError(gorm.ErrDryRunModeUnsupported))
		assert.Equal(t, ErrConfigurationError, postgres.TranslateError(gorm.ErrInvalidDB))
		assert.Equal(t, ErrInvalidData, postgres.TranslateError(gorm.ErrInvalidValue))
		assert.Equal(t, ErrDataTooLong, postgres.TranslateError(gorm.ErrInvalidValueOfLength))

		// Test error message patterns
		connectionRefusedErr := fmt.Errorf("connection refused")
		assert.Equal(t, ErrConnectionFailed, postgres.TranslateError(connectionRefusedErr))

		connectionResetErr := fmt.Errorf("connection reset by peer")
		assert.Equal(t, ErrConnectionLost, postgres.TranslateError(connectionResetErr))

		tooManyClientsErr := fmt.Errorf("too many clients already")
		assert.Equal(t, ErrTooManyConnections, postgres.TranslateError(tooManyClientsErr))

		timeoutErr := fmt.Errorf("operation timeout")
		assert.Equal(t, ErrQueryTimeout, postgres.TranslateError(timeoutErr))

		statementTimeoutErr := fmt.Errorf("canceling statement due to statement timeout")
		assert.Equal(t, ErrStatementTimeout, postgres.TranslateError(statementTimeoutErr))

		deadlockErr := fmt.Errorf("deadlock detected")
		assert.Equal(t, ErrDeadlock, postgres.TranslateError(deadlockErr))

		lockTimeoutErr := fmt.Errorf("lock timeout exceeded")
		assert.Equal(t, ErrLockTimeout, postgres.TranslateError(lockTimeoutErr))

		valueTooLongErr := fmt.Errorf("value too long for type character")
		assert.Equal(t, ErrDataTooLong, postgres.TranslateError(valueTooLongErr))

		invalidJSONErr := fmt.Errorf("invalid json format")
		assert.Equal(t, ErrInvalidJSON, postgres.TranslateError(invalidJSONErr))

		divisionByZeroErr := fmt.Errorf("division by zero")
		assert.Equal(t, ErrDivisionByZero, postgres.TranslateError(divisionByZeroErr))

		relationNotExistErr := fmt.Errorf("relation \"users\" does not exist")
		assert.Equal(t, ErrTableNotFound, postgres.TranslateError(relationNotExistErr))

		columnNotExistErr := fmt.Errorf("column \"invalid_column\" does not exist")
		assert.Equal(t, ErrColumnNotFound, postgres.TranslateError(columnNotExistErr))

		functionNotExistErr := fmt.Errorf("function my_function() does not exist")
		assert.Equal(t, ErrFunctionNotFound, postgres.TranslateError(functionNotExistErr))

		permissionDeniedErr := fmt.Errorf("permission denied for table users")
		assert.Equal(t, ErrPermissionDenied, postgres.TranslateError(permissionDeniedErr))

		insufficientPrivilegeErr := fmt.Errorf("insufficient privilege")
		assert.Equal(t, ErrInsufficientPrivileges, postgres.TranslateError(insufficientPrivilegeErr))

		diskFullErr := fmt.Errorf("disk full")
		assert.Equal(t, ErrDiskFull, postgres.TranslateError(diskFullErr))

		indexCorruptionErr := fmt.Errorf("index corruption detected")
		assert.Equal(t, ErrIndexCorruption, postgres.TranslateError(indexCorruptionErr))

		// Test custom error (should return unchanged)
		customErr := fmt.Errorf("custom error")
		assert.Equal(t, customErr, postgres.TranslateError(customErr))
	})

	t.Run("GetErrorCategory", func(t *testing.T) {
		// Connection errors
		assert.Equal(t, CategoryConnection, postgres.GetErrorCategory(ErrConnectionFailed))
		assert.Equal(t, CategoryConnection, postgres.GetErrorCategory(ErrConnectionLost))
		assert.Equal(t, CategoryConnection, postgres.GetErrorCategory(ErrTooManyConnections))

		// Query errors
		assert.Equal(t, CategoryQuery, postgres.GetErrorCategory(ErrInvalidQuery))
		assert.Equal(t, CategoryQuery, postgres.GetErrorCategory(ErrQueryTimeout))
		assert.Equal(t, CategoryQuery, postgres.GetErrorCategory(ErrStatementTimeout))
		assert.Equal(t, CategoryQuery, postgres.GetErrorCategory(ErrInvalidCursor))
		assert.Equal(t, CategoryQuery, postgres.GetErrorCategory(ErrCursorNotFound))

		// Data errors
		assert.Equal(t, CategoryData, postgres.GetErrorCategory(ErrInvalidData))
		assert.Equal(t, CategoryData, postgres.GetErrorCategory(ErrDataTooLong))
		assert.Equal(t, CategoryData, postgres.GetErrorCategory(ErrInvalidDataType))
		assert.Equal(t, CategoryData, postgres.GetErrorCategory(ErrInvalidJSON))
		assert.Equal(t, CategoryData, postgres.GetErrorCategory(ErrDivisionByZero))
		assert.Equal(t, CategoryData, postgres.GetErrorCategory(ErrNumericOverflow))

		// Constraint errors
		assert.Equal(t, CategoryConstraint, postgres.GetErrorCategory(ErrDuplicateKey))
		assert.Equal(t, CategoryConstraint, postgres.GetErrorCategory(ErrForeignKey))
		assert.Equal(t, CategoryConstraint, postgres.GetErrorCategory(ErrConstraintViolation))
		assert.Equal(t, CategoryConstraint, postgres.GetErrorCategory(ErrCheckConstraintViolation))
		assert.Equal(t, CategoryConstraint, postgres.GetErrorCategory(ErrNotNullViolation))

		// Permission errors
		assert.Equal(t, CategoryPermission, postgres.GetErrorCategory(ErrPermissionDenied))
		assert.Equal(t, CategoryPermission, postgres.GetErrorCategory(ErrInsufficientPrivileges))
		assert.Equal(t, CategoryPermission, postgres.GetErrorCategory(ErrInvalidPassword))
		assert.Equal(t, CategoryPermission, postgres.GetErrorCategory(ErrAccountLocked))

		// Transaction errors
		assert.Equal(t, CategoryTransaction, postgres.GetErrorCategory(ErrTransactionFailed))
		assert.Equal(t, CategoryTransaction, postgres.GetErrorCategory(ErrDeadlock))
		assert.Equal(t, CategoryTransaction, postgres.GetErrorCategory(ErrSerializationFailure))
		assert.Equal(t, CategoryTransaction, postgres.GetErrorCategory(ErrIdleInTransaction))

		// Resource errors
		assert.Equal(t, CategoryResource, postgres.GetErrorCategory(ErrDiskFull))
		assert.Equal(t, CategoryResource, postgres.GetErrorCategory(ErrLockTimeout))

		// System errors
		assert.Equal(t, CategorySystem, postgres.GetErrorCategory(ErrInternalError))
		assert.Equal(t, CategorySystem, postgres.GetErrorCategory(ErrSystemError))
		assert.Equal(t, CategorySystem, postgres.GetErrorCategory(ErrIndexCorruption))
		assert.Equal(t, CategorySystem, postgres.GetErrorCategory(ErrProtocolViolation))
		assert.Equal(t, CategorySystem, postgres.GetErrorCategory(ErrConfigurationError))

		// Schema errors
		assert.Equal(t, CategorySchema, postgres.GetErrorCategory(ErrTableNotFound))
		assert.Equal(t, CategorySchema, postgres.GetErrorCategory(ErrColumnNotFound))
		assert.Equal(t, CategorySchema, postgres.GetErrorCategory(ErrDatabaseNotFound))
		assert.Equal(t, CategorySchema, postgres.GetErrorCategory(ErrSchemaNotFound))
		assert.Equal(t, CategorySchema, postgres.GetErrorCategory(ErrFunctionNotFound))
		assert.Equal(t, CategorySchema, postgres.GetErrorCategory(ErrTriggerNotFound))
		assert.Equal(t, CategorySchema, postgres.GetErrorCategory(ErrIndexNotFound))
		assert.Equal(t, CategorySchema, postgres.GetErrorCategory(ErrViewNotFound))
		assert.Equal(t, CategorySchema, postgres.GetErrorCategory(ErrSequenceNotFound))

		// Operation errors
		assert.Equal(t, CategoryOperation, postgres.GetErrorCategory(ErrUnsupportedOperation))
		assert.Equal(t, CategoryOperation, postgres.GetErrorCategory(ErrMigrationFailed))
		assert.Equal(t, CategoryOperation, postgres.GetErrorCategory(ErrBackupFailed))
		assert.Equal(t, CategoryOperation, postgres.GetErrorCategory(ErrRestoreFailed))
		assert.Equal(t, CategoryOperation, postgres.GetErrorCategory(ErrSchemaValidation))

		// Unknown errors
		customErr := fmt.Errorf("some unknown error")
		assert.Equal(t, CategoryUnknown, postgres.GetErrorCategory(customErr))
	})

	t.Run("IsRetryable", func(t *testing.T) {
		// Retryable errors
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
			ErrInternalError,
		}

		for _, err := range retryableErrors {
			assert.True(t, postgres.IsRetryable(err), "Error %v should be retryable", err)
		}

		// Non-retryable errors
		nonRetryableErrors := []error{
			ErrPermissionDenied,
			ErrInvalidData,
			ErrDuplicateKey,
			ErrForeignKey,
			ErrTableNotFound,
			ErrColumnNotFound,
		}

		for _, err := range nonRetryableErrors {
			assert.False(t, postgres.IsRetryable(err), "Error %v should not be retryable", err)
		}
	})

	t.Run("IsTemporary", func(t *testing.T) {
		// Temporary errors
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

		for _, err := range temporaryErrors {
			assert.True(t, postgres.IsTemporary(err), "Error %v should be temporary", err)
		}

		// Non-temporary errors
		nonTemporaryErrors := []error{
			ErrPermissionDenied,
			ErrInvalidData,
			ErrDuplicateKey,
			ErrForeignKey,
			ErrTableNotFound,
			ErrColumnNotFound,
		}

		for _, err := range nonTemporaryErrors {
			assert.False(t, postgres.IsTemporary(err), "Error %v should not be temporary", err)
		}
	})

	t.Run("IsCritical", func(t *testing.T) {
		// Critical errors
		criticalErrors := []error{
			ErrIndexCorruption,
			ErrSystemError,
			ErrConfigurationError,
			ErrProtocolViolation,
			ErrDiskFull,
			ErrInternalError,
		}

		for _, err := range criticalErrors {
			assert.True(t, postgres.IsCritical(err), "Error %v should be critical", err)
		}

		// Non-critical errors
		nonCriticalErrors := []error{
			ErrPermissionDenied,
			ErrInvalidData,
			ErrDuplicateKey,
			ErrForeignKey,
			ErrTableNotFound,
			ErrColumnNotFound,
			ErrQueryTimeout,
		}

		for _, err := range nonCriticalErrors {
			assert.False(t, postgres.IsCritical(err), "Error %v should not be critical", err)
		}
	})

	t.Run("PostgreSQLErrorTranslation", func(t *testing.T) {
		// Create mock PostgreSQL errors using pgconn.PgError
		tests := []struct {
			name        string
			pgErrorCode string
			expectedErr error
		}{
			{"unique_violation", "23505", ErrDuplicateKey},
			{"foreign_key_violation", "23503", ErrForeignKey},
			{"not_null_violation", "23502", ErrNotNullViolation},
			{"check_violation", "23514", ErrCheckConstraintViolation},
			{"deadlock_detected", "40P01", ErrDeadlock},
			{"serialization_failure", "40001", ErrSerializationFailure},
			{"connection_exception", "08000", ErrConnectionFailed},
			{"connection_failure", "08006", ErrConnectionLost},
			{"undefined_table", "42P01", ErrTableNotFound},
			{"undefined_column", "42703", ErrColumnNotFound},
			{"undefined_function", "42883", ErrFunctionNotFound},
			{"insufficient_privilege", "42501", ErrInsufficientPrivileges},
			{"invalid_password", "28P01", ErrInvalidPassword},
			{"disk_full", "53100", ErrDiskFull},
			{"too_many_connections", "53300", ErrTooManyConnections},
			{"division_by_zero", "22012", ErrDivisionByZero},
			{"numeric_value_out_of_range", "22003", ErrNumericOverflow},
			{"string_data_right_truncation", "22001", ErrDataTooLong},
			{"invalid_json_text", "22032", ErrInvalidJSON},
			{"query_canceled", "57014", ErrQueryTimeout},
			{"statement_timeout", "57014", ErrQueryTimeout}, // Same code as query_canceled
			{"internal_error", "XX000", ErrInternalError},
			{"index_corrupted", "XX002", ErrIndexCorruption},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				// Create a mock pgconn.PgError
				pgErr := &pgconn.PgError{
					Code:    tt.pgErrorCode,
					Message: tt.name,
				}

				translatedErr := postgres.translatePostgreSQLError(pgErr)
				assert.Equal(t, tt.expectedErr, translatedErr,
					"Expected error %v for PostgreSQL code %s, got %v",
					tt.expectedErr, tt.pgErrorCode, translatedErr)
			})
		}
	})

	t.Run("RealDatabaseErrors", func(t *testing.T) {
		// Create test_users table first to test record not found vs table not found
		_, err := postgres.Exec(ctx, `
			CREATE TABLE IF NOT EXISTS test_users (
				id SERIAL PRIMARY KEY,
				name VARCHAR(100),
				email VARCHAR(100),
				deleted_at TIMESTAMP
			)
		`)
		require.NoError(t, err)

		// Test record not found (table exists but no matching record)
		var user TestUser
		err = postgres.First(ctx, &user, "id = ?", 99999)
		translatedErr := postgres.TranslateError(err)
		assert.Equal(t, ErrRecordNotFound, translatedErr)

		// Create a test table for constraint testing
		_, err = postgres.Exec(ctx, `
			CREATE TABLE IF NOT EXISTS test_constraints (
				id SERIAL PRIMARY KEY,
				email VARCHAR(100) UNIQUE NOT NULL,
				name VARCHAR(50) NOT NULL,
				age INTEGER CHECK (age >= 0)
			)
		`)
		require.NoError(t, err)

		// Test duplicate key violation
		_, err = postgres.Exec(ctx, "INSERT INTO test_constraints (email, name, age) VALUES (?, ?, ?)",
			"test@example.com", "Test User", 25)
		require.NoError(t, err)

		// Try to insert duplicate email
		_, err = postgres.Exec(ctx, "INSERT INTO test_constraints (email, name, age) VALUES (?, ?, ?)",
			"test@example.com", "Another User", 30)
		translatedErr = postgres.TranslateError(err)
		assert.Equal(t, ErrDuplicateKey, translatedErr)

		// Test not null violation
		_, err = postgres.Exec(ctx, "INSERT INTO test_constraints (email, age) VALUES (?, ?)",
			"test2@example.com", 25)
		translatedErr = postgres.TranslateError(err)
		assert.Equal(t, ErrNotNullViolation, translatedErr)

		// Test check constraint violation
		_, err = postgres.Exec(ctx, "INSERT INTO test_constraints (email, name, age) VALUES (?, ?, ?)",
			"test3@example.com", "Test User 3", -5)
		translatedErr = postgres.TranslateError(err)
		assert.Equal(t, ErrCheckConstraintViolation, translatedErr)

		// Test table not found
		_, err = postgres.Exec(ctx, "SELECT * FROM non_existent_table")
		translatedErr = postgres.TranslateError(err)
		assert.Equal(t, ErrTableNotFound, translatedErr)

		// Test column not found
		_, err = postgres.Exec(ctx, "SELECT non_existent_column FROM test_constraints")
		translatedErr = postgres.TranslateError(err)
		assert.Equal(t, ErrColumnNotFound, translatedErr)

		// Test invalid SQL syntax
		_, err = postgres.Exec(ctx, "INVALID SQL STATEMENT")
		translatedErr = postgres.TranslateError(err)
		assert.Equal(t, ErrInvalidQuery, translatedErr)

		// Clean up
		_, err = postgres.Exec(ctx, "DROP TABLE IF EXISTS test_constraints")
		require.NoError(t, err)
		_, err = postgres.Exec(ctx, "DROP TABLE IF EXISTS test_users")
		require.NoError(t, err)
	})

	require.NoError(t, app.Stop(ctx))
}

// Additional test helper functions for specific error scenarios
func TestErrorCategoriesComprehensive(t *testing.T) {
	ctx := context.Background()
	containerInstance, err := setupPostgresContainer(ctx)
	require.NoError(t, err)
	defer func() {
		if err := containerInstance.Terminate(ctx); err != nil {
			t.Fatalf("failed to terminate containerInstance: %s", err)
		}
	}()

	// Create mock controller and Logger
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Get the Postgres instance to test it
	var postgres *Postgres

	// Create a test app using the existing FXModule
	app := fxtest.New(t,
		// Provide dependencies with the correct containerInstance config
		fx.Provide(
			func() Config {
				return containerInstance.Config
			},
		),
		// Use the existing FXModule
		FXModule,
		fx.Populate(&postgres),
	)

	// Start the application
	err = app.Start(ctx)
	require.NoError(t, err)

	tests := []struct {
		name     string
		err      error
		category ErrorCategory
	}{
		// Connection category
		{"ConnectionFailed", ErrConnectionFailed, CategoryConnection},
		{"ConnectionLost", ErrConnectionLost, CategoryConnection},
		{"TooManyConnections", ErrTooManyConnections, CategoryConnection},

		// Query category
		{"InvalidQuery", ErrInvalidQuery, CategoryQuery},
		{"QueryTimeout", ErrQueryTimeout, CategoryQuery},
		{"StatementTimeout", ErrStatementTimeout, CategoryQuery},
		{"InvalidCursor", ErrInvalidCursor, CategoryQuery},
		{"CursorNotFound", ErrCursorNotFound, CategoryQuery},

		// Data category
		{"InvalidData", ErrInvalidData, CategoryData},
		{"DataTooLong", ErrDataTooLong, CategoryData},
		{"InvalidDataType", ErrInvalidDataType, CategoryData},
		{"InvalidJSON", ErrInvalidJSON, CategoryData},
		{"DivisionByZero", ErrDivisionByZero, CategoryData},
		{"NumericOverflow", ErrNumericOverflow, CategoryData},

		// Constraint category
		{"DuplicateKey", ErrDuplicateKey, CategoryConstraint},
		{"ForeignKey", ErrForeignKey, CategoryConstraint},
		{"ConstraintViolation", ErrConstraintViolation, CategoryConstraint},
		{"CheckConstraintViolation", ErrCheckConstraintViolation, CategoryConstraint},
		{"NotNullViolation", ErrNotNullViolation, CategoryConstraint},

		// Permission category
		{"PermissionDenied", ErrPermissionDenied, CategoryPermission},
		{"InsufficientPrivileges", ErrInsufficientPrivileges, CategoryPermission},
		{"InvalidPassword", ErrInvalidPassword, CategoryPermission},
		{"AccountLocked", ErrAccountLocked, CategoryPermission},

		// Transaction category
		{"TransactionFailed", ErrTransactionFailed, CategoryTransaction},
		{"Deadlock", ErrDeadlock, CategoryTransaction},
		{"SerializationFailure", ErrSerializationFailure, CategoryTransaction},
		{"IdleInTransaction", ErrIdleInTransaction, CategoryTransaction},

		// Resource category
		{"DiskFull", ErrDiskFull, CategoryResource},
		{"LockTimeout", ErrLockTimeout, CategoryResource},

		// System category
		{"InternalError", ErrInternalError, CategorySystem},
		{"SystemError", ErrSystemError, CategorySystem},
		{"IndexCorruption", ErrIndexCorruption, CategorySystem},
		{"ProtocolViolation", ErrProtocolViolation, CategorySystem},
		{"ConfigurationError", ErrConfigurationError, CategorySystem},

		// Schema category
		{"TableNotFound", ErrTableNotFound, CategorySchema},
		{"ColumnNotFound", ErrColumnNotFound, CategorySchema},
		{"DatabaseNotFound", ErrDatabaseNotFound, CategorySchema},
		{"SchemaNotFound", ErrSchemaNotFound, CategorySchema},
		{"FunctionNotFound", ErrFunctionNotFound, CategorySchema},
		{"TriggerNotFound", ErrTriggerNotFound, CategorySchema},
		{"IndexNotFound", ErrIndexNotFound, CategorySchema},
		{"ViewNotFound", ErrViewNotFound, CategorySchema},
		{"SequenceNotFound", ErrSequenceNotFound, CategorySchema},

		// Operation category
		{"UnsupportedOperation", ErrUnsupportedOperation, CategoryOperation},
		{"MigrationFailed", ErrMigrationFailed, CategoryOperation},
		{"BackupFailed", ErrBackupFailed, CategoryOperation},
		{"RestoreFailed", ErrRestoreFailed, CategoryOperation},
		{"SchemaValidation", ErrSchemaValidation, CategoryOperation},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			category := postgres.GetErrorCategory(tt.err)
			assert.Equal(t, tt.category, category,
				"Expected category %v for error %v, got %v",
				tt.category, tt.err, category)
		})
	}
}

func TestErrorClassificationFunctions(t *testing.T) {
	ctx := context.Background()
	containerInstance, err := setupPostgresContainer(ctx)
	require.NoError(t, err)
	defer func() {
		if err := containerInstance.Terminate(ctx); err != nil {
			t.Fatalf("failed to terminate containerInstance: %s", err)
		}
	}()

	// Create mock controller and Logger
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Get the Postgres instance to test it
	var postgres *Postgres

	// Create a test app using the existing FXModule
	app := fxtest.New(t,
		// Provide dependencies with the correct containerInstance config
		fx.Provide(
			func() Config {
				return containerInstance.Config
			},
		),
		// Use the existing FXModule
		FXModule,
		fx.Populate(&postgres),
	)

	// Start the application
	err = app.Start(ctx)
	require.NoError(t, err)

	t.Run("AllErrorsClassified", func(t *testing.T) {
		// Ensure all defined errors are properly classified
		allErrors := []error{
			ErrRecordNotFound, ErrDuplicateKey, ErrForeignKey, ErrInvalidData,
			ErrConnectionFailed, ErrTransactionFailed, ErrQueryTimeout, ErrInvalidQuery,
			ErrPermissionDenied, ErrTableNotFound, ErrColumnNotFound, ErrConstraintViolation,
			ErrCheckConstraintViolation, ErrNotNullViolation, ErrDataTooLong, ErrDeadlock,
			ErrLockTimeout, ErrInvalidDataType, ErrDivisionByZero, ErrNumericOverflow,
			ErrDiskFull, ErrTooManyConnections, ErrInvalidJSON, ErrIndexCorruption,
			ErrConfigurationError, ErrUnsupportedOperation, ErrMigrationFailed,
			ErrBackupFailed, ErrRestoreFailed, ErrSchemaValidation, ErrSerializationFailure,
			ErrInsufficientPrivileges, ErrInvalidPassword, ErrAccountLocked,
			ErrDatabaseNotFound, ErrSchemaNotFound, ErrFunctionNotFound, ErrTriggerNotFound,
			ErrIndexNotFound, ErrViewNotFound, ErrSequenceNotFound, ErrInvalidCursor,
			ErrCursorNotFound, ErrStatementTimeout, ErrIdleInTransaction, ErrConnectionLost,
			ErrProtocolViolation, ErrInternalError, ErrSystemError,
		}

		for _, err := range allErrors {
			category := postgres.GetErrorCategory(err)
			assert.NotEqual(t, CategoryUnknown, category,
				"Error %v should have a defined category", err)
		}
	})

	t.Run("RetryableErrorsValidation", func(t *testing.T) {
		// Validate that only appropriate errors are marked as retryable
		retryableErrors := []error{
			ErrConnectionFailed, ErrConnectionLost, ErrQueryTimeout, ErrStatementTimeout,
			ErrDeadlock, ErrLockTimeout, ErrSerializationFailure, ErrTooManyConnections,
			ErrIdleInTransaction, ErrProtocolViolation, ErrInternalError,
		}

		for _, err := range retryableErrors {
			assert.True(t, postgres.IsRetryable(err), "Error %v should be retryable", err)
		}

		// These should NOT be retryable
		nonRetryableErrors := []error{
			ErrPermissionDenied, ErrInvalidData, ErrDuplicateKey, ErrForeignKey,
			ErrTableNotFound, ErrColumnNotFound, ErrCheckConstraintViolation,
			ErrNotNullViolation, ErrInvalidPassword, ErrAccountLocked,
		}

		for _, err := range nonRetryableErrors {
			assert.False(t, postgres.IsRetryable(err), "Error %v should NOT be retryable", err)
		}
	})
}

// Test models for advanced query operations
type (
	Product struct {
		gorm.Model
		Name       string
		Price      float64
		CategoryID uint
		Category   Category    // Belongs to a relationship
		OrderItems []OrderItem // Has many relationships
	}

	Category struct {
		gorm.Model
		Name     string
		Products []Product // Has many relationships
	}

	Customer struct {
		gorm.Model
		Name    string
		Email   string
		Address string
		Orders  []Order // Has many relationships
	}

	Order struct {
		gorm.Model
		CustomerID  uint
		Customer    Customer // Belongs to a relationship
		OrderDate   time.Time
		OrderStatus string
		OrderItems  []OrderItem // Has many relationships
		TotalAmount float64
	}

	OrderItem struct {
		gorm.Model
		OrderID   uint
		Order     Order // Belongs to a relationship
		ProductID uint
		Product   Product // Belongs to a relationship
		Quantity  int
		Price     float64
	}
)

// TestAdvancedQueryOperations tests advanced SQL operations
func TestAdvancedQueryOperations(t *testing.T) {
	// Skip if running in short mode
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Setup PostgresSQL containerInstance
	ctx := context.Background()
	containerInstance, err := setupPostgresContainer(ctx)
	require.NoError(t, err)
	defer func() {
		if err := containerInstance.Terminate(ctx); err != nil {
			t.Fatalf("failed to terminate containerInstance: %s", err)
		}
	}()

	// Create mock controller and Logger
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Get the Postgres instance to test it
	var postgres *Postgres

	// Create a test app using the existing FXModule
	app := fxtest.New(t,
		// Provide dependencies with the correct containerInstance config
		fx.Provide(
			func() Config {
				return containerInstance.Config
			},
		),
		// Use the existing FXModule
		FXModule,
		fx.Populate(&postgres),
	)

	// Start the application
	err = app.Start(ctx)
	require.NoError(t, err)

	// Get the DB
	db := postgres.DB()
	require.NotNil(t, db)

	err = postgres.AutoMigrate(&Category{}, &Product{}, &Customer{}, &Order{}, &OrderItem{})
	require.NoError(t, err)

	// Seed test data
	err = seedTestData(ctx, postgres)
	require.NoError(t, err)

	// Run subtests
	t.Run("ComplexWhereClause", testComplexWhereClauses(ctx, postgres))
	t.Run("JoinsAndRelationships", testJoinsAndRelationships(ctx, postgres))
	t.Run("GroupByHavingClauses", testGroupByHavingClauses(ctx, postgres))
	t.Run("WindowFunctions", testWindowFunctions(ctx, postgres))
	t.Run("CTEExpressions", testCTEExpressions(ctx, postgres))

	err = app.Stop(ctx)
	require.NoError(t, err)
}

// seedTestData creates test data for all tests
func seedTestData(ctx context.Context, postgres *Postgres) error {
	//db := postgres.DB()

	// Create categories
	categories := []Category{
		{Name: "Electronics"},
		{Name: "Clothing"},
		{Name: "Books"},
		{Name: "Home & Kitchen"},
	}

	for i := range categories {
		if err := postgres.Create(ctx, &categories[i]); err != nil {
			return fmt.Errorf("failed to create category: %w", err)
		}
	}

	// Create products
	products := []Product{
		{Name: "Smartphone", Price: 799.99, CategoryID: categories[0].ID},
		{Name: "Laptop", Price: 1299.99, CategoryID: categories[0].ID},
		{Name: "Headphones", Price: 199.99, CategoryID: categories[0].ID},
		{Name: "T-Shirt", Price: 29.99, CategoryID: categories[1].ID},
		{Name: "Jeans", Price: 59.99, CategoryID: categories[1].ID},
		{Name: "Novel", Price: 15.99, CategoryID: categories[2].ID},
		{Name: "Cookbook", Price: 24.99, CategoryID: categories[2].ID},
		{Name: "Coffee Maker", Price: 89.99, CategoryID: categories[3].ID},
		{Name: "Toaster", Price: 49.99, CategoryID: categories[3].ID},
	}

	for i := range products {
		if err := postgres.Create(ctx, &products[i]); err != nil {
			return fmt.Errorf("failed to create product: %w", err)
		}
	}

	// Create customers
	customers := []Customer{
		{Name: "Alice Smith", Email: "alice@example.com", Address: "123 Main St"},
		{Name: "Bob Johnson", Email: "bob@example.com", Address: "456 Oak Ave"},
		{Name: "Carol Williams", Email: "carol@example.com", Address: "789 Pine Rd"},
	}

	for i := range customers {
		if err := postgres.Create(ctx, &customers[i]); err != nil {
			return fmt.Errorf("failed to create customer: %w", err)
		}
	}

	// Create orders
	now := time.Now()
	orders := []Order{
		{CustomerID: customers[0].ID, OrderDate: now.Add(-10 * 24 * time.Hour), OrderStatus: "Delivered", TotalAmount: 0},
		{CustomerID: customers[0].ID, OrderDate: now.Add(-5 * 24 * time.Hour), OrderStatus: "Shipped", TotalAmount: 0},
		{CustomerID: customers[1].ID, OrderDate: now.Add(-3 * 24 * time.Hour), OrderStatus: "Processing", TotalAmount: 0},
		{CustomerID: customers[1].ID, OrderDate: now.Add(-2 * 24 * time.Hour), OrderStatus: "Processing", TotalAmount: 0},
		{CustomerID: customers[2].ID, OrderDate: now.Add(-1 * 24 * time.Hour), OrderStatus: "Pending", TotalAmount: 0},
	}

	for i := range orders {
		if err := postgres.Create(ctx, &orders[i]); err != nil {
			return fmt.Errorf("failed to create order: %w", err)
		}
	}

	// Create order items
	orderItems := []OrderItem{
		{OrderID: orders[0].ID, ProductID: products[0].ID, Quantity: 1, Price: products[0].Price},
		{OrderID: orders[0].ID, ProductID: products[3].ID, Quantity: 2, Price: products[3].Price},
		{OrderID: orders[1].ID, ProductID: products[1].ID, Quantity: 1, Price: products[1].Price},
		{OrderID: orders[2].ID, ProductID: products[2].ID, Quantity: 1, Price: products[2].Price},
		{OrderID: orders[2].ID, ProductID: products[5].ID, Quantity: 3, Price: products[5].Price},
		{OrderID: orders[3].ID, ProductID: products[7].ID, Quantity: 1, Price: products[7].Price},
		{OrderID: orders[4].ID, ProductID: products[8].ID, Quantity: 1, Price: products[8].Price},
		{OrderID: orders[4].ID, ProductID: products[4].ID, Quantity: 1, Price: products[4].Price},
	}

	for i := range orderItems {
		if err := postgres.Create(ctx, &orderItems[i]); err != nil {
			return fmt.Errorf("failed to create order item: %w", err)
		}
	}

	// Update order total amounts based on order items
	for _, order := range orders {
		var items []OrderItem
		if err := postgres.Find(ctx, &items, "order_id = ?", order.ID); err != nil {
			return fmt.Errorf("failed to fetch order items: %w", err)
		}

		var total float64
		for _, item := range items {
			total += item.Price * float64(item.Quantity)
		}

		if _, err := postgres.Update(ctx, &order, map[string]interface{}{
			"total_amount": total,
		}); err != nil {
			return fmt.Errorf("failed to update order total: %w", err)
		}
	}

	return nil
}

// testComplexWhereClauses tests complex WHERE clauses
func testComplexWhereClauses(ctx context.Context, postgres *Postgres) func(t *testing.T) {
	return func(t *testing.T) {
		// Test 1: Complex WHERE with AND, OR, and comparison operators
		t.Run("ComplexConditions", func(t *testing.T) {
			var products []Product
			err := postgres.Query(ctx).
				Model(&Product{}).
				Where("price > ?", 50.0).
				Where("(category_id = ? OR category_id = ?)", 1, 2).
				Not("name LIKE ?", "%Coffee%").
				Order("price DESC").
				Find(&products)

			require.NoError(t, err)
			require.GreaterOrEqual(t, len(products), 1)

			// Verify all products in the result match our criteria
			for _, p := range products {
				assert.Greater(t, p.Price, 50.0)
				assert.True(t, p.CategoryID == 1 || p.CategoryID == 2)
				assert.NotContains(t, p.Name, "Coffee")
			}
		})

		// Test 2: Date range filtering
		t.Run("DateRangeFiltering", func(t *testing.T) {
			var orders []Order
			oneWeekAgo := time.Now().Add(-7 * 24 * time.Hour)

			err := postgres.Query(ctx).
				Model(&Order{}).
				Where("order_date > ?", oneWeekAgo).
				Where("order_status IN ?", []string{"Processing", "Pending"}).
				Order("order_date DESC").
				Find(&orders)

			require.NoError(t, err)
			require.GreaterOrEqual(t, len(orders), 1)

			// Verify all orders in the result match our criteria
			for _, o := range orders {
				assert.True(t, o.OrderDate.After(oneWeekAgo))
				assert.Contains(t, []string{"Processing", "Pending"}, o.OrderStatus)
			}
		})

		// Test 3: NULL handling with COALESCE
		t.Run("NullHandlingWithCoalesce", func(t *testing.T) {
			type ProductPrice struct {
				ID    uint
				Name  string
				Price float64
			}

			var productPrices []ProductPrice
			err := postgres.Query(ctx).
				Model(&Product{}).
				Select("id, name, COALESCE(price, 0) as price").
				Where("COALESCE(price, 0) > ?", 100).
				Order("price DESC").
				Scan(&productPrices)

			require.NoError(t, err)
			require.GreaterOrEqual(t, len(productPrices), 1)

			// Verify all products have price > 100
			for _, p := range productPrices {
				assert.Greater(t, p.Price, 100.0)
			}
		})

		// Test 4: Subquery in WHERE clause
		t.Run("SubqueryInWhere", func(t *testing.T) {
			var customers []Customer
			err := postgres.Query(ctx).
				Model(&Customer{}).
				Where("id IN (?)",
					postgres.DB().Model(&Order{}).
						Select("customer_id").
						Where("order_status = ?", "Delivered").
						Group("customer_id")).
				Find(&customers)

			require.NoError(t, err)
			require.GreaterOrEqual(t, len(customers), 1)
		})

		// Test 5: CASE expression in WHERE
		t.Run("CaseExpressionInWhere", func(t *testing.T) {
			type OrderWithPriority struct {
				ID       uint
				Priority string
			}

			var orders []OrderWithPriority
			err := postgres.Query(ctx).
				Model(&Order{}).
				Select(`id, 
					CASE 
						WHEN order_status = 'Pending' THEN 'High' 
						WHEN order_status = 'Processing' THEN 'Medium' 
						ELSE 'Low' 
					END as priority`).
				Where(`CASE 
					WHEN order_status = 'Pending' THEN 'High' 
					WHEN order_status = 'Processing' THEN 'Medium' 
					ELSE 'Low' 
				END = ?`, "High").
				Scan(&orders)

			require.NoError(t, err)

			// All orders should have High priority
			for _, o := range orders {
				assert.Equal(t, "High", o.Priority)
			}
		})
	}
}

// testJoinsAndRelationships tests various types of joins and relationships
func testJoinsAndRelationships(ctx context.Context, postgres *Postgres) func(t *testing.T) {
	return func(t *testing.T) {
		// Test 1: INNER JOIN
		t.Run("InnerJoin", func(t *testing.T) {
			type ProductWithCategory struct {
				ProductID    uint    `gorm:"column:product_id"`
				ProductName  string  `gorm:"column:product_name"`
				Price        float64 `gorm:"column:price"`
				CategoryID   uint    `gorm:"column:category_id"`
				CategoryName string  `gorm:"column:category_name"`
			}

			var results []ProductWithCategory
			err := postgres.Query(ctx).
				Model(&Product{}).
				Select("products.id as product_id, products.name as product_name, products.price, categories.id as category_id, categories.name as category_name").
				Joins("JOIN categories ON products.category_id = categories.id").
				Where("products.price > ?", 50).
				Scan(&results)

			require.NoError(t, err)
			require.GreaterOrEqual(t, len(results), 1)

			// Verify join worked correctly
			for _, r := range results {
				assert.NotEmpty(t, r.ProductName)
				assert.NotEmpty(t, r.CategoryName)
				assert.Greater(t, r.Price, 50.0)
			}
		})

		// Test 2: LEFT JOIN
		t.Run("LeftJoin", func(t *testing.T) {
			type OrderSummary struct {
				OrderID      uint      `gorm:"column:order_id"`
				OrderDate    time.Time `gorm:"column:order_date"`
				CustomerName string    `gorm:"column:customer_name"`
				TotalAmount  float64   `gorm:"column:total_amount"`
			}

			var results []OrderSummary
			err := postgres.Query(ctx).
				Model(&Order{}).
				Select("orders.id as order_id, orders.order_date, customers.name as customer_name, orders.total_amount").
				LeftJoin("customers ON orders.customer_id = customers.id").
				Order("orders.order_date DESC").
				Scan(&results)

			require.NoError(t, err)
			require.GreaterOrEqual(t, len(results), 1)

			// Verify join worked correctly
			for _, r := range results {
				assert.NotZero(t, r.OrderID)
				assert.NotEmpty(t, r.CustomerName)
			}
		})

		// Test 3: Multiple JOINs
		t.Run("MultipleJoins", func(t *testing.T) {
			type OrderItemDetail struct {
				OrderID        uint    `gorm:"column:order_id"`
				CustomerName   string  `gorm:"column:customer_name"`
				ProductName    string  `gorm:"column:product_name"`
				CategoryName   string  `gorm:"column:category_name"`
				Quantity       int     `gorm:"column:quantity"`
				ItemPrice      float64 `gorm:"column:item_price"`
				TotalItemPrice float64 `gorm:"column:total_item_price"`
			}

			var results []OrderItemDetail
			err := postgres.Query(ctx).
				Model(&OrderItem{}).
				Select(`
					orders.id as order_id, 
					customers.name as customer_name,
					products.name as product_name,
					categories.name as category_name,
					order_items.quantity,
					order_items.price as item_price,
					(order_items.quantity * order_items.price) as total_item_price
				`).
				Joins("JOIN orders ON order_items.order_id = orders.id").
				Joins("JOIN customers ON orders.customer_id = customers.id").
				Joins("JOIN products ON order_items.product_id = products.id").
				Joins("JOIN categories ON products.category_id = categories.id").
				Where("orders.order_status != ?", "Cancelled").
				Order("total_item_price DESC").
				Scan(&results)

			require.NoError(t, err)
			require.GreaterOrEqual(t, len(results), 1)

			// Verify all joins worked correctly
			for _, r := range results {
				assert.NotZero(t, r.OrderID)
				assert.NotEmpty(t, r.CustomerName)
				assert.NotEmpty(t, r.ProductName)
				assert.NotEmpty(t, r.CategoryName)
				assert.Equal(t, r.ItemPrice*float64(r.Quantity), r.TotalItemPrice)
			}
		})

		// Test 4: GORM Association Preloading
		t.Run("GormAssociationPreloading", func(t *testing.T) {
			var orders []Order
			err := postgres.Query(ctx).
				Model(&Order{}).
				Preload("Customer").
				Preload("OrderItems").
				Preload("OrderItems.Product").
				Preload("OrderItems.Product.Category").
				Limit(2).
				Find(&orders)

			require.NoError(t, err)
			require.GreaterOrEqual(t, len(orders), 1)

			// Verify preloading worked
			for _, order := range orders {
				assert.NotEmpty(t, order.Customer.Name)
				assert.GreaterOrEqual(t, len(order.OrderItems), 1)

				for _, item := range order.OrderItems {
					assert.NotEmpty(t, item.Product.Name)
					assert.NotEmpty(t, item.Product.Category.Name)
				}
			}
		})

		// Test 5: Self JOIN
		t.Run("SelfJoin", func(t *testing.T) {
			// First, create some sample data with self-references
			_, err := postgres.Exec(ctx, `
				CREATE TABLE IF NOT EXISTS employees (
					id SERIAL PRIMARY KEY,
					name TEXT NOT NULL,
					manager_id INTEGER REFERENCES employees(id)
				)
			`)
			require.NoError(t, err)

			// Insert sample data
			_, err = postgres.Exec(ctx, `
				INSERT INTO employees (name, manager_id) VALUES
				('CEO', NULL),
				('CTO', 1),
				('CFO', 1),
				('Developer 1', 2),
				('Developer 2', 2),
				('Accountant', 3)
			`)
			require.NoError(t, err)

			// Query with self-join
			type EmployeeWithManager struct {
				ID          int    `gorm:"column:id"`
				Name        string `gorm:"column:name"`
				ManagerID   *int   `gorm:"column:manager_id"`
				ManagerName string `gorm:"column:manager_name"`
			}

			var results []EmployeeWithManager
			err = postgres.Query(ctx).
				Raw(`
					SELECT e.id, e.name, e.manager_id, m.name as manager_name
					FROM employees e
					LEFT JOIN employees m ON e.manager_id = m.id
					ORDER BY e.id
				`).
				Scan(&results)

			require.NoError(t, err)
			require.Equal(t, 6, len(results))

			// Verify self-join worked
			assert.Equal(t, "CEO", results[0].Name)
			assert.Nil(t, results[0].ManagerID)
			assert.Empty(t, results[0].ManagerName)

			assert.Equal(t, "CTO", results[1].Name)
			assert.Equal(t, 1, *results[1].ManagerID)
			assert.Equal(t, "CEO", results[1].ManagerName)

			assert.Equal(t, "Developer 1", results[3].Name)
			assert.Equal(t, 2, *results[3].ManagerID)
			assert.Equal(t, "CTO", results[3].ManagerName)
		})
	}
}

// testGroupByHavingClauses tests GROUP BY and HAVING clauses
func testGroupByHavingClauses(ctx context.Context, postgres *Postgres) func(t *testing.T) {
	return func(t *testing.T) {
		// Test 1: Basic GROUP BY
		t.Run("BasicGroupBy", func(t *testing.T) {
			type CategoryCount struct {
				CategoryID   uint   `gorm:"column:category_id"`
				CategoryName string `gorm:"column:category_name"`
				ProductCount int    `gorm:"column:product_count"`
			}

			var results []CategoryCount
			err := postgres.Query(ctx).
				Model(&Product{}).
				Select("category_id, categories.name as category_name, COUNT(*) as product_count").
				Joins("JOIN categories ON products.category_id = categories.id").
				Group("category_id, categories.name").
				Order("product_count DESC").
				Scan(&results)

			require.NoError(t, err)
			require.GreaterOrEqual(t, len(results), 1)

			// Verify GROUP BY worked correctly
			for _, r := range results {
				assert.NotZero(t, r.CategoryID)
				assert.NotEmpty(t, r.CategoryName)
				assert.GreaterOrEqual(t, r.ProductCount, 1)
			}
		})

		// Test 2: GROUP BY with HAVING clause
		t.Run("GroupByWithHaving", func(t *testing.T) {
			type CategoryAvgPrice struct {
				CategoryID   uint    `gorm:"column:category_id"`
				CategoryName string  `gorm:"column:category_name"`
				AvgPrice     float64 `gorm:"column:avg_price"`
			}

			var results []CategoryAvgPrice
			err := postgres.Query(ctx).
				Model(&Product{}).
				Select("category_id, categories.name as category_name, AVG(price) as avg_price").
				Joins("JOIN categories ON products.category_id = categories.id").
				Group("category_id, categories.name").
				Having("AVG(price) > ?", 50).
				Order("avg_price DESC").
				Scan(&results)

			require.NoError(t, err)

			// Verify HAVING worked correctly
			for _, r := range results {
				assert.Greater(t, r.AvgPrice, 50.0)
			}
		})

		// Test 3: Multiple aggregations
		t.Run("MultipleAggregations", func(t *testing.T) {
			type CategoryStats struct {
				CategoryID   uint    `gorm:"column:category_id"`
				CategoryName string  `gorm:"column:category_name"`
				MinPrice     float64 `gorm:"column:min_price"`
				MaxPrice     float64 `gorm:"column:max_price"`
				AvgPrice     float64 `gorm:"column:avg_price"`
				TotalPrice   float64 `gorm:"column:total_price"`
				ProductCount int     `gorm:"column:product_count"`
			}

			var results []CategoryStats
			err := postgres.Query(ctx).
				Model(&Product{}).
				Select(`
					category_id, 
					categories.name as category_name,
					MIN(price) as min_price,
					MAX(price) as max_price,
					AVG(price) as avg_price,
					SUM(price) as total_price,
					COUNT(*) as product_count
				`).
				Joins("JOIN categories ON products.category_id = categories.id").
				Group("category_id, categories.name").
				Order("category_id").
				Scan(&results)

			require.NoError(t, err)
			require.GreaterOrEqual(t, len(results), 1)

			// Verify all aggregations worked correctly
			for _, r := range results {
				assert.LessOrEqual(t, r.MinPrice, r.MaxPrice)
				assert.GreaterOrEqual(t, r.AvgPrice, r.MinPrice)
				assert.LessOrEqual(t, r.AvgPrice, r.MaxPrice)
				assert.Greater(t, r.ProductCount, 0)

				// Check that the total price is approximately product_count * avg_price
				expectedTotal := float64(r.ProductCount) * r.AvgPrice
				assert.InDelta(t, expectedTotal, r.TotalPrice, expectedTotal*0.1) // Allow 10% tolerance
			}
		})

		// Test 4: GROUP BY with date parts
		t.Run("GroupByDateParts", func(t *testing.T) {
			type OrdersByDay struct {
				OrderDay    string  `gorm:"column:order_day"`
				OrderCount  int     `gorm:"column:order_count"`
				TotalAmount float64 `gorm:"column:total_amount"`
			}

			var results []OrdersByDay
			err := postgres.Query(ctx).
				Model(&Order{}).
				Select(`
					to_char(order_date, 'YYYY-MM-DD') as order_day,
					COUNT(*) as order_count,
					SUM(total_amount) as total_amount
				`).
				Group("order_day").
				Order("order_day").
				Scan(&results)

			require.NoError(t, err)
		})

		// Test 5: GROUP BY with FILTER
		t.Run("GroupByWithFilter", func(t *testing.T) {
			type CategoryStatusCounts struct {
				CategoryID       uint   `gorm:"column:category_id"`
				CategoryName     string `gorm:"column:category_name"`
				PendingOrders    int    `gorm:"column:pending_orders"`
				ProcessingOrders int    `gorm:"column:processing_orders"`
				CompletedOrders  int    `gorm:"column:completed_orders"`
			}

			// This requires PostgreSQL 9.4+ for FILTER clause
			var results []CategoryStatusCounts
			err := postgres.Query(ctx).
				Raw(`
					SELECT 
						p.category_id,
						c.name as category_name,
						COUNT(*) FILTER (WHERE o.order_status = 'Pending') as pending_orders,
						COUNT(*) FILTER (WHERE o.order_status = 'Processing') as processing_orders,
						COUNT(*) FILTER (WHERE o.order_status = 'Delivered') as completed_orders
					FROM order_items oi
					JOIN products p ON oi.product_id = p.id
					JOIN categories c ON p.category_id = c.id
					JOIN orders o ON oi.order_id = o.id
					GROUP BY p.category_id, c.name
					ORDER BY p.category_id
				`).
				Scan(&results)

			require.NoError(t, err)
		})
	}
}

// testWindowFunctions tests window functions
func testWindowFunctions(ctx context.Context, postgres *Postgres) func(t *testing.T) {
	return func(t *testing.T) {
		// Test 1: Basic window function with PARTITION BY
		t.Run("BasicWindowFunction", func(t *testing.T) {
			type ProductWithRank struct {
				ID           uint    `gorm:"column:id"`
				Name         string  `gorm:"column:name"`
				Price        float64 `gorm:"column:price"`
				CategoryID   uint    `gorm:"column:category_id"`
				CategoryName string  `gorm:"column:category_name"`
				Rank         int     `gorm:"column:rank"`
			}

			var results []ProductWithRank
			err := postgres.Query(ctx).
				Raw(`
					SELECT 
						p.id, p.name, p.price, p.category_id, c.name as category_name,
						RANK() OVER (PARTITION BY p.category_id ORDER BY p.price DESC) as rank
					FROM products p
					JOIN categories c ON p.category_id = c.id
					ORDER BY p.category_id, rank
				`).
				Scan(&results)

			require.NoError(t, err)
			require.GreaterOrEqual(t, len(results), 1)

			// Verify window function worked correctly
			currentCategory := uint(0)
			currentRank := 0

			for _, r := range results {
				if r.CategoryID != currentCategory {
					currentCategory = r.CategoryID
					currentRank = 0
				}

				currentRank++
				assert.Equal(t, currentRank, r.Rank)
			}
		})

		// Test 2: Multiple window functions
		t.Run("MultipleWindowFunctions", func(t *testing.T) {
			type OrderAnalysis struct {
				ID           uint      `gorm:"column:id"`
				OrderDate    time.Time `gorm:"column:order_date"`
				TotalAmount  float64   `gorm:"column:total_amount"`
				CustomerID   uint      `gorm:"column:customer_id"`
				RowNumber    int       `gorm:"column:row_number"`
				RunningTotal float64   `gorm:"column:running_total"`
			}

			var results []OrderAnalysis
			err := postgres.Query(ctx).
				Raw(`
					SELECT 
						o.id, o.order_date, o.total_amount, o.customer_id,
						ROW_NUMBER() OVER (PARTITION BY o.customer_id ORDER BY o.order_date) as row_number,
						SUM(o.total_amount) OVER (PARTITION BY o.customer_id ORDER BY o.order_date) as running_total
					FROM orders o
					ORDER BY o.customer_id, o.order_date
				`).
				Scan(&results)

			require.NoError(t, err)

			// Verify window functions worked correctly
			currentCustomer := uint(0)
			currentRowNumber := 0
			runningTotal := 0.0

			for _, r := range results {
				if r.CustomerID != currentCustomer {
					currentCustomer = r.CustomerID
					currentRowNumber = 0
					runningTotal = 0.0
				}

				currentRowNumber++
				runningTotal += r.TotalAmount

				assert.Equal(t, currentRowNumber, r.RowNumber)
				assert.InDelta(t, runningTotal, r.RunningTotal, 0.01)
			}
		})

		// Test 3: Window functions with framing
		t.Run("WindowFunctionsWithFraming", func(t *testing.T) {
			type ProductPriceAnalysis struct {
				ID         uint    `gorm:"column:id"`
				Name       string  `gorm:"column:name"`
				Price      float64 `gorm:"column:price"`
				CategoryID uint    `gorm:"column:category_id"`
				AvgPrice   float64 `gorm:"column:avg_price"`
				PriceRatio float64 `gorm:"column:price_ratio"`
				MovingAvg  float64 `gorm:"column:moving_avg"`
			}

			var results []ProductPriceAnalysis
			err := postgres.Query(ctx).
				Raw(`
					SELECT 
						p.id, p.name, p.price, p.category_id,
						AVG(p.price) OVER (PARTITION BY p.category_id) as avg_price,
						p.price / AVG(p.price) OVER (PARTITION BY p.category_id) as price_ratio,
						AVG(p.price) OVER (
							PARTITION BY p.category_id 
							ORDER BY p.price 
							ROWS BETWEEN 1 PRECEDING AND 1 FOLLOWING
						) as moving_avg
					FROM products p
					ORDER BY p.category_id, p.price
				`).
				Scan(&results)

			require.NoError(t, err)
			require.GreaterOrEqual(t, len(results), 1)

			// Verify window functions worked correctly
			for _, r := range results {
				// Check that price_ratio = price / avg_price
				assert.InDelta(t, r.Price/r.AvgPrice, r.PriceRatio, 0.01)
			}
		})

		// Test 4: NTILE window function
		t.Run("NtileWindowFunction", func(t *testing.T) {
			type ProductPriceTier struct {
				ID         uint    `gorm:"column:id"`
				Name       string  `gorm:"column:name"`
				Price      float64 `gorm:"column:price"`
				CategoryID uint    `gorm:"column:category_id"`
				PriceTier  int     `gorm:"column:price_tier"`
			}

			var results []ProductPriceTier
			err := postgres.Query(ctx).
				Raw(`
					SELECT 
						p.id, p.name, p.price, p.category_id,
						NTILE(3) OVER (PARTITION BY p.category_id ORDER BY p.price) as price_tier
					FROM products p
					ORDER BY p.category_id, p.price
				`).
				Scan(&results)

			require.NoError(t, err)
			require.GreaterOrEqual(t, len(results), 1)

			// Verify NTILE function worked correctly
			for _, r := range results {
				assert.GreaterOrEqual(t, r.PriceTier, 1)
				assert.LessOrEqual(t, r.PriceTier, 3)
			}
		})

		// Test 5: LEAD and LAG window functions
		t.Run("LeadLagWindowFunctions", func(t *testing.T) {
			type OrderWithNextPrev struct {
				ID                 uint      `gorm:"column:id"`
				CustomerID         uint      `gorm:"column:customer_id"`
				OrderDate          time.Time `gorm:"column:order_date"`
				TotalAmount        float64   `gorm:"column:total_amount"`
				NextAmount         *float64  `gorm:"column:next_amount"`
				PrevAmount         *float64  `gorm:"column:prev_amount"`
				DaysSinceLastOrder *int      `gorm:"column:days_since_last_order"`
			}

			var results []OrderWithNextPrev
			err := postgres.Query(ctx).
				Raw(`
					SELECT 
						o.id, o.customer_id, o.order_date, o.total_amount,
						LEAD(o.total_amount) OVER (PARTITION BY o.customer_id ORDER BY o.order_date) as next_amount,
						LAG(o.total_amount) OVER (PARTITION BY o.customer_id ORDER BY o.order_date) as prev_amount,
						EXTRACT(DAY FROM (o.order_date - LAG(o.order_date) OVER (PARTITION BY o.customer_id ORDER BY o.order_date))) as days_since_last_order
					FROM orders o
					ORDER BY o.customer_id, o.order_date
				`).
				Scan(&results)

			require.NoError(t, err)
			require.GreaterOrEqual(t, len(results), 1)

			// Verify LEAD and LAG functions
			currentCustomerID := uint(0)
			var prevResult *OrderWithNextPrev

			for i, r := range results {
				if r.CustomerID != currentCustomerID {
					currentCustomerID = r.CustomerID
					prevResult = nil
				}

				if prevResult != nil {
					// The current order's prev_amount should equal the previous order's total_amount
					if r.PrevAmount != nil {
						assert.InDelta(t, prevResult.TotalAmount, *r.PrevAmount, 0.01)
					}

					// The previous order's next_amount should equal the current order's total_amount
					if prevResult.NextAmount != nil {
						assert.InDelta(t, r.TotalAmount, *prevResult.NextAmount, 0.01)
					}

					// Check days_since_last_order calculation
					if r.DaysSinceLastOrder != nil {
						daysDiff := int(r.OrderDate.Sub(prevResult.OrderDate).Hours() / 24)
						assert.InDelta(t, daysDiff, *r.DaysSinceLastOrder, 1) // Allow a 1-day difference due to time part
					}
				}

				prevResult = &results[i]
			}
		})
	}
}

// testCTEExpressions tests Common Table Expressions
func testCTEExpressions(ctx context.Context, postgres *Postgres) func(t *testing.T) {
	return func(t *testing.T) {
		// Test 1: Basic CTE
		t.Run("BasicCTE", func(t *testing.T) {
			type ProductSummary struct {
				CategoryID   uint    `gorm:"column:category_id"`
				CategoryName string  `gorm:"column:category_name"`
				AvgPrice     float64 `gorm:"column:avg_price"`
				MinPrice     float64 `gorm:"column:min_price"`
				MaxPrice     float64 `gorm:"column:max_price"`
			}

			var results []ProductSummary
			err := postgres.Query(ctx).
				Raw(`
					WITH category_stats AS (
						SELECT 
							p.category_id,
							c.name as category_name,
							AVG(p.price) as avg_price,
							MIN(p.price) as min_price,
							MAX(p.price) as max_price
						FROM products p
						JOIN categories c ON p.category_id = c.id
						GROUP BY p.category_id, c.name
					)
					SELECT * FROM category_stats
					ORDER BY category_id
				`).
				Scan(&results)

			require.NoError(t, err)
			require.GreaterOrEqual(t, len(results), 1)

			// Verify CTE results
			for _, r := range results {
				assert.NotZero(t, r.CategoryID)
				assert.NotEmpty(t, r.CategoryName)
				assert.LessOrEqual(t, r.MinPrice, r.AvgPrice)
				assert.LessOrEqual(t, r.AvgPrice, r.MaxPrice)
			}
		})

		// Test 2: Multiple CTEs
		t.Run("MultipleCTEs", func(t *testing.T) {
			type CustomerOrderStats struct {
				CustomerID    uint    `gorm:"column:customer_id"`
				CustomerName  string  `gorm:"column:customer_name"`
				OrderCount    int     `gorm:"column:order_count"`
				TotalSpent    float64 `gorm:"column:total_spent"`
				AvgOrderValue float64 `gorm:"column:avg_order_value"`
				CategoryName  string  `gorm:"column:most_ordered_category"`
			}

			var results []CustomerOrderStats
			err := postgres.Query(ctx).
				Raw(`
					WITH 
					order_summary AS (
						SELECT 
							o.customer_id,
							COUNT(*) as order_count,
							SUM(o.total_amount) as total_spent,
							AVG(o.total_amount) as avg_order_value
						FROM orders o
						GROUP BY o.customer_id
					),
					category_counts AS (
						SELECT 
							o.customer_id,
							p.category_id,
							c.name as category_name,
							COUNT(*) as category_order_count,
							ROW_NUMBER() OVER (PARTITION BY o.customer_id ORDER BY COUNT(*) DESC) as rank
						FROM orders o
						JOIN order_items oi ON o.id = oi.order_id
						JOIN products p ON oi.product_id = p.id
						JOIN categories c ON p.category_id = c.id
						GROUP BY o.customer_id, p.category_id, c.name
					)
					SELECT 
						os.customer_id,
						cu.name as customer_name,
						os.order_count,
						os.total_spent,
						os.avg_order_value,
						cc.category_name as most_ordered_category
					FROM order_summary os
					JOIN customers cu ON os.customer_id = cu.id
					LEFT JOIN category_counts cc ON os.customer_id = cc.customer_id AND cc.rank = 1
					ORDER BY os.total_spent DESC
				`).
				Scan(&results)

			require.NoError(t, err)
			require.GreaterOrEqual(t, len(results), 1)

			// Verify multiple CTE results
			for _, r := range results {
				assert.NotZero(t, r.CustomerID)
				assert.NotEmpty(t, r.CustomerName)
				assert.GreaterOrEqual(t, r.OrderCount, 1)
				assert.GreaterOrEqual(t, r.TotalSpent, 0.0)

				// Avg order value should be totally spent divided by order count
				assert.InDelta(t, r.TotalSpent/float64(r.OrderCount), r.AvgOrderValue, 0.01)
			}
		})

		// Test 3: Recursive CTE
		t.Run("RecursiveCTE", func(t *testing.T) {
			// First, we'll create a hierarchical structure to test with
			_, err := postgres.Exec(ctx, `
				CREATE TABLE IF NOT EXISTS categories_tree (
					id SERIAL PRIMARY KEY,
					name TEXT NOT NULL,
					parent_id INTEGER REFERENCES categories_tree(id)
				)
			`)
			require.NoError(t, err)

			// Insert sample hierarchical data
			_, err = postgres.Exec(ctx, `
				INSERT INTO categories_tree (id, name, parent_id) VALUES
				(1, 'Electronics', NULL),
				(2, 'Computers', 1),
				(3, 'Phones', 1),
				(4, 'Laptops', 2),
				(5, 'Desktops', 2),
				(6, 'Gaming Laptops', 4),
				(7, 'Business Laptops', 4),
				(8, 'Smartphones', 3),
				(9, 'Accessories', NULL),
				(10, 'Phone Cases', 9)
			`)
			require.NoError(t, err)

			// Test recursive CTE to get category paths
			type CategoryPath struct {
				ID    int    `gorm:"column:id"`
				Name  string `gorm:"column:name"`
				Path  string `gorm:"column:path"`
				Level int    `gorm:"column:level"`
			}

			var results []CategoryPath
			err = postgres.Query(ctx).
				Raw(`
					WITH RECURSIVE category_paths AS (
            -- Base case: top-level categories
            SELECT 
                id, 
                name,
                name as path,
                0 as level
            FROM categories_tree
            WHERE parent_id IS NULL
            
            UNION ALL
            
            -- Recursive case: child categories
            SELECT 
                ct.id,
                ct.name,
                cp.path || ' > ' || ct.name as path,
                cp.level + 1 as level
            FROM categories_tree ct
            JOIN category_paths cp ON ct.parent_id = cp.id
        )
        SELECT * FROM category_paths
        ORDER BY id  -- Changed from ORDER BY path to ORDER BY id
				`).
				Scan(&results)

			require.NoError(t, err)
			require.GreaterOrEqual(t, len(results), 1)

			// Verify recursive CTE results
			assert.Equal(t, "Electronics", results[0].Name)
			assert.Equal(t, 0, results[0].Level)

			// Check a deep-nested path
			var gamingLaptopsResult *CategoryPath
			for i, r := range results {
				if r.Name == "Gaming Laptops" {
					gamingLaptopsResult = &results[i]
					break
				}
			}

			require.NotNil(t, gamingLaptopsResult)
			assert.Equal(t, "Electronics > Computers > Laptops > Gaming Laptops", gamingLaptopsResult.Path)
			assert.Equal(t, 3, gamingLaptopsResult.Level)
		})

		// Test 4: CTE with UPDATE
		t.Run("CTEWithUpdate", func(t *testing.T) {
			// First, check current prices
			var beforeProducts []Product
			err := postgres.Query(ctx).
				Model(&Product{}).
				Where("category_id = ?", 1). // Electronics
				Find(&beforeProducts)
			require.NoError(t, err)
			require.Greater(t, len(beforeProducts), 0)

			// Use CTE to update prices with a 10% discount for electronics products
			_, err = postgres.Exec(ctx, `
				WITH electronics_products AS (
        SELECT id, price 
        FROM products
        WHERE category_id = 1
    )
    UPDATE products
    SET price = products.price * 0.9  -- Qualified the price reference
    FROM electronics_products
    WHERE products.id = electronics_products.id
			`)
			require.NoError(t, err)

			// Check updated prices
			var afterProducts []Product
			err = postgres.Query(ctx).
				Model(&Product{}).
				Where("category_id = ?", 1).
				Find(&afterProducts)
			require.NoError(t, err)
			require.Equal(t, len(beforeProducts), len(afterProducts))

			// Verify prices were discounted by 10%
			for i := range beforeProducts {
				expectedPrice := beforeProducts[i].Price * 0.9
				assert.InDelta(t, expectedPrice, afterProducts[i].Price, 0.01)
			}
		})

		// Test 5: CTE to generate sequential data
		t.Run("CTEToGenerateData", func(t *testing.T) {
			type DateSeries struct {
				Date time.Time `gorm:"column:date"`
			}

			var results []DateSeries
			err := postgres.Query(ctx).
				Raw(`
					WITH RECURSIVE date_series AS (
						SELECT CURRENT_DATE - INTERVAL '6 days' as date
						UNION ALL
						SELECT date + INTERVAL '1 day'
						FROM date_series
						WHERE date < CURRENT_DATE
					)
					SELECT * FROM date_series
				`).
				Scan(&results)

			require.NoError(t, err)
			require.Equal(t, 7, len(results)) // Should get 7 days

			// Verify dates are sequential
			for i := 1; i < len(results); i++ {
				nextExpectedDate := results[i-1].Date.AddDate(0, 0, 1)
				assert.Equal(t, nextExpectedDate.Format("2006-01-02"), results[i].Date.Format("2006-01-02"))
			}
		})
	}
}

// Models for transaction testing
type (
	Account struct {
		gorm.Model
		Name    string
		Balance float64
	}

	TransactionLog struct {
		gorm.Model
		FromAccountID uint
		ToAccountID   uint
		Amount        float64
		Description   string
		Status        string // "pending", "completed", "failed"
	}
)

// TestTransactionHandling tests various aspects of transaction management
func TestTransactionHandling(t *testing.T) {
	// Skip if running in short mode
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Setup PostgresSQL containerInstance
	ctx := context.Background()
	containerInstance, err := setupPostgresContainer(ctx)
	require.NoError(t, err)
	defer func() {
		if err := containerInstance.Terminate(ctx); err != nil {
			t.Fatalf("failed to terminate containerInstance: %s", err)
		}
	}()

	// Create mock controller and Logger
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	var postgres *Postgres

	// Create a test app using the existing FXModule
	app := fxtest.New(t,
		// Provide dependencies with the correct containerInstance config
		fx.Provide(
			func() Config {
				return containerInstance.Config
			},
		),
		// Use the existing FXModule
		FXModule,
		fx.Populate(&postgres),
	)

	// Start the application
	err = app.Start(ctx)
	require.NoError(t, err)

	// Get the DB and auto-migrate schemas
	db := postgres.DB()
	require.NotNil(t, db)
	err = db.AutoMigrate(&Account{}, &TransactionLog{})
	require.NoError(t, err)

	// Setup test accounts
	setupTestAccounts(t, ctx, postgres)

	// Run subtests
	t.Run("BasicTransactionCommit", testBasicTransactionCommit(ctx, postgres))
	t.Run("TransactionRollback", testTransactionRollback(ctx, postgres))

	err = app.Stop(ctx)
	require.NoError(t, err)
}

// setupTestAccounts creates test accounts for transaction tests
func setupTestAccounts(t *testing.T, ctx context.Context, postgres *Postgres) {
	// Create initial accounts
	accounts := []Account{
		{Name: "Alice", Balance: 1000.00},
		{Name: "Bob", Balance: 500.00},
		{Name: "Charlie", Balance: 1500.00},
		{Name: "Dave", Balance: 2000.00},
	}

	for _, account := range accounts {
		err := postgres.Create(ctx, &account)
		require.NoError(t, err)
		require.NotZero(t, account.ID)
	}
}

// testBasicTransactionCommit tests a simple transaction that commits successfully
func testBasicTransactionCommit(ctx context.Context, postgres *Postgres) func(t *testing.T) {
	return func(t *testing.T) {
		// Get initial balances
		var aliceAccount, bobAccount Account
		err := postgres.First(ctx, &aliceAccount, "name = ?", "Alice")
		require.NoError(t, err)
		err = postgres.First(ctx, &bobAccount, "name = ?", "Bob")
		require.NoError(t, err)

		initialAliceBalance := aliceAccount.Balance
		initialBobBalance := bobAccount.Balance
		transferAmount := 100.00

		// Perform transfer in a transaction
		err = postgres.Transaction(ctx, func(tx Client) error {
			// Update Alice's account (subtract)
			aliceAccount.Balance -= transferAmount
			err := tx.Save(ctx, &aliceAccount)
			if err != nil {
				return err
			}

			// Update Bob's account (add)
			bobAccount.Balance += transferAmount
			err = tx.Save(ctx, &bobAccount)
			if err != nil {
				return err
			}

			// Create a transaction log
			txLog := TransactionLog{
				FromAccountID: aliceAccount.ID,
				ToAccountID:   bobAccount.ID,
				Amount:        transferAmount,
				Description:   "Basic transfer test",
				Status:        "completed",
			}
			return tx.Create(ctx, &txLog)
		})
		require.NoError(t, err)

		// Verify final balances
		err = postgres.First(ctx, &aliceAccount, aliceAccount.ID)
		require.NoError(t, err)
		err = postgres.First(ctx, &bobAccount, bobAccount.ID)
		require.NoError(t, err)

		assert.Equal(t, initialAliceBalance-transferAmount, aliceAccount.Balance)
		assert.Equal(t, initialBobBalance+transferAmount, bobAccount.Balance)

		// Verify transaction log was created
		var txLog TransactionLog
		err = postgres.First(ctx, &txLog, "from_account_id = ? AND to_account_id = ?", aliceAccount.ID, bobAccount.ID)
		require.NoError(t, err)
		assert.Equal(t, transferAmount, txLog.Amount)
		assert.Equal(t, "completed", txLog.Status)
	}
}

// testTransactionRollback tests a transaction that rolls back due to an error
func testTransactionRollback(ctx context.Context, postgres *Postgres) func(t *testing.T) {
	return func(t *testing.T) {
		// Get initial balances
		var charlieAccount, daveAccount Account
		err := postgres.First(ctx, &charlieAccount, "name = ?", "Charlie")
		require.NoError(t, err)
		err = postgres.First(ctx, &daveAccount, "name = ?", "Dave")
		require.NoError(t, err)

		initialCharlieBalance := charlieAccount.Balance
		initialDaveBalance := daveAccount.Balance
		transferAmount := 200.00

		// Perform transfer in a transaction, but return an error to cause rollback
		err = postgres.Transaction(ctx, func(tx Client) error {
			// Update Charlie's account (subtract)
			charlieAccount.Balance -= transferAmount
			err := tx.Save(ctx, &charlieAccount)
			if err != nil {
				return err
			}

			// Update Dave's account (add)
			daveAccount.Balance += transferAmount
			err = tx.Save(ctx, &daveAccount)
			if err != nil {
				return err
			}

			// Create a transaction log
			txLog := TransactionLog{
				FromAccountID: charlieAccount.ID,
				ToAccountID:   daveAccount.ID,
				Amount:        transferAmount,
				Description:   "Transfer with rollback",
				Status:        "pending",
			}
			err = tx.Create(ctx, &txLog)
			if err != nil {
				return err
			}

			// Return an error to trigger rollback
			return fmt.Errorf("simulated error to trigger rollback")
		})
		// Ensure the transaction returned our error
		require.Error(t, err)
		assert.Contains(t, err.Error(), "simulated error to trigger rollback")

		// Verify balances are unchanged (rollback worked)
		err = postgres.First(ctx, &charlieAccount, charlieAccount.ID)
		require.NoError(t, err)
		err = postgres.First(ctx, &daveAccount, daveAccount.ID)
		require.NoError(t, err)

		assert.Equal(t, initialCharlieBalance, charlieAccount.Balance)
		assert.Equal(t, initialDaveBalance, daveAccount.Balance)

		// Verify no transaction log was created
		var count int64
		err = postgres.Count(ctx, &TransactionLog{}, &count, "from_account_id = ? AND to_account_id = ?", charlieAccount.ID, daveAccount.ID)
		require.NoError(t, err)
		assert.Equal(t, int64(0), count)
	}
}

// TestQueryBuilder_Create tests the Create() method on QueryBuilder
func TestQueryBuilder_Create(t *testing.T) {
	// Skip if running in short mode
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Setup PostgresSQL container
	ctx := context.Background()
	containerInstance, err := setupPostgresContainer(ctx)
	require.NoError(t, err)
	defer func() {
		if err := containerInstance.Terminate(ctx); err != nil {
			t.Fatalf("failed to terminate container: %s", err)
		}
	}()

	// Get the Postgres instance
	var postgres *Postgres
	app := fxtest.New(t,
		fx.Provide(
			func() Config {
				return containerInstance.Config
			},
		),
		FXModule,
		fx.Populate(&postgres),
	)

	err = app.Start(ctx)
	require.NoError(t, err)
	defer func() {
		require.NoError(t, app.Stop(ctx))
	}()

	// Auto-migrate TestUser table
	err = postgres.DB().AutoMigrate(&TestUser{})
	require.NoError(t, err)

	t.Run("Create_Simple", func(t *testing.T) {
		ctx := context.Background()

		// Create a user using Query().Create()
		user := TestUser{
			Name:  "Alice",
			Email: "alice@example.com",
			Age:   25,
		}

		rowsAffected, err := postgres.Query(ctx).Create(&user)
		require.NoError(t, err)
		assert.Equal(t, int64(1), rowsAffected)
		assert.NotZero(t, user.ID)

		// Verify user was created
		var fetchedUser TestUser
		err = postgres.First(ctx, &fetchedUser, user.ID)
		require.NoError(t, err)
		assert.Equal(t, "Alice", fetchedUser.Name)
		assert.Equal(t, "alice@example.com", fetchedUser.Email)
		assert.Equal(t, 25, fetchedUser.Age)
	})

	t.Run("Create_WithOnConflictDoNothing", func(t *testing.T) {
		ctx := context.Background()

		// First create a user
		user1 := TestUser{
			Name:  "Bob",
			Email: "bob@unique.com",
			Age:   30,
		}
		err := postgres.Create(ctx, &user1)
		require.NoError(t, err)

		// Try to create another user with same email (should use OnConflict)
		user2 := TestUser{
			Name:  "Bob Duplicate",
			Email: "bob@unique.com", // Same email
			Age:   35,
		}

		// Use OnConflict to ignore duplicate
		rowsAffected, err := postgres.Query(ctx).
			OnConflict(clause.OnConflict{
				Columns:   []clause.Column{{Name: "email"}},
				DoNothing: true,
			}).
			Create(&user2)

		require.NoError(t, err)
		// Should be 0 if conflict was handled
		t.Logf("Rows affected with OnConflict: %d", rowsAffected)

		// Verify only one user with that email exists
		var count int64
		err = postgres.Count(ctx, &TestUser{}, &count, "email = ?", "bob@unique.com")
		require.NoError(t, err)
		assert.Equal(t, int64(1), count)
	})

	t.Run("Create_WithOnConflictUpdate", func(t *testing.T) {
		ctx := context.Background()

		// First create a user
		user1 := TestUser{
			Name:  "Charlie",
			Email: "charlie@example.com",
			Age:   25,
		}
		err := postgres.Create(ctx, &user1)
		require.NoError(t, err)

		originalAge := user1.Age

		// Try to create another user with same email but update age on conflict
		user2 := TestUser{
			Name:  "Charlie Updated",
			Email: "charlie@example.com", // Same email
			Age:   30,
		}

		rowsAffected, err := postgres.Query(ctx).
			OnConflict(clause.OnConflict{
				Columns:   []clause.Column{{Name: "email"}},
				DoUpdates: clause.AssignmentColumns([]string{"age", "name"}),
			}).
			Create(&user2)

		require.NoError(t, err)
		assert.NotZero(t, rowsAffected)

		// Verify user was updated
		var fetchedUser TestUser
		err = postgres.First(ctx, &fetchedUser, "email = ?", "charlie@example.com")
		require.NoError(t, err)
		assert.NotEqual(t, originalAge, fetchedUser.Age)
		assert.Equal(t, 30, fetchedUser.Age)
		assert.Equal(t, "Charlie Updated", fetchedUser.Name)
	})
}

// TestQueryBuilder_ToSubquery tests the ToSubquery() method
func TestQueryBuilder_ToSubquery(t *testing.T) {
	// Skip if running in short mode
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Setup PostgresSQL container
	ctx := context.Background()
	containerInstance, err := setupPostgresContainer(ctx)
	require.NoError(t, err)
	defer func() {
		if err := containerInstance.Terminate(ctx); err != nil {
			t.Fatalf("failed to terminate container: %s", err)
		}
	}()

	// Get the Postgres instance
	var postgres *Postgres
	app := fxtest.New(t,
		fx.Provide(
			func() Config {
				return containerInstance.Config
			},
		),
		FXModule,
		fx.Populate(&postgres),
	)

	err = app.Start(ctx)
	require.NoError(t, err)
	defer func() {
		require.NoError(t, app.Stop(ctx))
	}()

	// Auto-migrate TestUser table
	err = postgres.DB().AutoMigrate(&TestUser{})
	require.NoError(t, err)

	t.Run("ToSubquery_SimpleIN", func(t *testing.T) {
		ctx := context.Background()

		// Create test users
		users := []TestUser{
			{Name: "Alice", Email: "alice@example.com", Age: 25},
			{Name: "Bob", Email: "bob@example.com", Age: 30},
			{Name: "Charlie", Email: "charlie@example.com", Age: 35},
			{Name: "Dave", Email: "dave@example.com", Age: 40},
		}

		for i := range users {
			err := postgres.Create(ctx, &users[i])
			require.NoError(t, err)
		}

		// Use subquery to find users whose age is in a subquery
		// First, get ages > 30 as subquery
		ageSubquery := postgres.Query(ctx).
			Model(&TestUser{}).
			Select("age").
			Where("age > ?", 30).
			ToSubquery()

		// Find users with those ages
		var results []TestUser
		err = postgres.Query(ctx).
			Where("age IN (?)", ageSubquery).
			Find(&results)

		require.NoError(t, err)
		assert.Len(t, results, 2) // Charlie and Dave

		names := make([]string, len(results))
		for i, u := range results {
			names[i] = u.Name
		}
		assert.Contains(t, names, "Charlie")
		assert.Contains(t, names, "Dave")
	})

	t.Run("ToSubquery_NotIN", func(t *testing.T) {
		ctx := context.Background()

		// Create test users
		users := []TestUser{
			{Name: "Eve", Email: "eve@example.com", Age: 20},
			{Name: "Frank", Email: "frank@example.com", Age: 25},
			{Name: "Grace", Email: "grace@example.com", Age: 30},
		}

		for i := range users {
			err := postgres.Create(ctx, &users[i])
			require.NoError(t, err)
		}

		// Use subquery to find users NOT in a specific age range
		youngAgesSubquery := postgres.Query(ctx).
			Model(&TestUser{}).
			Select("age").
			Where("age < ?", 25).
			ToSubquery()

		// Find users not in that age range
		var results []TestUser
		err = postgres.Query(ctx).
			Where("age NOT IN (?)", youngAgesSubquery).
			Find(&results)

		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(results), 2) // Frank and Grace at minimum
	})

	t.Run("ToSubquery_MultipleSubqueries", func(t *testing.T) {
		ctx := context.Background()

		// Create test users
		users := []TestUser{
			{Name: "Henry", Email: "henry@example.com", Age: 22},
			{Name: "Ivy", Email: "ivy@example.com", Age: 28},
			{Name: "Jack", Email: "jack@example.com", Age: 32},
		}

		for i := range users {
			err := postgres.Create(ctx, &users[i])
			require.NoError(t, err)
		}

		// Use two separate queries with subqueries
		// Find minimum age
		minAgeSubquery := postgres.Query(ctx).
			Model(&TestUser{}).
			Select("MIN(age)").
			ToSubquery()

		// Find users who are not at minimum age
		var notMinUsers []TestUser
		err = postgres.Query(ctx).
			Where("age != (?)", minAgeSubquery).
			Find(&notMinUsers)

		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(notMinUsers), 2) // Ivy and Jack

		// Find maximum age
		maxAgeSubquery := postgres.Query(ctx).
			Model(&TestUser{}).
			Select("MAX(age)").
			ToSubquery()

		// Find users who are not at maximum age
		var notMaxUsers []TestUser
		err = postgres.Query(ctx).
			Where("age != (?)", maxAgeSubquery).
			Find(&notMaxUsers)

		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(notMaxUsers), 2) // Henry and Ivy

		// Verify Ivy (middle age) is found in both results
		hasIvyNotMin := false
		hasIvyNotMax := false
		for _, u := range notMinUsers {
			if u.Name == "Ivy" {
				hasIvyNotMin = true
				break
			}
		}
		for _, u := range notMaxUsers {
			if u.Name == "Ivy" {
				hasIvyNotMax = true
				break
			}
		}
		assert.True(t, hasIvyNotMin, "Expected to find Ivy (not min age)")
		assert.True(t, hasIvyNotMax, "Expected to find Ivy (not max age)")
	})
}
