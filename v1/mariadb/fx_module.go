package mariadb

import (
	"context"
	"sync"

	"github.com/Aleph-Alpha/std/v1/observability"
	"go.uber.org/fx"
)

// FXModule is an fx module that provides the MariaDB database component.
// It registers the MariaDB constructor for dependency injection
// and sets up lifecycle hooks to properly initialize and shut down
// the database connection.
//
// This module provides:
//   - *MariaDB (concrete type) - for direct use and lifecycle management
//   - Client (interface) - for consumers who want database abstraction
//
// Consumers can inject either:
//   - *MariaDB for full access to all methods
//   - Client for interface-based programming
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

	Config   Config
	Logger   Logger                 `optional:"true"`
	Observer observability.Observer `optional:"true"`
}

// NewMariaDBClientWithDI creates a new MariaDB Client using dependency injection.
// This function is designed to be used with Uber's fx dependency injection framework
// where the Config, Logger, and Observer dependencies are automatically provided via the MariaDBParams struct.
//
// Parameters:
//   - params: A MariaDBParams struct containing the Config instance
//     and optionally a Logger and Observer instances
//     required to initialize the MariaDB Client. This struct embeds fx.In to enable
//     automatic injection of these dependencies.
//
// Returns:
//   - *MariaDB: A fully initialized MariaDB Client (concrete type).
//     The FX module also provides this as Client interface for consumers who want abstraction.
//
// Example usage with fx:
//
//	app := fx.New(
//	    mariadb.FXModule,
//	    logger.FXModule,  // Optional: provides logger
//	    fx.Provide(
//	        func() mariadb.Config {
//	            return loadMariaDBConfig() // Your config loading function
//	        },
//	        func(metrics *prometheus.Metrics) observability.Observer {
//	            return &MyObserver{metrics: metrics}  // Optional observer
//	        },
//	    ),
//	)
//
// This function creates the client and injects the optional logger and observer before returning.
func NewMariaDBClientWithDI(params MariaDBParams) (*MariaDB, error) {
	// Create client with config
	client, err := NewMariaDB(params.Config)
	if err != nil {
		return nil, err
	}

	// Inject logger if provided
	if params.Logger != nil {
		client.logger = params.Logger
	}

	// Inject observer if provided
	if params.Observer != nil {
		client.observer = params.Observer
	}

	return client, nil
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
	if m.shutdownSignal != nil {
		m.closeShutdownOnce.Do(func() {
			close(m.shutdownSignal)
		})
	}

	if m.retryChanSignal != nil {
		m.closeRetryChanOnce.Do(func() {
			close(m.retryChanSignal)
		})
	}

	// Snapshot the connection; do not hold any package-level lock while closing.
	dbConn := m.DB()
	if dbConn == nil {
		return nil
	}

	sqlDB, err := dbConn.DB()
	if err != nil {
		return nil
	}
	if err := sqlDB.Close(); err != nil {
		return err
	}
	return nil
}
