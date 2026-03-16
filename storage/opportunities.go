package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/seanmeyer/opportunity-hunter/core"
)

// InsertOpportunity inserts a new opportunity and returns its ID.
func (d *DB) InsertOpportunity(ctx context.Context, opp core.Opportunity) (int64, error) {
	attrs := string(opp.Attributes)
	if attrs == "" {
		attrs = "{}"
	}

	showDatesJSON := marshalShowDates(opp.ShowDates)

	result, err := d.db.ExecContext(ctx,
		`INSERT INTO opportunities
		 (hunt_name, source_id, source, title, subtitle, venue_id,
		  start_time, end_time, price_min, price_max, ticket_url,
		  state, attributes, raw_data, discovered_at, show_dates)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		opp.HuntName, opp.SourceID, opp.Source, opp.Title, opp.Subtitle, opp.VenueID,
		opp.StartTime.Format(time.RFC3339), nullTimeStr(opp.EndTime),
		opp.PriceMin, opp.PriceMax, opp.TicketURL,
		string(opp.State), attrs, opp.RawData,
		opp.DiscoveredAt.Format(time.RFC3339), showDatesJSON,
	)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

// GetOpportunity retrieves an opportunity by ID.
func (d *DB) GetOpportunity(ctx context.Context, id int64) (core.Opportunity, error) {
	return d.scanOpportunity(d.db.QueryRowContext(ctx,
		`SELECT id, hunt_name, source_id, source, title, subtitle, venue_id,
		        start_time, end_time, price_min, price_max, ticket_url,
		        state, attributes, raw_data, discovered_at,
		        evaluated_at, notified_at, reminded_at, show_dates
		 FROM opportunities WHERE id = ?`, id,
	))
}

// GetByState returns all opportunities for a hunt in a given state.
func (d *DB) GetByState(ctx context.Context, huntName string, state core.State) ([]core.Opportunity, error) {
	rows, err := d.db.QueryContext(ctx,
		`SELECT id, hunt_name, source_id, source, title, subtitle, venue_id,
		        start_time, end_time, price_min, price_max, ticket_url,
		        state, attributes, raw_data, discovered_at,
		        evaluated_at, notified_at, reminded_at, show_dates
		 FROM opportunities WHERE hunt_name = ? AND state = ?
		 ORDER BY start_time`, huntName, string(state),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return d.scanOpportunities(rows)
}

// OpportunityExists checks if an opportunity with the given source_id exists for a hunt.
func (d *DB) OpportunityExists(ctx context.Context, huntName, sourceID string) (bool, error) {
	var count int
	err := d.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM opportunities WHERE hunt_name = ? AND source_id = ?`,
		huntName, sourceID,
	).Scan(&count)
	return count > 0, err
}

