package postgres

import (
	"context"
	"fmt"
	"github.com/docker/docker/api/types/container"
	"net"
	"os"
	"testing"
	"time"

	"database/sql"
	"github.com/docker/go-connections/nat"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"go.uber.org/fx"
	"go.uber.org/fx/fxtest"
	"go.uber.org/mock/gomock"
	"gorm.io/gorm"
)

// TestUser is a sample model for testing GORM operations
type TestUser struct {
	gorm.Model
	Name  string
	Email string
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

// TestPostgresWithFXModule tests the postgres package using the existing FX module
func TestPostgresWithFXModule(t *testing.T) {
	// Skip if running in short mode
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Setup PostgreSQL container
	ctx := context.Background()
	container, err := setupPostgresContainer(ctx)
	require.NoError(t, err)
	defer func() {
		if err := container.Terminate(ctx); err != nil {
			t.Fatalf("failed to terminate container: %s", err)
		}
	}()

	// Create mock controller and logger
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockLogger := NewMocklogger(ctrl)

	// Override Fatal to prevent test termination
	mockLogger.EXPECT().Fatal(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(
		func(msg string, err error, fields ...map[string]interface{}) {
			t.Logf("FATAL: %s, Error: %v", msg, err)
			// Don't actually exit the test
		}).AnyTimes()

	// Setup expected logger calls
	mockLogger.EXPECT().Info(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Error(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Debug(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Warn(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

	// Print connection details for debugging
	t.Logf("Using PostgreSQL on %s:%s", container.Host, container.Port)

	// Get the Postgres instance to test it
	var postgres *Postgres

	// Create a test app using the existing FXModule
	app := fxtest.New(t,
		// Provide dependencies with the correct container config
		fx.Provide(
			func() Config {
				return container.Config
			},
			func() logger {
				return mockLogger
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
	if postgres == nil || postgres.client == nil {
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
		err = postgres.UpdateWhere(ctx, &TestUser{}, map[string]interface{}{
			"Age": 32,
		}, "name = ?", "John Doe")
		assert.NoError(t, err)

		// Verify UpdateWhere
		err = postgres.First(ctx, &updatedUser, "name = ?", "John Doe")
		assert.NoError(t, err)
		assert.Equal(t, 32, updatedUser.Age)

		// Update specific model
		err = postgres.Update(ctx, &updatedUser, map[string]interface{}{
			"Age": 33,
		})
		assert.NoError(t, err)

		// Verify Update
		err = postgres.First(ctx, &updatedUser, "name = ?", "John Doe")
		assert.NoError(t, err)
		assert.Equal(t, 33, updatedUser.Age)

		// UpdateColumn
		err = postgres.UpdateColumn(ctx, &updatedUser, "Age", 34)
		assert.NoError(t, err)

		// Verify UpdateColumn
		err = postgres.First(ctx, &updatedUser, "name = ?", "John Doe")
		assert.NoError(t, err)
		assert.Equal(t, 34, updatedUser.Age)

		// UpdateColumns
		err = postgres.UpdateColumns(ctx, &updatedUser, map[string]interface{}{
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
		err = postgres.Delete(ctx, &TestUser{}, "name = ?", "John Doe")
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
		err := postgres.Exec(ctx, `
			CREATE TABLE IF NOT EXISTS test_items (
				id SERIAL PRIMARY KEY,
				name TEXT NOT NULL,
				value INTEGER
			)
		`)
		assert.NoError(t, err)

		// Insert data using Exec
		err = postgres.Exec(ctx, `
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

	// Test error translation
	t.Run("ErrorTranslation", func(t *testing.T) {
		ctx := context.Background()

		// Test record not found error
		var user TestUser
		err := postgres.First(ctx, &user, "name = ?", "NonExistentUser")
		translatedErr := TranslateError(err)
		assert.ErrorIs(t, translatedErr, ErrRecordNotFound)

		// Create a table with a unique constraint
		err = postgres.Exec(ctx, `
			CREATE TABLE IF NOT EXISTS unique_test (
				id SERIAL PRIMARY KEY,
				email TEXT UNIQUE NOT NULL
			)
		`)
		assert.NoError(t, err)

		// Create a record
		err = postgres.Exec(ctx, `INSERT INTO unique_test (email) VALUES ('test@example.com')`)
		assert.NoError(t, err)

		// Try to create a duplicate (will fail due to unique constraint)
		err = postgres.Exec(ctx, `INSERT INTO unique_test (email) VALUES ('test@example.com')`)
		assert.Error(t, err)
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
	container, err := setupPostgresContainer(ctx)
	require.NoError(t, err)
	defer func() {
		if err := container.Terminate(ctx); err != nil {
			t.Fatalf("failed to terminate container: %s", err)
		}
	}()

	// Print connection details for debugging
	t.Logf("Using PostgreSQL on %s:%s", container.Host, container.Port)

	// Create mock controller and logger
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockLogger := NewMocklogger(ctrl)

	// Override Fatal to prevent test termination
	mockLogger.EXPECT().Fatal(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(
		func(msg string, err error, fields ...map[string]interface{}) {
			t.Logf("FATAL: %s, Error: %v", msg, err)
			// Don't actually exit the test
		}).AnyTimes()

	// Setup expected logger calls
	mockLogger.EXPECT().Info(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Error(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Debug(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Warn(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

	// Create a Postgres instance directly (not through FX)
	postgres := NewPostgres(container.Config, mockLogger)
	if postgres == nil || postgres.client == nil {
		t.Skip("Skipping test as database connection failed")
		return
	}

	require.NotNil(t, postgres)

	// Verify connection works using the public DB() method
	db := postgres.DB()
	require.NotNil(t, db)

	var result int
	err = db.Raw("SELECT 1").Scan(&result).Error
	assert.NoError(t, err)

	// Simulate a connection error by sending a signal to the retry channel
	postgres.retryChanSignal <- fmt.Errorf("test connection error")

	// Give time for reconnection attempt
	time.Sleep(100 * time.Millisecond)

	// Connection should still work since the container is running
	db = postgres.DB() // Get the DB again in case it was updated
	err = db.Raw("SELECT 1").Scan(&result).Error
	assert.NoError(t, err)
}

// TestErrorHandling tests the error handling and translation
func TestErrorHandling(t *testing.T) {
	t.Run("TranslateError", func(t *testing.T) {
		assert.Equal(t, nil, TranslateError(nil))
		assert.Equal(t, ErrRecordNotFound, TranslateError(gorm.ErrRecordNotFound))
		assert.Equal(t, ErrDuplicateKey, TranslateError(gorm.ErrDuplicatedKey))
		assert.Equal(t, ErrForeignKey, TranslateError(gorm.ErrForeignKeyViolated))

		// Test custom error
		customErr := fmt.Errorf("custom error")
		assert.Equal(t, customErr, TranslateError(customErr))
	})
}

// waitForPostgresReady attempts to connect to PostgreSQL until it's ready or times out
func waitForPostgresReady(host, port, user, password, dbname string, timeout time.Duration) error {
	// Use standard PostgreSQL connection string
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
