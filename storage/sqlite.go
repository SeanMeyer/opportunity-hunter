package storage

import (
	"context"
	"database/sql"
	_ "embed"
	"fmt"

	_ "modernc.org/sqlite"
)

//go:embed schema.sql
var schemaSQL string

// DB wraps a SQLite database connection.
type DB struct {
	db *sql.DB
}

// Open creates or opens a SQLite database at path with WAL mode, busy timeout,
// and foreign keys enabled. Max 1 connection to avoid contention.
func Open(path string) (*DB, error) {
	dsn := path + "?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	db.SetMaxOpenConns(1)

	if _, err := db.ExecContext(context.Background(), schemaSQL); err != nil {
		db.Close()
		return nil, fmt.Errorf("apply schema: %w", err)
	}

	d := &DB{db: db}
	if err := d.runMigrations(context.Background()); err != nil {
		db.Close()
		return nil, fmt.Errorf("run migrations: %w", err)
	}

	return d, nil
}

// runMigrations applies idempotent schema migrations.
// Each migration uses IF NOT EXISTS or similar guards so it's safe to re-run.
func (d *DB) runMigrations(ctx context.Context) error {
	migrations := []string{
		// Add eval_summary and eval_score columns to feedback table.
		`ALTER TABLE feedback ADD COLUMN eval_summary TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE feedback ADD COLUMN eval_score TEXT NOT NULL DEFAULT ''`,
		// Add start_hour/start_minute/start_day to hunt_schedules for user-chosen start time.
		`ALTER TABLE hunt_schedules ADD COLUMN start_hour INTEGER NOT NULL DEFAULT 6`,
		`ALTER TABLE hunt_schedules ADD COLUMN start_minute INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE hunt_schedules ADD COLUMN start_day INTEGER NOT NULL DEFAULT -1`,
		// Add show_dates for multi-date event merging.
		`ALTER TABLE opportunities ADD COLUMN show_dates TEXT NOT NULL DEFAULT '[]'`,
	}

	for _, m := range migrations {
		// Ignore errors from already-applied migrations (column already exists).
		d.db.ExecContext(ctx, m)
	}
	// Do not silently continue if the new decision column could not be installed.
	var exists int
	if err := d.db.QueryRowContext(ctx, `SELECT count(*) FROM pragma_table_info('evaluations') WHERE name = 'structured_response'`).Scan(&exists); err != nil {
		return err
	}
	if exists == 0 {
		if _, err := d.db.ExecContext(ctx, `ALTER TABLE evaluations ADD COLUMN structured_response TEXT NOT NULL DEFAULT ''`); err != nil {
			return err
		}
	}
	if err := d.db.QueryRowContext(ctx, `SELECT count(*) FROM pragma_table_info('opportunities') WHERE name = 'superseded_by'`).Scan(&exists); err != nil {
		return err
	}
	if exists == 0 {
		if _, err := d.db.ExecContext(ctx, `ALTER TABLE opportunities ADD COLUMN superseded_by INTEGER REFERENCES opportunities(id)`); err != nil {
			return err
		}
	}
	if _, err := d.db.ExecContext(ctx, `CREATE INDEX IF NOT EXISTS idx_opportunities_superseded_by ON opportunities(superseded_by)`); err != nil {
		return err
	}
	return nil
}

// Close closes the database connection.
func (d *DB) Close() error { return d.db.Close() }

// RawDB returns the underlying *sql.DB for use in transactions.
func (d *DB) RawDB() *sql.DB { return d.db }