// GetRawItemsForDedup returns lightweight RawItem stubs for all opportunities in a hunt,
// suitable for recomputing dedup keys against incoming scan results.
func (d *DB) GetRawItemsForDedup(ctx context.Context, huntName string) ([]core.RawItem, error) {
	rows, err := d.db.QueryContext(ctx,
		`SELECT o.title, COALESCE(v.name, ''), o.start_time, COALESCE(o.end_time, ''), o.show_dates
		 FROM opportunities o
		 LEFT JOIN venues v ON o.venue_id = v.id
		 WHERE o.hunt_name = ?`, huntName,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []core.RawItem
	for rows.Next() {
		var item core.RawItem
		var showDatesStr string
		if err := rows.Scan(&item.Title, &item.VenueName, &item.StartTime, &item.EndTime, &showDatesStr); err != nil {
			continue
		}
		item.ShowDates = unmarshalShowDates(showDatesStr)
		items = append(items, item)
	}
	return items, rows.Err()
}

// UpdateState transitions an opportunity to a new state with an optional timestamp.
func (d *DB) UpdateState(ctx context.Context, id int64, state core.State, at *time.Time) error {
	var atStr *string
	if at != nil {
		s := at.Format(time.RFC3339)
		atStr = &s
	}

	var query string
	var args []any
	switch state {
	case core.Evaluated:
		query = `UPDATE opportunities SET state = ?, evaluated_at = ? WHERE id = ?`
		args = []any{string(state), atStr, id}
	case core.Notified:
		query = `UPDATE opportunities SET state = ?, notified_at = ? WHERE id = ?`
		args = []any{string(state), atStr, id}
	case core.Reminded:
		query = `UPDATE opportunities SET state = ?, reminded_at = ? WHERE id = ?`
		args = []any{string(state), atStr, id}
	default:
		query = `UPDATE opportunities SET state = ? WHERE id = ?`
		args = []any{string(state), id}
	}

	_, err := d.db.ExecContext(ctx, query, args...)
	return err
}

// GetUpcoming returns notified/reminded opportunities starting within the given window.
func (d *DB) GetUpcoming(ctx context.Context, huntName string, within time.Duration) ([]core.Opportunity, error) {
	now := time.Now()
	cutoff := now.Add(within)
	rows, err := d.db.QueryContext(ctx,
		`SELECT id, hunt_name, source_id, source, title, subtitle, venue_id,
		        start_time, end_time, price_min, price_max, ticket_url,
		        state, attributes, raw_data, discovered_at,
		        evaluated_at, notified_at, reminded_at, show_dates
		 FROM opportunities
		 WHERE hunt_name = ? AND state IN ('notified', 'reminded')
		   AND start_time >= ? AND start_time <= ?
		 ORDER BY start_time`,
		huntName, now.Format(time.RFC3339), cutoff.Format(time.RFC3339),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return d.scanOpportunities(rows)
}

func (d *DB) scanOpportunity(row *sql.Row) (core.Opportunity, error) {
	var opp core.Opportunity
	var startStr, discoveredStr string
	var endStr, evalStr, notifStr, remindStr sql.NullString
	var venueID sql.NullInt64
	var priceMin, priceMax sql.NullFloat64
	var stateStr string
	var attrsStr string
	var showDatesStr string

	err := row.Scan(
		&opp.ID, &opp.HuntName, &opp.SourceID, &opp.Source,
		&opp.Title, &opp.Subtitle, &venueID,
		&startStr, &endStr, &priceMin, &priceMax, &opp.TicketURL,
		&stateStr, &attrsStr, &opp.RawData, &discoveredStr,
		&evalStr, &notifStr, &remindStr, &showDatesStr,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return opp, ErrNotFound
	}
	if err != nil {
		return opp, err
	}

	opp.State = core.State(stateStr)
	opp.Attributes = core.Attributes(attrsStr)
	opp.StartTime, _ = time.Parse(time.RFC3339, startStr)
	opp.DiscoveredAt, _ = time.Parse(time.RFC3339, discoveredStr)
	opp.ShowDates = unmarshalShowDates(showDatesStr)
	if venueID.Valid {
		opp.VenueID = &venueID.Int64
	}
	if priceMin.Valid {
		opp.PriceMin = &priceMin.Float64
	}
	if priceMax.Valid {
		opp.PriceMax = &priceMax.Float64
	}
	if endStr.Valid && endStr.String != "" {
		t, _ := time.Parse(time.RFC3339, endStr.String)
		opp.EndTime = &t
	}
	if evalStr.Valid && evalStr.String != "" {
		t, _ := time.Parse(time.RFC3339, evalStr.String)
		opp.EvaluatedAt = &t
	}
	if notifStr.Valid && notifStr.String != "" {
		t, _ := time.Parse(time.RFC3339, notifStr.String)
		opp.NotifiedAt = &t
	}
	if remindStr.Valid && remindStr.String != "" {
		t, _ := time.Parse(time.RFC3339, remindStr.String)
		opp.RemindedAt = &t
	}

	return opp, nil
}

func (d *DB) scanOpportunities(rows *sql.Rows) ([]core.Opportunity, error) {
	var result []core.Opportunity
	for rows.Next() {
		var opp core.Opportunity
		var startStr, discoveredStr string
		var endStr, evalStr, notifStr, remindStr sql.NullString
		var venueID sql.NullInt64
		var priceMin, priceMax sql.NullFloat64
		var stateStr string
		var attrsStr string
		var showDatesStr string

		err := rows.Scan(
			&opp.ID, &opp.HuntName, &opp.SourceID, &opp.Source,
			&opp.Title, &opp.Subtitle, &venueID,
			&startStr, &endStr, &priceMin, &priceMax, &opp.TicketURL,
			&stateStr, &attrsStr, &opp.RawData, &discoveredStr,
			&evalStr, &notifStr, &remindStr, &showDatesStr,
		)
		if err != nil {
			return nil, err
		}

		opp.State = core.State(stateStr)
		opp.Attributes = core.Attributes(attrsStr)
		opp.StartTime, _ = time.Parse(time.RFC3339, startStr)
		opp.DiscoveredAt, _ = time.Parse(time.RFC3339, discoveredStr)
		opp.ShowDates = unmarshalShowDates(showDatesStr)
		if venueID.Valid {
			opp.VenueID = &venueID.Int64
		}
		if priceMin.Valid {
			opp.PriceMin = &priceMin.Float64
		}
		if priceMax.Valid {
			opp.PriceMax = &priceMax.Float64
		}
		if endStr.Valid && endStr.String != "" {
			t, _ := time.Parse(time.RFC3339, endStr.String)
			opp.EndTime = &t
		}
		if evalStr.Valid && evalStr.String != "" {
			t, _ := time.Parse(time.RFC3339, evalStr.String)
			opp.EvaluatedAt = &t
		}
		if notifStr.Valid && notifStr.String != "" {
			t, _ := time.Parse(time.RFC3339, notifStr.String)
			opp.NotifiedAt = &t
		}
		if remindStr.Valid && remindStr.String != "" {
			t, _ := time.Parse(time.RFC3339, remindStr.String)
			opp.RemindedAt = &t
		}

		result = append(result, opp)
	}
	return result, rows.Err()
}

func nullTimeStr(t *time.Time) *string {
	if t == nil {
		return nil
	}
	s := t.Format(time.RFC3339)
	return &s
}

func marshalShowDates(dates []time.Time) string {
	if len(dates) == 0 {
		return "[]"
	}
	strs := make([]string, len(dates))
	for i, d := range dates {
		strs[i] = d.Format(time.RFC3339)
	}
	b, _ := json.Marshal(strs)
	return string(b)
}

func unmarshalShowDates(s string) []time.Time {
	if s == "" || s == "[]" {
		return nil
	}
	var strs []string
	if err := json.Unmarshal([]byte(s), &strs); err != nil {
		return nil
	}
	dates := make([]time.Time, 0, len(strs))
	for _, str := range strs {
		t, err := time.Parse(time.RFC3339, str)
		if err == nil {
			dates = append(dates, t)
		}
	}
	return dates
}
