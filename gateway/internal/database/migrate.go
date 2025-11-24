package database

import (
	"embed"
	"fmt"
	"sort"
	"strings"

	"github.com/jmoiron/sqlx"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// Migration represents a single database migration
type Migration struct {
	Version int
	Name    string
	UpSQL   string
	DownSQL string
}

// Migrator handles database migrations
type Migrator struct {
	db         *sqlx.DB
	migrations []Migration
}

// NewMigrator creates a new migrator instance
func NewMigrator(db *DB) (*Migrator, error) {
	m := &Migrator{
		db:         db.DB,
		migrations: []Migration{},
	}

	if err := m.loadMigrations(); err != nil {
		return nil, fmt.Errorf("failed to load migrations: %w", err)
	}

	return m, nil
}

// loadMigrations reads migration files from the embedded filesystem
func (m *Migrator) loadMigrations() error {
	entries, err := migrationsFS.ReadDir("migrations")
	if err != nil {
		return fmt.Errorf("failed to read migrations directory: %w", err)
	}

	migrationMap := make(map[int]*Migration)

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		if !strings.HasSuffix(name, ".sql") {
			continue
		}

		// Parse filename: 001_initial_schema.up.sql or 001_initial_schema.down.sql
		parts := strings.Split(name, "_")
		if len(parts) < 2 {
			continue
		}

		var version int
		if _, err := fmt.Sscanf(parts[0], "%d", &version); err != nil {
			continue
		}

		content, err := migrationsFS.ReadFile("migrations/" + name)
		if err != nil {
			return fmt.Errorf("failed to read migration file %s: %w", name, err)
		}

		if migrationMap[version] == nil {
			migrationMap[version] = &Migration{
				Version: version,
				Name:    strings.TrimSuffix(strings.Join(parts[1:], "_"), ".up.sql"),
			}
		}

		if strings.HasSuffix(name, ".up.sql") {
			migrationMap[version].UpSQL = string(content)
		} else if strings.HasSuffix(name, ".down.sql") {
			migrationMap[version].DownSQL = string(content)
		}
	}

	// Convert map to sorted slice
	versions := make([]int, 0, len(migrationMap))
	for v := range migrationMap {
		versions = append(versions, v)
	}
	sort.Ints(versions)

	for _, v := range versions {
		m.migrations = append(m.migrations, *migrationMap[v])
	}

	return nil
}

// Up runs all pending migrations
func (m *Migrator) Up() error {
	if err := m.ensureMigrationsTable(); err != nil {
		return err
	}

	currentVersion, err := m.getCurrentVersion()
	if err != nil {
		return err
	}

	for _, migration := range m.migrations {
		if migration.Version <= currentVersion {
			continue
		}

		fmt.Printf("Running migration %d: %s\n", migration.Version, migration.Name)

		tx, err := m.db.Beginx()
		if err != nil {
			return fmt.Errorf("failed to begin transaction: %w", err)
		}

		if _, err := tx.Exec(migration.UpSQL); err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to execute migration %d: %w", migration.Version, err)
		}

		if err := m.recordMigration(tx, migration.Version); err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to record migration %d: %w", migration.Version, err)
		}

		if err := tx.Commit(); err != nil {
			return fmt.Errorf("failed to commit migration %d: %w", migration.Version, err)
		}

		fmt.Printf("Migration %d completed successfully\n", migration.Version)
	}

	return nil
}

// Down rolls back the last migration
func (m *Migrator) Down() error {
	if err := m.ensureMigrationsTable(); err != nil {
		return err
	}

	currentVersion, err := m.getCurrentVersion()
	if err != nil {
		return err
	}

	if currentVersion == 0 {
		fmt.Println("No migrations to roll back")
		return nil
	}

	var migration *Migration
	for i := range m.migrations {
		if m.migrations[i].Version == currentVersion {
			migration = &m.migrations[i]
			break
		}
	}

	if migration == nil {
		return fmt.Errorf("migration %d not found", currentVersion)
	}

	fmt.Printf("Rolling back migration %d: %s\n", migration.Version, migration.Name)

	tx, err := m.db.Beginx()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	if _, err := tx.Exec(migration.DownSQL); err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to execute rollback %d: %w", migration.Version, err)
	}

	if err := m.removeMigration(tx, migration.Version); err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to remove migration record %d: %w", migration.Version, err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit rollback %d: %w", migration.Version, err)
	}

	fmt.Printf("Migration %d rolled back successfully\n", migration.Version)

	return nil
}

// ensureMigrationsTable creates the schema_migrations table if it doesn't exist
func (m *Migrator) ensureMigrationsTable() error {
	query := `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version INTEGER PRIMARY KEY,
			applied_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
		)
	`
	_, err := m.db.Exec(query)
	return err
}

// getCurrentVersion returns the current migration version
func (m *Migrator) getCurrentVersion() (int, error) {
	var version int
	err := m.db.Get(&version, "SELECT COALESCE(MAX(version), 0) FROM schema_migrations")
	if err != nil {
		return 0, fmt.Errorf("failed to get current version: %w", err)
	}
	return version, nil
}

// recordMigration records a migration in the schema_migrations table
func (m *Migrator) recordMigration(tx *sqlx.Tx, version int) error {
	_, err := tx.Exec("INSERT INTO schema_migrations (version) VALUES ($1)", version)
	return err
}

// removeMigration removes a migration record from the schema_migrations table
func (m *Migrator) removeMigration(tx *sqlx.Tx, version int) error {
	_, err := tx.Exec("DELETE FROM schema_migrations WHERE version = $1", version)
	return err
}

// Status returns the current migration status
func (m *Migrator) Status() (int, int, error) {
	currentVersion, err := m.getCurrentVersion()
	if err != nil {
		return 0, 0, err
	}

	totalMigrations := len(m.migrations)
	return currentVersion, totalMigrations, nil
}
