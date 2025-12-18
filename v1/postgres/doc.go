// Package postgres provides functionality for interacting with PostgreSQL databases.
//
// The postgres package offers a robust interface for working with PostgreSQL databases,
// built on top of the standard database/sql package. It includes connection management,
// query execution, transaction handling, and migration tools.
//
// Core Features:
//   - Connection pooling and management
//   - Parameterized query execution
//   - Transaction support with automatic rollback on errors
//   - Schema migration tools
//   - Row scanning utilities
//   - Basic CRUD operations
//
// Basic Usage:
//
//	import (
//		"github.com/Aleph-Alpha/std/v1/postgres"
//		"github.com/Aleph-Alpha/std/v1/logger"
//	)
//
//	// Create a logger
//	log, _ := logger.NewLogger(logger.Config{Level: "info"})
//
//	// Create a new database connection
//	db, err := postgres.New(postgres.Config{
//		Host:     "localhost",
//		Port:     5432,
//		Username: "postgres",
//		Password: "password",
//		Database: "mydb",
//	}, log)
//	if err != nil {
//		log.Fatal("Failed to connect to database", err, nil)
//	}
//	defer db.Close()
//
//	// Execute a query
//	rows, err := db.Query(context.Background(), "SELECT id, name FROM users WHERE age > $1", 18)
//	if err != nil {
//		log.Error("Query failed", err, nil)
//	}
//	defer rows.Close()
//
//	// Scan the results
//	var users []User
//	for rows.Next() {
//		var user User
//		if err := rows.Scan(&user.ID, &user.Name); err != nil {
//			log.Error("Scan failed", err, nil)
//		}
//		users = append(users, user)
//	}
//
// Transaction Example:
//
//	err = db.WithTransaction(context.Background(), func(tx postgres.Transaction) error {
//		// Execute multiple queries in a transaction
//		_, err := tx.Exec("UPDATE accounts SET balance = balance - $1 WHERE id = $2", amount, fromID)
//		if err != nil {
//			return err  // Transaction will be rolled back
//		}
//
//		_, err = tx.Exec("UPDATE accounts SET balance = balance + $1 WHERE id = $2", amount, toID)
//		if err != nil {
//			return err  // Transaction will be rolled back
//		}
//
//		return nil  // Transaction will be committed
//	})
//
// Basic Operations:
//
//	// Create a record
//	id, err := db.Insert(ctx, "INSERT INTO users(name, email) VALUES($1, $2) RETURNING id", "John", "john@example.com")
//
//	// Check if a record exists
//	exists, err := db.Exists(ctx, "SELECT 1 FROM users WHERE email = $1", "john@example.com")
//
//	// Get a single record
//	var user User
//	err = db.Get(ctx, &user, "SELECT id, name, email FROM users WHERE id = $1", id)
//
// FX Module Integration:
//
// This package provides an fx module for easy integration:
//
//	app := fx.New(
//		logger.Module,
//		postgres.Module,
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
//	    if errors.Is(err, postgres.ErrRecordNotFound) {
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
// Performance Considerations:
//
//   - Connection pooling is automatically handled to optimize performance
//   - Prepared statements are used internally to reduce parsing overhead
//   - Consider using batch operations for multiple insertions
//   - Query timeouts are recommended for all database operations
//
// Thread Safety:
//
// All methods on the DB interface are safe for concurrent use by multiple
// goroutines.
package postgres
