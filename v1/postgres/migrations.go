package postgres

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// MigrationType defines the type of migration, categorizing the purpose of the change.
// This helps track and organize migrations based on their impact on the database.
type MigrationType string

const (
	// SchemaType represents schema changes (tables, columns, indexes, etc.)
	// These migrations modify the structure of the database.
	SchemaType MigrationType = "schema"

	// DataType represents data manipulations (inserts, updates, etc.)
	// These migrations modify the content within the database.
	DataType MigrationType = "data"
)

// MigrationDirection specifies the direction of the migration,
// indicating whether it's applying a change or reverting one.
type MigrationDirection string

const (
	// UpMigration indicates a forward migration that applies a change.
	UpMigration MigrationDirection = "up"

	// DownMigration indicates a rollback migration that reverts a change.
	DownMigration MigrationDirection = "down"
)

// Migration represents a single database migration with all its metadata and content.
// Each migration contains the SQL to execute and information about its purpose and identity.
type Migration struct {
	// ID is a unique identifier for the migration, typically a timestamp (YYYYMMDDHHMMSS)
	// that also helps establish the execution order.
	ID string

	// Name is a descriptive label for the migration, typically describing what it does.
	Name string

	// Type categorizes the migration as either schema or data.
	Type MigrationType

	// Direction indicates whether this is an up (apply) or down (rollback) migration.
	Direction MigrationDirection

	// SQL contains the actual SQL statements to execute for this migration.
	SQL string
}

// MigrationHistoryRecord represents a record in the migration history table.
// It tracks when and how each migration was applied, enabling the system
// to determine which migrations have been run and providing an audit trail.
type MigrationHistoryRecord struct {
	// ID matches the migration ID that was applied.
	ID string

	// Name is the descriptive name of the migration.
	Name string

	// Type indicates whether this was a schema or data migration.
	Type string

	// ExecutedAt records when the migration was applied.
	ExecutedAt time.Time

	// ExecutedBy tracks who or what system applied the migration.
	ExecutedBy string

	// Duration measures how long the migration took to execute in milliseconds.
	Duration int64

	// Status indicates whether the migration completed successfully or failed.
	Status string

	// ErrorMessage contains details if the migration failed.
	ErrorMessage string `gorm:"type:text"`
}

// AutoMigrate is a wrapper around GORM's AutoMigrate with additional features.
// It tracks migrations in the migration history table and provides better error handling.
//
// Parameters:
//   - models: The GORM models to auto-migrate
//
// Returns a GORM error if any part of the migration process fails.
//
// This method is useful during development or for simple applications,
// but for production systems, explicit migrations are recommended.
func (p *Postgres) AutoMigrate(models ...interface{}) error {
	start := time.Now()
	db := p.DB()

	// Ensure the migration history table exists
	if err := p.ensureMigrationHistoryTable(db); err != nil {
		p.observeOperation("auto_migrate", "", "", time.Since(start), err, 0, nil)
		return fmt.Errorf("failed to ensure migration history table: %w", err)
	}

	// Execute GORM's AutoMigrate
	if err := db.AutoMigrate(models...); err != nil {
		p.observeOperation("auto_migrate", "", "", time.Since(start), err, 0, nil)
		return err
	}

	// Record this auto-migration
	record := MigrationHistoryRecord{
		ID:         time.Now().Format("20060102150405"),
		Name:       "auto_migration",
		Type:       string(SchemaType),
		ExecutedAt: time.Now(),
		ExecutedBy: "system",
		Duration:   0, // We don't track the duration for auto-migrations
		Status:     "success",
	}

	if err := db.Create(&record).Error; err != nil {
		p.observeOperation("auto_migrate", "", "", time.Since(start), err, 0, nil)
		return fmt.Errorf("failed to record migration: %w", err)
	}

	p.observeOperation("auto_migrate", "", "", time.Since(start), nil, 0, nil)
	return nil
}

// ensureMigrationHistoryTable creates the migration history table if it doesn't exist.
// This internal method is called before any migration operation to make sure
// the system can track which migrations have been applied.
func (p *Postgres) ensureMigrationHistoryTable(dbConn interface{ AutoMigrate(...interface{}) error }) error {
	return dbConn.AutoMigrate(&MigrationHistoryRecord{})
}

