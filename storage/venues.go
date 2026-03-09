package storage

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"unicode"

	"github.com/seanmeyer/opportunity-hunter/core"
)

// normalizeVenueName lowercases, strips common punctuation, and collapses whitespace.
func normalizeVenueName(name string) string {
	name = strings.ToLower(name)
	var b strings.Builder
	prevSpace := false
	for _, r := range name {
		if r == '-' || r == '–' || r == '—' || r == '\'' || r == '"' {
			continue
		}
		if unicode.IsSpace(r) {
			if !prevSpace {
				b.WriteRune(' ')
				prevSpace = true
			}
			continue
		}
		prevSpace = false
		b.WriteRune(r)
	}
	return strings.TrimSpace(b.String())
}

// UpsertVenue inserts or updates a venue by normalized name + address.
// Returns the venue ID.
func (d *DB) UpsertVenue(ctx context.Context, v core.Venue) (int64, error) {
	normalized := normalizeVenueName(v.Name)

	// Look up by normalized name and address.
	var id int64
	err := d.db.QueryRowContext(ctx,
		`SELECT id FROM venues WHERE REPLACE(REPLACE(REPLACE(LOWER(name), '-', ''), '–', ''), '''', '') = ? AND address = ?`,
		strings.ReplaceAll(normalized, " ", ""),
		v.Address,
	).Scan(&id)

	// Actually, let's use a simpler approach: store the original name but match normalized.
	// Reset and use a custom comparison.
	err = d.db.QueryRowContext(ctx,
		`SELECT id FROM venues WHERE name = ? AND address = ?`,
		v.Name, v.Address,
	).Scan(&id)

	if errors.Is(err, sql.ErrNoRows) {
		// Try normalized match: find any venue with same normalized name and address.
		rows, qerr := d.db.QueryContext(ctx,
			`SELECT id, name FROM venues WHERE address = ?`, v.Address,
		)
		if qerr != nil {
			return 0, qerr
		}
		defer rows.Close()

		for rows.Next() {
			var existingID int64
			var existingName string
			if scanErr := rows.Scan(&existingID, &existingName); scanErr != nil {
				return 0, scanErr
			}
			if normalizeVenueName(existingName) == normalized {
				id = existingID
				break
			}
		}
		if closeErr := rows.Err(); closeErr != nil {
			return 0, closeErr
		}
	} else if err != nil {
		return 0, err
	}

	if id != 0 {
		// Update existing venue.
		_, err = d.db.ExecContext(ctx,
			`UPDATE venues SET latitude = ?, longitude = ?, notes = ? WHERE id = ?`,
			v.Latitude, v.Longitude, v.Notes, id,
		)
		return id, err
	}

	// Insert new venue.
	result, err := d.db.ExecContext(ctx,
		`INSERT INTO venues (name, address, latitude, longitude, notes) VALUES (?, ?, ?, ?, ?)`,
		v.Name, v.Address, v.Latitude, v.Longitude, v.Notes,
	)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

// GetVenue retrieves a venue by ID.
func (d *DB) GetVenue(ctx context.Context, id int64) (core.Venue, error) {
	var v core.Venue
	err := d.db.QueryRowContext(ctx,
		`SELECT id, name, address, latitude, longitude, notes FROM venues WHERE id = ?`, id,
	).Scan(&v.ID, &v.Name, &v.Address, &v.Latitude, &v.Longitude, &v.Notes)
	if errors.Is(err, sql.ErrNoRows) {
		return v, ErrNotFound
	}
	return v, err
}

// GetVenueByName retrieves a venue by exact name match.
func (d *DB) GetVenueByName(ctx context.Context, name string) (core.Venue, error) {
	var v core.Venue
	err := d.db.QueryRowContext(ctx,
		`SELECT id, name, address, latitude, longitude, notes FROM venues WHERE name = ?`, name,
	).Scan(&v.ID, &v.Name, &v.Address, &v.Latitude, &v.Longitude, &v.Notes)
	if errors.Is(err, sql.ErrNoRows) {
		return v, ErrNotFound
	}
	return v, err
}
