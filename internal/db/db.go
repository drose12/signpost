package db

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"

	_ "github.com/mattn/go-sqlite3"
)

// DB wraps a sql.DB connection with SignPost-specific operations.
type DB struct {
	*sql.DB
}

// Open opens or creates the SQLite database at the given path.
// It enables WAL mode and foreign keys, then runs any pending migrations.
func Open(dbPath string) (*DB, error) {
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0750); err != nil {
		return nil, fmt.Errorf("creating db directory: %w", err)
	}

	sqlDB, err := sql.Open("sqlite3", dbPath+"?_journal_mode=WAL&_foreign_keys=on")
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}

	if err := sqlDB.Ping(); err != nil {
		sqlDB.Close()
		return nil, fmt.Errorf("connecting to database: %w", err)
	}

	db := &DB{sqlDB}

	if err := db.migrate(); err != nil {
		db.Close()
		return nil, fmt.Errorf("running migrations: %w", err)
	}

	return db, nil
}

// migrate applies any pending migrations.
func (db *DB) migrate() error {
	// Ensure schema_migrations table exists (bootstrap).
	// We check if it exists rather than creating it unconditionally
	// because migration 1 also creates it.
	var tableExists int
	err := db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='schema_migrations'`).Scan(&tableExists)
	if err != nil {
		return fmt.Errorf("checking for migrations table: %w", err)
	}

	currentVersion := 0
	if tableExists > 0 {
		err := db.QueryRow(`SELECT COALESCE(MAX(version), 0) FROM schema_migrations`).Scan(&currentVersion)
		if err != nil {
			return fmt.Errorf("getting current schema version: %w", err)
		}
	}

	for i := currentVersion; i < len(migrations); i++ {
		version := i + 1
		log.Printf("Applying migration %d...", version)

		tx, err := db.Begin()
		if err != nil {
			return fmt.Errorf("beginning migration %d: %w", version, err)
		}

		if _, err := tx.Exec(migrations[i]); err != nil {
			tx.Rollback()
			return fmt.Errorf("applying migration %d: %w", version, err)
		}

		if _, err := tx.Exec(`INSERT INTO schema_migrations (version) VALUES (?)`, version); err != nil {
			tx.Rollback()
			return fmt.Errorf("recording migration %d: %w", version, err)
		}

		if err := tx.Commit(); err != nil {
			return fmt.Errorf("committing migration %d: %w", version, err)
		}

		log.Printf("Migration %d applied successfully", version)
	}

	return nil
}

// CheckIntegrity runs SQLite's integrity check and returns any errors found.
func (db *DB) CheckIntegrity() error {
	var result string
	if err := db.QueryRow(`PRAGMA integrity_check`).Scan(&result); err != nil {
		return fmt.Errorf("running integrity check: %w", err)
	}
	if result != "ok" {
		return fmt.Errorf("integrity check failed: %s", result)
	}
	return nil
}

// SchemaVersion returns the current schema migration version.
func (db *DB) SchemaVersion() (int, error) {
	var version int
	err := db.QueryRow(`SELECT COALESCE(MAX(version), 0) FROM schema_migrations`).Scan(&version)
	return version, err
}
