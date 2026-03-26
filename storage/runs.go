package storage

import (
	"context"
	"crypto/rand"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// PipelineRun represents a single pipeline execution record.
type PipelineRun struct {
	ID           string
	HuntName     string
	StartedAt    time.Time
	FinishedAt   *time.Time
	Status       string
	Scanned      int
	Evaluated    int
	Notified     int
	CostUSD      float64
	ErrorSummary string
	Trigger      string
}

// RunResult holds the outcome of a pipeline run for FinishRun.
type RunResult struct {
	Status       string
	Scanned      int
	Evaluated    int
	Notified     int
	CostUSD      float64
	ErrorSummary string
}

func generateRunID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return fmt.Sprintf("%x", b)
}

func nullIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// InsertRun creates a new pipeline run record and returns its ID.
func (d *DB) InsertRun(ctx context.Context, huntName, trigger string) (string, error) {
	id := generateRunID()
	_, err := d.db.ExecContext(ctx,
		`INSERT INTO pipeline_runs (id, hunt_name, started_at, trigger)
		 VALUES (?, ?, ?, ?)`,
		id, huntName, time.Now().Format(time.RFC3339), trigger,
	)
	return id, err
}

// FinishRun updates a run record with its outcome.
func (d *DB) FinishRun(ctx context.Context, runID string, r RunResult) error {
	_, err := d.db.ExecContext(ctx,
		`UPDATE pipeline_runs
		 SET finished_at = ?, status = ?, scanned = ?, evaluated = ?,
		     notified = ?, cost_usd = ?, error_summary = ?
		 WHERE id = ?`,
		time.Now().Format(time.RFC3339), r.Status, r.Scanned, r.Evaluated,
		r.Notified, r.CostUSD, nullIfEmpty(r.ErrorSummary), runID,
	)
	return err
}

// LatestRun returns the most recent run for a hunt, or nil if none exist.
func (d *DB) LatestRun(ctx context.Context, huntName string) (*PipelineRun, error) {
	row := d.db.QueryRowContext(ctx,
		`SELECT id, hunt_name, started_at, finished_at, status, scanned,
		        evaluated, notified, cost_usd, COALESCE(error_summary, ''), trigger
		 FROM pipeline_runs WHERE hunt_name = ?
		 ORDER BY started_at DESC LIMIT 1`, huntName,
	)
	var r PipelineRun
	var startedAt, trigger string
	var finishedAt *string
	err := row.Scan(&r.ID, &r.HuntName, &startedAt, &finishedAt, &r.Status,
		&r.Scanned, &r.Evaluated, &r.Notified, &r.CostUSD, &r.ErrorSummary, &trigger)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	r.StartedAt, _ = time.Parse(time.RFC3339, startedAt)
	if finishedAt != nil {
		t, _ := time.Parse(time.RFC3339, *finishedAt)
		r.FinishedAt = &t
	}
	r.Trigger = trigger
	return &r, nil
}

// RecentRuns returns the last N runs, optionally filtered by hunt.
func (d *DB) RecentRuns(ctx context.Context, huntName string, limit int) ([]PipelineRun, error) {
	query := `SELECT id, hunt_name, started_at, finished_at, status, scanned,
	                 evaluated, notified, cost_usd, COALESCE(error_summary, ''), trigger
	          FROM pipeline_runs`
	var args []any
	if huntName != "" {
		query += " WHERE hunt_name = ?"
		args = append(args, huntName)
	}
	query += " ORDER BY started_at DESC LIMIT ?"
	args = append(args, limit)

	rows, err := d.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var runs []PipelineRun
	for rows.Next() {
		var r PipelineRun
		var startedAt, trigger string
		var finishedAt *string
		if err := rows.Scan(&r.ID, &r.HuntName, &startedAt, &finishedAt, &r.Status,
			&r.Scanned, &r.Evaluated, &r.Notified, &r.CostUSD, &r.ErrorSummary, &trigger); err != nil {
			return nil, err
		}
		r.StartedAt, _ = time.Parse(time.RFC3339, startedAt)
		if finishedAt != nil {
			t, _ := time.Parse(time.RFC3339, *finishedAt)
			r.FinishedAt = &t
		}
		r.Trigger = trigger
		runs = append(runs, r)
	}
	return runs, rows.Err()
}