// MigrateUp applies all pending migrations from the specified directory.
// It identifies which migrations haven't been applied yet, sorts them by ID,
// and applies them in order within transactions.
//
// Parameters:
//   - ctx: Context for database operations
//   - migrationsDir: Directory containing the migration SQL files
//
// Returns a wrapped error if any migration fails or if there are issues accessing the migrations.
// The error wraps the underlying GORM error with additional context.
//
// Example:
//
//	err := db.MigrateUp(ctx, "./migrations")
func (p *Postgres) MigrateUp(ctx context.Context, migrationsDir string) error {
	startAll := time.Now()
	db := p.DB().WithContext(ctx)

	// Ensure the migration history table exists
	if err := p.ensureMigrationHistoryTable(db); err != nil {
		p.observeOperation("migrate_up", "", migrationsDir, time.Since(startAll), err, 0, nil)
		return fmt.Errorf("failed to ensure migration history table: %w", err)
	}

	// Get a list of applied migrations
	var applied []MigrationHistoryRecord
	if err := db.Find(&applied).Error; err != nil {
		p.observeOperation("migrate_up", "", migrationsDir, time.Since(startAll), err, 0, nil)
		return fmt.Errorf("failed to get applied migrations: %w", err)
	}

	// Build a map of applied migration IDs for a quick lookup
	appliedMap := make(map[string]bool)
	for _, m := range applied {
		appliedMap[m.ID] = true
	}

	// Get available migrations from the migrations directory
	migrations, err := p.loadMigrations(migrationsDir, UpMigration)
	if err != nil {
		p.observeOperation("migrate_up", "", migrationsDir, time.Since(startAll), err, 0, nil)
		return fmt.Errorf("failed to load migrations: %w", err)
	}

	// Sort migrations by ID to ensure the correct order
	sort.Slice(migrations, func(i, j int) bool {
		return migrations[i].ID < migrations[j].ID
	})

	// Apply pending migrations within a transaction if possible
	for _, migration := range migrations {
		if appliedMap[migration.ID] {
			continue // Skip already applied migrations
		}

		start := time.Now()
		var status, errorMsg string

		// Start a transaction
		tx := db.Begin()
		if tx.Error != nil {
			p.observeOperation("migrate_up", "", migration.ID, time.Since(start), tx.Error, 0, map[string]interface{}{
				"dir": migrationsDir,
			})
			return fmt.Errorf("failed to start transaction: %w", tx.Error)
		}

		// Execute the migration
		if err := tx.Exec(migration.SQL).Error; err != nil {
			tx.Rollback()
			status = "failed"
			errorMsg = err.Error()
			p.observeOperation("migrate_up", "", migration.ID, time.Since(start), err, 0, map[string]interface{}{
				"dir": migrationsDir,
			})
		} else {
			// Record the migration in the history table
			record := MigrationHistoryRecord{
				ID:         migration.ID,
				Name:       migration.Name,
				Type:       string(migration.Type),
				ExecutedAt: time.Now(),
				ExecutedBy: "system", // This could be customized
				Duration:   time.Since(start).Milliseconds(),
				Status:     "success",
			}

			if err := tx.Create(&record).Error; err != nil {
				tx.Rollback()
				p.observeOperation("migrate_up", "", migration.ID, time.Since(start), err, 0, map[string]interface{}{
					"dir": migrationsDir,
				})
				return fmt.Errorf("failed to record migration history: %w", err)
			}

			if err := tx.Commit().Error; err != nil {
				p.observeOperation("migrate_up", "", migration.ID, time.Since(start), err, 0, map[string]interface{}{
					"dir": migrationsDir,
				})
				return fmt.Errorf("failed to commit transaction: %w", err)
			}

			status = "success"
			p.observeOperation("migrate_up", "", migration.ID, time.Since(start), nil, 1, map[string]interface{}{
				"dir": migrationsDir,
			})
		}

		// If migration failed, return an error
		if status == "failed" {
			p.observeOperation("migrate_up", "", migrationsDir, time.Since(startAll), fmt.Errorf("%s", errorMsg), 0, nil)
			return fmt.Errorf("migration %s failed: %s", migration.ID, errorMsg)
		}
	}

	p.observeOperation("migrate_up", "", migrationsDir, time.Since(startAll), nil, 0, nil)
	return nil
}

