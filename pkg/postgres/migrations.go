package postgres

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// MigrationType defines the type of migration
type MigrationType string

const (
	// SchemaType represents schema changes (tables, columns, indexes, etc.)
	SchemaType MigrationType = "schema"
	// DataType represents data manipulations (inserts, updates, etc.)
	DataType MigrationType = "data"
)

// MigrationDirection specifies the direction of the migration
type MigrationDirection string

const (
	// UpMigration indicates a forward migration
	UpMigration MigrationDirection = "up"
	// DownMigration indicates a rollback migration
	DownMigration MigrationDirection = "down"
)

// Migration represents a single database migration
type Migration struct {
	ID        string             // Unique ID (typically a timestamp or version number)
	Name      string             // Descriptive name
	Type      MigrationType      // Schema or data migration
	Direction MigrationDirection // Up or down migration
	SQL       string             // The SQL to execute
}

// MigrationHistoryRecord represents a record in the migration history table
type MigrationHistoryRecord struct {
	ID           string    // Migration ID
	Name         string    // Migration name
	Type         string    // Migration type (schema/data)
	ExecutedAt   time.Time // When the migration was applied
	ExecutedBy   string    // Who applied the migration
	Duration     int64     // How long it took to apply in milliseconds
	Status       string    // Success or failure
	ErrorMessage string    `gorm:"type:text"` // Error message if failed
}

// AutoMigrate is a wrapper around GORM's AutoMigrate with additional features
func (p *Postgres) AutoMigrate(models ...interface{}) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Ensure the migration history table exists
	if err := p.ensureMigrationHistoryTable(); err != nil {
		return fmt.Errorf("failed to ensure migration history table: %w", err)
	}

	// Execute GORM's AutoMigrate
	if err := p.client.AutoMigrate(models...); err != nil {
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

	if err := p.client.Create(&record).Error; err != nil {
		return fmt.Errorf("failed to record migration: %w", err)
	}

	return nil
}

// ensureMigrationHistoryTable creates the migration history table if it doesn't exist
func (p *Postgres) ensureMigrationHistoryTable() error {
	return p.client.AutoMigrate(&MigrationHistoryRecord{})
}

// MigrateUp applies all pending migrations
func (p *Postgres) MigrateUp(ctx context.Context, migrationsDir string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Ensure the migration history table exists
	if err := p.ensureMigrationHistoryTable(); err != nil {
		return fmt.Errorf("failed to ensure migration history table: %w", err)
	}

	// Get a list of applied migrations
	var applied []MigrationHistoryRecord
	if err := p.client.Find(&applied).Error; err != nil {
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
		tx := p.client.WithContext(ctx).Begin()
		if tx.Error != nil {
			return fmt.Errorf("failed to start transaction: %w", tx.Error)
		}

		// Execute the migration
		if err := tx.Exec(migration.SQL).Error; err != nil {
			tx.Rollback()
			status = "failed"
			errorMsg = err.Error()
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
				return fmt.Errorf("failed to record migration history: %w", err)
			}

			if err := tx.Commit().Error; err != nil {
				return fmt.Errorf("failed to commit transaction: %w", err)
			}

			status = "success"
		}

		// If migration failed, return an error
		if status == "failed" {
			return fmt.Errorf("migration %s failed: %s", migration.ID, errorMsg)
		}
	}

	return nil
}

// MigrateDown rolls back the last applied migration
func (p *Postgres) MigrateDown(ctx context.Context, migrationsDir string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Get the last applied migration
	var lastMigration MigrationHistoryRecord
	if err := p.client.Order("id DESC").First(&lastMigration).Error; err != nil {
		return fmt.Errorf("failed to get last migration: %w", err)
	}

	// Load down migration for this ID
	downMigrations, err := p.loadMigrations(migrationsDir, DownMigration)
	if err != nil {
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
		return fmt.Errorf("no down migration found for %s", lastMigration.ID)
	}

	// Start a transaction
	tx := p.client.WithContext(ctx).Begin()
	if tx.Error != nil {
		return fmt.Errorf("failed to start transaction: %w", tx.Error)
	}

	// Execute the down migration
	if err := tx.Exec(downMigration.SQL).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to apply down migration: %w", err)
	}

	// Remove the migration from history
	if err := tx.Delete(&MigrationHistoryRecord{}, "id = ?", lastMigration.ID).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to update migration history: %w", err)
	}

	if err := tx.Commit().Error; err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// loadMigrations loads migrations from the specified directory
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
		content, err := fs.ReadFile(nil, entry) // This should be adapted to your filesystem
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

// GetMigrationStatus returns the status of all migrations
func (p *Postgres) GetMigrationStatus(ctx context.Context, migrationsDir string) ([]map[string]interface{}, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	// Get a list of applied migrations
	var applied []MigrationHistoryRecord
	if err := p.client.WithContext(ctx).Find(&applied).Error; err != nil {
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

	return status, nil
}

// CreateMigration generates a new migration file
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
