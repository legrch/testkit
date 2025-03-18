package testkit

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// TableConfig holds configuration for a table's primary keys
type TableConfig struct {
	PrimaryKeys []string
}

// FixtureConfig holds configuration for fixture loading
type FixtureConfig struct {
	// File extensions to consider as fixtures (defaults to [".yml", ".yaml"])
	FileExtensions []string
}

// DefaultFixtureConfig returns the default fixture configuration
func DefaultFixtureConfig() *FixtureConfig {
	return &FixtureConfig{
		FileExtensions: []string{".yml", ".yaml"},
	}
}

// FixtureManager handles loading and cleaning up database fixtures
type FixtureManager struct {
	db     *sql.DB
	config *FixtureConfig
	// Map of table name to its configuration for non-standard primary keys
	tableConfigs map[string]TableConfig
	// Track inserted records by table and their primary key values
	insertedRecords map[string][]map[string]any
}

// TableFixtures represents fixtures for all tables
type TableFixtures map[string][]map[string]any

// NewFixtureManager creates a new fixture manager
func NewFixtureManager(db *sql.DB) *FixtureManager {
	return NewFixtureManagerWithConfig(db, DefaultFixtureConfig())
}

// NewFixtureManagerWithConfig creates a new fixture manager with the given configuration
func NewFixtureManagerWithConfig(db *sql.DB, config *FixtureConfig) *FixtureManager {
	return &FixtureManager{
		db:              db,
		config:          config,
		tableConfigs:    make(map[string]TableConfig),
		insertedRecords: make(map[string][]map[string]any),
	}
}

// ConfigureTable sets custom primary key configuration for a table
// Only needed when the primary key is not 'id'
func (fm *FixtureManager) ConfigureTable(tableName string, primaryKeys []string) {
	fm.tableConfigs[tableName] = TableConfig{
		PrimaryKeys: primaryKeys,
	}
}

// getPrimaryKeys returns the primary keys for a table
// Uses 'id' by default unless configured otherwise
func (fm *FixtureManager) getPrimaryKeys(tableName string) []string {
	if config, ok := fm.tableConfigs[tableName]; ok {
		return config.PrimaryKeys
	}
	return []string{"id"}
}

// LoadYAMLFixtures loads fixtures from a YAML file
func (fm *FixtureManager) LoadYAMLFixtures(fixturePath string) error {
	content, err := os.ReadFile(fixturePath)
	if err != nil {
		return fmt.Errorf("failed to read fixture file: %w", err)
	}

	var fixtures TableFixtures
	if err2 := yaml.Unmarshal(content, &fixtures); err2 != nil {
		return fmt.Errorf("failed to unmarshal YAML fixtures: %w", err2)
	}

	// Begin transaction
	tx, err := fm.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
			log.Printf("failed to rollback transaction: %v", err)
		}
	}()

	// Process each table
	for tableName, records := range fixtures {
		if err := fm.insertRecords(tx, tableName, records); err != nil {
			return fmt.Errorf("failed to insert records for table %s: %w", tableName, err)
		}
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// insertRecords inserts records for a specific table
func (fm *FixtureManager) insertRecords(tx *sql.Tx, tableName string, records []map[string]any) error {
	for _, record := range records {
		// Extract columns and values
		var columns []string
		var placeholders []string
		var values []any
		i := 1

		// Track primary key values for cleanup
		pkValues := make(map[string]any)
		primaryKeys := fm.getPrimaryKeys(tableName)
		for _, pk := range primaryKeys {
			if value, exists := record[pk]; exists {
				pkValues[pk] = value
			}
		}

		// Store primary key values for cleanup
		if len(pkValues) > 0 {
			if _, exists := fm.insertedRecords[tableName]; !exists {
				fm.insertedRecords[tableName] = make([]map[string]any, 0)
			}
			fm.insertedRecords[tableName] = append(fm.insertedRecords[tableName], pkValues)
		}

		for column, value := range record {
			columns = append(columns, column)
			placeholders = append(placeholders, fmt.Sprintf("$%d", i))

			// Handle special values
			switch v := value.(type) {
			case string:
				if v == "NOW()" {
					values = append(values, time.Now())
				} else {
					values = append(values, v)
				}
			default:
				values = append(values, v)
			}
			i++
		}

		// Build and execute query
		// This is safe because we're using quoted identifiers and parameterized values
		//nolint:gosec // G201: SQL string formatting is safe here with quoted identifiers
		query := fmt.Sprintf(
			"INSERT INTO %s (%s) VALUES (%s)",
			tableName,
			strings.Join(columns, ", "),
			strings.Join(placeholders, ", "),
		)

		if _, err := tx.Exec(query, values...); err != nil {
			return fmt.Errorf("failed to insert record: %w", err)
		}
	}

	return nil
}

// CleanupFixtures removes test data from the database
func (fm *FixtureManager) CleanupFixtures() error {
	if len(fm.insertedRecords) == 0 {
		return nil // Nothing to clean up
	}

	// Begin transaction
	tx, err := fm.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin cleanup transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
			log.Printf("failed to rollback cleanup transaction: %v", err)
		}
	}()

	// Clean up each table's inserted records
	for tableName, records := range fm.insertedRecords {
		if len(records) == 0 {
			continue
		}

		primaryKeys := fm.getPrimaryKeys(tableName)

		// Build WHERE clause for composite keys
		var conditions []string
		var values []any
		paramCount := 1

		for _, record := range records {
			var recordConditions []string
			var recordValues []any

			for _, pk := range primaryKeys {
				if value, exists := record[pk]; exists {
					recordConditions = append(recordConditions, fmt.Sprintf("%s = $%d", pk, paramCount))
					recordValues = append(recordValues, value)
					paramCount++
				}
			}

			if len(recordConditions) > 0 {
				conditions = append(conditions, "("+strings.Join(recordConditions, " AND ")+")")
				values = append(values, recordValues...)
			}
		}

		if len(conditions) > 0 {
			// Build and execute delete query
			// This is safe because we're using quoted identifiers and parameterized values
			//nolint:gosec // G201: SQL string formatting is safe here with quoted identifiers
			query := fmt.Sprintf(
				"DELETE FROM %s WHERE %s",
				tableName,
				strings.Join(conditions, " OR "),
			)

			if _, err := tx.Exec(query, values...); err != nil {
				return fmt.Errorf("failed to cleanup table %s: %w", tableName, err)
			}
		}
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit cleanup transaction: %w", err)
	}

	// Clear the tracking map after successful cleanup
	fm.insertedRecords = make(map[string][]map[string]any)

	return nil
}

// LoadFixturesFromDir loads all YAML fixtures from a directory
func (fm *FixtureManager) LoadFixturesFromDir(fixturesDir string) error {
	entries, err := os.ReadDir(fixturesDir)
	if err != nil {
		return fmt.Errorf("failed to read fixtures directory: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() && fm.isFixtureFile(entry.Name()) {
			fixturePath := filepath.Join(fixturesDir, entry.Name())
			if err := fm.LoadYAMLFixtures(fixturePath); err != nil {
				return fmt.Errorf("failed to load fixture %s: %w", entry.Name(), err)
			}
		}
	}

	return nil
}

// isFixtureFile checks if a file is a fixture file based on its extension
func (fm *FixtureManager) isFixtureFile(filename string) bool {
	ext := filepath.Ext(filename)
	for _, validExt := range fm.config.FileExtensions {
		if ext == validExt {
			return true
		}
	}
	return false
}
