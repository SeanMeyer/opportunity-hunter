package storage

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// DistanceRow represents a cached distance calculation.
type DistanceRow struct {
	VenueID     int64
	HomeAddress string
	Mode        string // "walking" or "driving"
	Minutes     int
	DistanceMi  float64
	CreatedAt   time.Time
}

// GetDistance retrieves a cached distance for a venue, address, and mode.
func (d *DB) GetDistance(ctx context.Context, venueID int64, homeAddress, mode string) (DistanceRow, error) {
	var r DistanceRow
	var createdStr string
	err := d.db.QueryRowContext(ctx,
		`SELECT venue_id, home_address, mode, minutes, distance_mi, created_at
		 FROM distance_cache WHERE venue_id = ? AND home_address = ? AND mode = ?`,
		venueID, homeAddress, mode,
	).Scan(&r.VenueID, &r.HomeAddress, &r.Mode, &r.Minutes, &r.DistanceMi, &createdStr)
	if errors.Is(err, sql.ErrNoRows) {
		return r, ErrNotFound
	}
	if err != nil {
		return r, err
	}
	r.CreatedAt, _ = time.Parse(time.RFC3339, createdStr)
	return r, nil
}

// SaveDistance caches a distance calculation.
func (d *DB) SaveDistance(ctx context.Context, r DistanceRow) error {
	_, err := d.db.ExecContext(ctx,
		`INSERT INTO distance_cache (venue_id, home_address, mode, minutes, distance_mi, created_at)
		 VALUES (?, ?, ?, ?, ?, ?)
		 ON CONFLICT(venue_id, home_address, mode) DO UPDATE SET
		   minutes = excluded.minutes, distance_mi = excluded.distance_mi, created_at = excluded.created_at`,
		r.VenueID, r.HomeAddress, r.Mode, r.Minutes, r.DistanceMi,
		r.CreatedAt.Format(time.RFC3339),
	)
	return err
}
