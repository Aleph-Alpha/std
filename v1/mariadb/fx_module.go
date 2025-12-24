package mariadb

import (
	"context"
	"sync"

	"go.uber.org/fx"
)

// FXModule is an fx module that provides the MariaDB database component.
// It registers the MariaDB constructor for dependency injection
// and sets up lifecycle hooks to properly initialize and shut down
// the database connection.
//
// This module provides Client interface, not *MariaDB concrete type.
var FXModule = fx.Module("mariadb",
	fx.Provide(
		NewMariaDBClientWithDI, // Returns *MariaDB for internal lifecycle
		fx.Annotate(
			ProvideClient,      // Returns Client interface
			fx.As(new(Client)), // Expose as Client interface
		),
	),
	fx.Invoke(RegisterMariaDBLifecycle),
)

// ProvideClient wraps the concrete *MariaDB and returns it as Client interface.
// This enables applications to depend on the interface rather than concrete type.
func ProvideClient(db *MariaDB) Client {
	return db
}

// MariaDBParams groups the dependencies needed to create a MariaDB Client via dependency injection.
// This struct is designed to work with Uber's fx dependency injection framework and provides
// the necessary parameters for initializing a MariaDB database connection.
//
// The embedded fx.In marker enables automatic injection of the struct fields from the
// dependency container when this struct is used as a parameter in provider functions.
type MariaDBParams struct {
	fx.In

	Config Config
}

// NewMariaDBClientWithDI creates a new MariaDB Client using dependency injection.
// This function is designed to be used with Uber's fx dependency injection framework
// where the Config dependency is automatically provided via the MariaDBParams struct.
//
// Parameters:
//   - params: A MariaDBParams struct containing the Config instance
//     required to initialize the MariaDB Client. This struct embeds fx.In to enable
//     automatic injection of these dependencies.
//
// Returns:
//   - *MariaDB: A fully initialized MariaDB Client (concrete type for lifecycle management).
//     To use the interface, inject Client instead.
//
// Example usage with fx:
//
//	app := fx.New(
//	    mariadb.FXModule,
//	    fx.Provide(
//	        func() mariadb.Config {
//	            return loadMariaDBConfig() // Your config loading function
//	        },
//	    ),
//	)
//
// This function delegates to the standard NewMariaDB function, maintaining the same
// initialization logic while enabling seamless integration with dependency injection.
func NewMariaDBClientWithDI(params MariaDBParams) (*MariaDB, error) {
	client, err := NewMariaDB(params.Config)
	if err != nil {
		return nil, err
	}
	// Return concrete type for lifecycle management
	return client.(*MariaDB), nil
}

// MariaDBLifeCycleParams groups the dependencies needed for MariaDB lifecycle management.
// This struct combines all the components required to properly manage the lifecycle
// of a MariaDB Client within an fx application, including startup, monitoring,
// and graceful shutdown.
//
// The embedded fx.In marker enables automatic injection of the struct fields from the
// dependency container when this struct is used as a parameter in lifecycle registration functions.
type MariaDBLifeCycleParams struct {
	fx.In

	Lifecycle fx.Lifecycle
	MariaDB   *MariaDB
}

// RegisterMariaDBLifecycle registers lifecycle hooks for the MariaDB database component.
// It sets up:
// 1. Connection monitoring on the application starts
// 2. Automatic reconnection mechanism on application start
// 3. Graceful shutdown of database connections on application stop
//
// The function uses a WaitGroup to ensure that all goroutines complete
// before the application terminates.
func RegisterMariaDBLifecycle(params MariaDBLifeCycleParams) {
	wg := &sync.WaitGroup{}
	params.Lifecycle.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			wg.Add(1)
			go func() {
				defer wg.Done()
				params.MariaDB.MonitorConnection(ctx)
			}()

			wg.Add(1)
			go func() {
				defer wg.Done()
				params.MariaDB.RetryConnection(ctx)
			}()

			return nil
		},
		OnStop: func(ctx context.Context) error {
			params.MariaDB.closeShutdownOnce.Do(func() {
				close(params.MariaDB.shutdownSignal)
			})

			wg.Wait()

			params.MariaDB.closeRetryChanOnce.Do(func() {
				close(params.MariaDB.retryChanSignal)
			})

			// Close the database connection
			sqlDB, err := params.MariaDB.DB().DB()
			if err == nil {
				err := sqlDB.Close()
				if err != nil {
					return err
				}
			}

			return nil
		},
	})
}

func (m *MariaDB) GracefulShutdown() error {
	m.closeShutdownOnce.Do(func() {
		close(m.shutdownSignal)
	})

	m.closeRetryChanOnce.Do(func() {
		close(m.retryChanSignal)
	})

	m.mu.Lock()
	// Close the database connection
	sqlDB, err := m.DB().DB()
	if err == nil {
		err := sqlDB.Close()
		if err != nil {
			return err
		}
	}
	m.mu.Unlock()
	return nil
}