// MigrateDown rolls back the last applied migration.
// It finds the most recently applied migration and executes its corresponding
// down migration to revert the changes.
//
// Parameters:
//   - ctx: Context for database operations
//   - migrationsDir: Directory containing the migration SQL files
//
// Returns a wrapped error if the rollback fails or if the down migration can't be found.
// The error wraps the underlying GORM error with additional context.
//
// Example:
//
//	err := db.MigrateDown(ctx, "./migrations")
func (p *Postgres) MigrateDown(ctx context.Context, migrationsDir string) error {
	startAll := time.Now()
	db := p.DB().WithContext(ctx)

	// Get the last applied migration
	var lastMigration MigrationHistoryRecord
	if err := db.Order("id DESC").First(&lastMigration).Error; err != nil {
		p.observeOperation("migrate_down", "", migrationsDir, time.Since(startAll), err, 0, nil)
		return fmt.Errorf("failed to get last migration: %w", err)
	}

	// Load down migration for this ID
	downMigrations, err := p.loadMigrations(migrationsDir, DownMigration)
	if err != nil {
		p.observeOperation("migrate_down", "", migrationsDir, time.Since(startAll), err, 0, nil)
		return fmt.Errorf("failed to load down migrations: %w", err)
	}

	var downMigration *Migration
	for _, m := range downMigrations {
		if m.ID == lastMigration.ID {
			downMigration = &m
			break
		}
	}

	if downMigration == nil {
		err := fmt.Errorf("no down migration found for %s", lastMigration.ID)
		p.observeOperation("migrate_down", "", migrationsDir, time.Since(startAll), err, 0, map[string]interface{}{
			"migration_id": lastMigration.ID,
		})
		return fmt.Errorf("no down migration found for %s", lastMigration.ID)
	}

	// Start a transaction
	tx := db.Begin()
	if tx.Error != nil {
		p.observeOperation("migrate_down", "", lastMigration.ID, time.Since(startAll), tx.Error, 0, map[string]interface{}{
			"dir": migrationsDir,
		})
		return fmt.Errorf("failed to start transaction: %w", tx.Error)
	}

	// Execute the down migration
	if err := tx.Exec(downMigration.SQL).Error; err != nil {
		tx.Rollback()
		p.observeOperation("migrate_down", "", lastMigration.ID, time.Since(startAll), err, 0, map[string]interface{}{
			"dir": migrationsDir,
		})
		return fmt.Errorf("failed to apply down migration: %w", err)
	}

	// Remove the migration from history
	if err := tx.Delete(&MigrationHistoryRecord{}, "id = ?", lastMigration.ID).Error; err != nil {
		tx.Rollback()
		p.observeOperation("migrate_down", "", lastMigration.ID, time.Since(startAll), err, 0, map[string]interface{}{
			"dir": migrationsDir,
		})
		return fmt.Errorf("failed to update migration history: %w", err)
	}

	if err := tx.Commit().Error; err != nil {
		p.observeOperation("migrate_down", "", lastMigration.ID, time.Since(startAll), err, 0, map[string]interface{}{
			"dir": migrationsDir,
		})
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	p.observeOperation("migrate_down", "", lastMigration.ID, time.Since(startAll), nil, 1, map[string]interface{}{
		"dir": migrationsDir,
	})
	return nil
}

// loadMigrations loads migrations from the specified directory.
// It parses migration files to extract their metadata and content,
// filtering by the specified direction (up or down).
//
// Parameters:
//   - dir: Directory containing the migration SQL files
//   - direction: Whether to load up (apply) or down (rollback) migrations
//
// Returns a slice of Migration objects and any error encountered.
//
// Migration files should follow the naming convention:
// <version>_<type>_<name>.<direction>.sql
// Example: 20230101120000_schema_create_users_table.up.sql
func (p *Postgres) loadMigrations(dir string, direction MigrationDirection) ([]Migration, error) {
	var migrations []Migration

	entries, err := filepath.Glob(filepath.Join(dir, "*.sql"))
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		filename := filepath.Base(entry)

		// Parse the filename to extract metadata
		// Expected format: <version>_<type>_<name>.<direction>.sql
		// Example: 20230101120000_schema_create_users_table.up.sql
		parts := strings.Split(filename, ".")
		if len(parts) != 3 || parts[2] != "sql" {
			continue
		}

		fileDirection := MigrationDirection(parts[1])
		if fileDirection != direction {
			continue
		}

		nameParts := strings.Split(parts[0], "_")
		if len(nameParts) < 3 {
			continue
		}

		id := nameParts[0]
		migrationType := MigrationType(nameParts[1])
		name := strings.Join(nameParts[2:], "_")

		// Read the SQL content
		content, err := os.ReadFile(entry)
		if err != nil {
			return nil, err
		}

		migrations = append(migrations, Migration{
			ID:        id,
			Name:      name,
			Type:      migrationType,
			Direction: direction,
			SQL:       string(content),
		})
	}

	return migrations, nil
}

