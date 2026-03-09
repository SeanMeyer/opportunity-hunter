package storage

import (
	"context"
	"database/sql"
	"errors"
)

// SavePreferences upserts preferences for a hunt.
func (d *DB) SavePreferences(ctx context.Context, huntName, text string) error {
	_, err := d.db.ExecContext(ctx,
		`INSERT INTO preferences (hunt_name, preferences_text) VALUES (?, ?)
		 ON CONFLICT(hunt_name) DO UPDATE SET preferences_text = excluded.preferences_text`,
		huntName, text,
	)
	return err
}

// GetPreferences returns the preferences text for a hunt.
func (d *DB) GetPreferences(ctx context.Context, huntName string) (string, error) {
	var text string
	err := d.db.QueryRowContext(ctx,
		`SELECT preferences_text FROM preferences WHERE hunt_name = ?`, huntName,
	).Scan(&text)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return text, err
}
