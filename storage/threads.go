package storage

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// GetThread retrieves a Discord thread ID for a hunt+group_key.
func (d *DB) GetThread(ctx context.Context, huntName, groupKey string) (string, error) {
	var threadID string
	err := d.db.QueryRowContext(ctx,
		`SELECT thread_id FROM notification_threads WHERE hunt_name = ? AND group_key = ?`,
		huntName, groupKey,
	).Scan(&threadID)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrNotFound
	}
	return threadID, err
}

// SaveThread stores a Discord thread ID. Ignores duplicate inserts.
func (d *DB) SaveThread(ctx context.Context, huntName, groupKey, threadID string) error {
	_, err := d.db.ExecContext(ctx,
		`INSERT INTO notification_threads (hunt_name, group_key, thread_id, created_at)
		 VALUES (?, ?, ?, ?)
		 ON CONFLICT(hunt_name, group_key) DO NOTHING`,
		huntName, groupKey, threadID, time.Now().Format(time.RFC3339),
	)
	return err
}
