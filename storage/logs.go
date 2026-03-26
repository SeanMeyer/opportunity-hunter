package storage

import (
	"context"
	"time"
)

// RunLog represents a single structured log entry.
type RunLog struct {
	ID        int64
	RunID     string
	HuntName  string
	Timestamp time.Time
	Level     string
	Message   string
	Attrs     string
}

// InsertLogs batch-inserts log entries.
func (d *DB) InsertLogs(ctx context.Context, logs []RunLog) error {
	if len(logs) == 0 {
		return nil
	}
	tx, err := d.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx,
		`INSERT INTO run_logs (run_id, hunt_name, timestamp, level, message, attrs)
		 VALUES (?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, l := range logs {
		runID := nullIfEmpty(l.RunID)
		huntName := nullIfEmpty(l.HuntName)
		_, err := stmt.ExecContext(ctx, runID, huntName,
			l.Timestamp.Format(time.RFC3339), l.Level, l.Message, l.Attrs)
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}

// RecentLogs returns the most recent log entries, optionally filtered.
func (d *DB) RecentLogs(ctx context.Context, huntName, level string, limit int) ([]RunLog, error) {
	query := `SELECT id, COALESCE(run_id, ''), COALESCE(hunt_name, ''),
	                 timestamp, level, message, attrs
	          FROM run_logs WHERE 1=1`
	var args []any
	if huntName != "" {
		query += " AND hunt_name = ?"
		args = append(args, huntName)
	}
	if level != "" {
		query += " AND level = ?"
		args = append(args, level)
	}
	query += " ORDER BY timestamp DESC, id DESC LIMIT ?"
	args = append(args, limit)

	rows, err := d.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []RunLog
	for rows.Next() {
		var l RunLog
		var ts string
		if err := rows.Scan(&l.ID, &l.RunID, &l.HuntName, &ts,
			&l.Level, &l.Message, &l.Attrs); err != nil {
			return nil, err
		}
		l.Timestamp, _ = time.Parse(time.RFC3339, ts)
		results = append(results, l)
	}
	return results, rows.Err()
}

// PruneLogs deletes log entries older than the given duration.
func (d *DB) PruneLogs(ctx context.Context, olderThan time.Duration) (int64, error) {
	cutoff := time.Now().Add(-olderThan).Format(time.RFC3339)
	result, err := d.db.ExecContext(ctx,
		`DELETE FROM run_logs WHERE timestamp < ?`, cutoff)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}