// GetMigrationStatus returns the status of all migrations.
// It compares available migrations with those that have been applied
// to build a comprehensive status report.
//
// Parameters:
//   - ctx: Context for database operations
//   - migrationsDir: Directory containing the migration SQL files
//
// Returns a slice of maps with status information for each migration,
// or a wrapped error if the status cannot be determined.
// The error wraps the underlying GORM error with additional context.
//
// Example:
//
//	status, err := db.GetMigrationStatus(ctx, "./migrations")
//	if err == nil {
//	    for _, m := range status {
//	        fmt.Printf("Migration %s: %v\n", m["id"], m["applied"])
//	    }
//	}
func (p *Postgres) GetMigrationStatus(ctx context.Context, migrationsDir string) ([]map[string]interface{}, error) {
	start := time.Now()
	db := p.DB().WithContext(ctx)

	// Get a list of applied migrations
	var applied []MigrationHistoryRecord
	if err := db.Find(&applied).Error; err != nil {
		p.observeOperation("migration_status", "", migrationsDir, time.Since(start), err, 0, nil)
		return nil, fmt.Errorf("failed to get applied migrations: %w", err)
	}

	// Build a map of applied migration IDs
	appliedMap := make(map[string]MigrationHistoryRecord)
	for _, m := range applied {
		appliedMap[m.ID] = m
	}

	// Get available migrations
	upMigrations, err := p.loadMigrations(migrationsDir, UpMigration)
	if err != nil {
		p.observeOperation("migration_status", "", migrationsDir, time.Since(start), err, 0, nil)
		return nil, fmt.Errorf("failed to load migrations: %w", err)
	}

	// Build status
	var status []map[string]interface{}
	for _, m := range upMigrations {
		record, applied := appliedMap[m.ID]

		entry := map[string]interface{}{
			"id":      m.ID,
			"name":    m.Name,
			"type":    m.Type,
			"applied": applied,
		}

		if applied {
			entry["executed_at"] = record.ExecutedAt
			entry["status"] = record.Status
		}

		status = append(status, entry)
	}

	// Sort by ID
	sort.Slice(status, func(i, j int) bool {
		return status[i]["id"].(string) < status[j]["id"].(string)
	})

	p.observeOperation("migration_status", "", migrationsDir, time.Since(start), nil, int64(len(status)), nil)
	return status, nil
}

// CreateMigration generates a new migration file.
// It creates both up and down migration files with appropriate names and timestamps.
//
// Parameters:
//   - migrationsDir: Directory where migration files should be created
//   - name: Descriptive name for the migration
//   - migrationType: Whether this is a schema or data migration
//
// Returns the base filename of the created migration or a wrapped error if creation fails.
//
// Example:
//
//	filename, err := db.CreateMigration("./migrations", "create_users_table", postgres.SchemaType)
//	if err == nil {
//	    fmt.Printf("Created migration: %s\n", filename)
//	}
func (p *Postgres) CreateMigration(migrationsDir, name string, migrationType MigrationType) (string, error) {
	// Generate a timestamp-based ID
	id := time.Now().Format("20060102150405")

	// Create the base filename
	baseFilename := fmt.Sprintf("%s_%s_%s", id, migrationType, name)

	// Create the migration files
	upFilename := filepath.Join(migrationsDir, baseFilename+".up.sql")
	downFilename := filepath.Join(migrationsDir, baseFilename+".down.sql")

	// Create empty migration files with comments
	upTemplate := fmt.Sprintf("-- Migration: %s\n-- Type: %s\n-- Created: %s\n\n", name, migrationType, time.Now().Format(time.RFC3339))
	downTemplate := fmt.Sprintf("-- Migration: %s (rollback)\n-- Type: %s\n-- Created: %s\n\n", name, migrationType, time.Now().Format(time.RFC3339))

	// Ensure the migrations directory exists
	if err := os.MkdirAll(migrationsDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create migrations directory: %w", err)
	}

	// Write the files
	if err := os.WriteFile(upFilename, []byte(upTemplate), 0644); err != nil {
		return "", fmt.Errorf("failed to create up migration file: %w", err)
	}

	if err := os.WriteFile(downFilename, []byte(downTemplate), 0644); err != nil {
		return "", fmt.Errorf("failed to create down migration file: %w", err)
	}

	return baseFilename, nil
}
