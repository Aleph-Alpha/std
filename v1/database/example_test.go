package database_test

import (
	"context"
	"testing"

	"github.com/Aleph-Alpha/std/v1/database"
	"github.com/Aleph-Alpha/std/v1/postgres"
)

// Example showing how to create a PostgreSQL config
func ExamplePostgresConfig() {
	cfg := database.PostgresConfig(postgres.Config{
		Connection: postgres.Connection{
			Host:   "localhost",
			Port:   "5432",
			User:   "myuser",
			DbName: "mydb",
		},
	})

	_ = cfg // Use the config with database.FXModule
}

// Example showing how to create a database-agnostic service
func ExampleConfig() {
	// This function would be called by your application
	// to select the database based on configuration
	createDatabase := func(dbType string) database.Config {
		switch dbType {
		case "postgres":
			return database.PostgresConfig(postgres.Config{
				Connection: postgres.Connection{
					Host: "localhost",
					Port: "5432",
				},
			})
		default:
			return database.Config{}
		}
	}

	cfg := createDatabase("postgres")
	_ = cfg // Pass to database.FXModule or NewClientWithDI
}

// Test that config helpers work correctly
func TestConfigHelpers(t *testing.T) {
	t.Run("PostgresConfig", func(t *testing.T) {
		cfg := database.PostgresConfig(postgres.Config{
			Connection: postgres.Connection{
				Host: "localhost",
				Port: "5432",
			},
		})

		if cfg.Type != "postgres" {
			t.Errorf("expected type=postgres, got %s", cfg.Type)
		}
		if cfg.Postgres == nil {
			t.Error("expected Postgres config to be set")
		}
		if cfg.Postgres.Connection.Host != "localhost" {
			t.Errorf("expected host=localhost, got %s", cfg.Postgres.Connection.Host)
		}
	})
}

// Example showing database-agnostic repository pattern
type UserRepository struct {
	db any // Can be *postgres.Postgres or *mariadb.MariaDB
}

func NewUserRepository(db any) *UserRepository {
	return &UserRepository{db: db}
}

func (r *UserRepository) GetUser(ctx context.Context, id int) error {
	// Define interface for methods we need
	type finder interface {
		First(ctx context.Context, dest interface{}, conditions ...interface{}) error
	}

	var user struct{ ID int }
	return r.db.(finder).First(ctx, &user, "id = ?", id)
}

func ExampleUserRepository() {
	// This would come from database.FXModule
	var db any // = injected database client

	repo := NewUserRepository(db)
	_ = repo // Use in your application
}
