// Package mariadb provides functionality for interacting with MariaDB and MySQL databases.
//
// The mariadb package offers a robust interface for working with MariaDB/MySQL databases,
// built on top of GORM ORM. It includes connection management, query execution,
// transaction handling, and migration tools.
//
// Core Features:
//   - Connection pooling and management with automatic reconnection
//   - Parameterized query execution via GORM
//   - Transaction support with automatic rollback on errors
//   - Schema migration tools
//   - Row scanning utilities
//   - Basic CRUD operations with thread safety
//   - Query builder for complex queries
//
// Basic Usage:
//
//	import (
//		"github.com/Aleph-Alpha/std/v1/mariadb"
//	)
//
//	// Create a new database connection
//	db, err := mariadb.NewMariaDB(mariadb.Config{
//		Connection: mariadb.Connection{
//			Host:      "localhost",
//			Port:      "3306",
//			User:      "root",
//			Password:  "password",
//			DbName:    "mydb",
//			Charset:   "utf8mb4",
//			ParseTime: true,
//		},
//	})
//	if err != nil {
//		log.Fatal("Failed to connect to database", err)
//	}
//	defer db.GracefulShutdown()
//
//	// Execute a query
//	var users []User
//	err = db.Find(context.Background(), &users, "age > ?", 18)
//	if err != nil {
//		log.Error("Query failed", err)
//	}
//
// Transaction Example:
//
//	err = db.Transaction(ctx, func(txDB *mariadb.MariaDB) error {
//		// Execute multiple queries in a transaction
//		if err := txDB.Create(ctx, &user); err != nil {
//			return err  // Transaction will be rolled back
//		}
//
//		if err := txDB.Create(ctx, &userProfile); err != nil {
//			return err  // Transaction will be rolled back
//		}
//
//		return nil  // Transaction will be committed
//	})
//
// Query Builder Example:
//
//	// Build complex queries
//	var users []User
//	err := db.Query(ctx).
//	    Where("age > ?", 18).
//	    Where("status = ?", "active").
//	    Order("created_at DESC").
//	    Limit(10).
//	    Find(&users)
//
// Basic Operations:
//
//	// Create a record
//	err := db.Create(ctx, &user)
//
//	// Get a single record
//	var user User
//	err = db.First(ctx, &user, "email = ?", "john@example.com")
//
//	// Update records
//	rowsAffected, err := db.Update(ctx, &user, map[string]interface{}{
//	    "name": "John Doe Updated",
//	})
//
//	// Delete records
//	rowsAffected, err := db.Delete(ctx, &User{}, "id = ?", 123)
//
// FX Module Integration:
//
// This package provides an fx module for easy integration:
//
//	app := fx.New(
//		mariadb.FXModule,
//		fx.Provide(
//			func() mariadb.Config {
//				return mariadb.Config{
//					Connection: mariadb.Connection{
//						Host:     "localhost",
//						Port:     "3306",
//						User:     "root",
//						Password: "password",
//						DbName:   "mydb",
//					},
//				}
//			},
//		),
//		// ... other modules
//	)
//	app.Run()
//
// Error Handling:
//
// All methods in this package return GORM errors directly. This design provides:
//   - Consistency: All methods behave the same way
//   - Flexibility: Consumers can use errors.Is() with GORM error types
//   - Performance: No translation overhead unless needed
//   - Transparency: Preserves the full error chain from GORM
//
// Basic error handling with GORM errors:
//
//	var user User
//	err := db.First(ctx, &user, "email = ?", "user@example.com")
//	if errors.Is(err, gorm.ErrRecordNotFound) {
//	    // Handle not found
//	}
//
//	err = db.Query(ctx).Where("email = ?", "user@example.com").First(&user)
//	if errors.Is(err, gorm.ErrRecordNotFound) {
//	    // Handle not found
//	}
//
// For standardized error types, use TranslateError():
//
//	err := db.First(ctx, &user, conditions)
//	if err != nil {
//	    err = db.TranslateError(err)
//	    if errors.Is(err, mariadb.ErrRecordNotFound) {
//	        // Handle not found with standardized error
//	    }
//	}
//
// Recommended pattern - create a helper function for common error checks:
//
//	func isRecordNotFound(err error) bool {
//	    return errors.Is(err, gorm.ErrRecordNotFound)
//	}
//
//	// Use consistently throughout your codebase
//	if isRecordNotFound(err) {
//	    // Handle not found
//	}
//
// Differences from PostgreSQL Package:
//
// This package provides nearly identical functionality to the postgres package,
// with the following notable differences:
//
//   - Connection configuration uses MySQL DSN format instead of PostgreSQL format
//   - ForNoKeyUpdate() is not available (PostgreSQL-only feature)
//   - ForKeyShare() is not available (PostgreSQL-only feature)
//   - Returning() clause is not available (use LAST_INSERT_ID() for inserts)
//   - SKIP LOCKED and NOWAIT require MySQL 8.0+ or MariaDB 10.3+/10.6+
//
// Performance Considerations:
//
//   - Connection pooling is automatically handled to optimize performance
//   - Prepared statements are used internally to reduce parsing overhead
//   - Consider using batch operations for multiple insertions
//   - Query timeouts are recommended for all database operations
//
// Thread Safety:
//
// All methods on the MariaDB interface are safe for concurrent use by multiple
// goroutines. Internal mutexes ensure thread-safe access to the database connection.
package mariadb
