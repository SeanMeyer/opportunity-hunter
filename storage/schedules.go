package storage

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// ScheduleRow represents a row in the hunt_schedules table.
type ScheduleRow struct {
	HuntName      string
	ScanIntervalM int       // minutes (e.g. 720 = 12h)
	NextScanAt    time.Time // next scan time
	UpdatedAt     time.Time
	StartHour     int // 0-23, user-chosen start time
	StartMinute   int // 0-59
	StartDay      int // 0=Sunday..6=Saturday, -1=not applicable (sub-weekly intervals)
}

// GetSchedule returns the schedule for a hunt, or sql.ErrNoRows if not found.
func (d *DB) GetSchedule(ctx context.Context, huntName string) (*ScheduleRow, error) {
	row := d.db.QueryRowContext(ctx,
		`SELECT hunt_name, scan_interval_m, next_scan_at, updated_at,
		        start_hour, start_minute, start_day
		 FROM hunt_schedules WHERE hunt_name = ?`, huntName)

	var s ScheduleRow
	var nextScan, updated string
	if err := row.Scan(&s.HuntName, &s.ScanIntervalM, &nextScan, &updated,
		&s.StartHour, &s.StartMinute, &s.StartDay); err != nil {
		return nil, err
	}
	var err error
	s.NextScanAt, err = time.Parse(time.RFC3339, nextScan)
	if err != nil {
		return nil, fmt.Errorf("parse next_scan_at: %w", err)
	}
	s.UpdatedAt, err = time.Parse(time.RFC3339, updated)
	if err != nil {
		return nil, fmt.Errorf("parse updated_at: %w", err)
	}
	return &s, nil
}

// GetAllSchedules returns schedules for all hunts.
func (d *DB) GetAllSchedules(ctx context.Context) ([]ScheduleRow, error) {
	rows, err := d.db.QueryContext(ctx,
		`SELECT hunt_name, scan_interval_m, next_scan_at, updated_at,
		        start_hour, start_minute, start_day
		 FROM hunt_schedules`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []ScheduleRow
	for rows.Next() {
		var s ScheduleRow
		var nextScan, updated string
		if err := rows.Scan(&s.HuntName, &s.ScanIntervalM, &nextScan, &updated,
			&s.StartHour, &s.StartMinute, &s.StartDay); err != nil {
			return nil, err
		}
		s.NextScanAt, _ = time.Parse(time.RFC3339, nextScan)
		s.UpdatedAt, _ = time.Parse(time.RFC3339, updated)
		result = append(result, s)
	}
	return result, rows.Err()
}

// SaveSchedule upserts a hunt schedule. It recomputes next_scan_at using
// the user's chosen start time, day, and interval.
func (d *DB) SaveSchedule(ctx context.Context, huntName string, intervalMinutes, startHour, startMinute, startDay int, now time.Time) error {
	nextScan := NextScanFrom(now, intervalMinutes, startHour, startMinute, startDay)
	_, err := d.db.ExecContext(ctx,
		`INSERT INTO hunt_schedules (hunt_name, scan_interval_m, next_scan_at, updated_at,
		                             start_hour, start_minute, start_day)
		 VALUES (?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(hunt_name) DO UPDATE SET
		   scan_interval_m = excluded.scan_interval_m,
		   next_scan_at = excluded.next_scan_at,
		   updated_at = excluded.updated_at,
		   start_hour = excluded.start_hour,
		   start_minute = excluded.start_minute,
		   start_day = excluded.start_day`,
		huntName, intervalMinutes, nextScan.Format(time.RFC3339), now.Format(time.RFC3339),
		startHour, startMinute, startDay)
	return err
}

// GetDueHunts returns hunt names where next_scan_at <= now.
func (d *DB) GetDueHunts(ctx context.Context, now time.Time) ([]string, error) {
	rows, err := d.db.QueryContext(ctx,
		`SELECT hunt_name FROM hunt_schedules WHERE next_scan_at <= ?`,
		now.Format(time.RFC3339))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var names []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		names = append(names, name)
	}
	return names, rows.Err()
}

// AdvanceNextScan advances the next_scan_at for a hunt based on its stored schedule.
func (d *DB) AdvanceNextScan(ctx context.Context, huntName string, now time.Time) error {
	sched, err := d.GetSchedule(ctx, huntName)
	if err != nil {
		return fmt.Errorf("get schedule for advance: %w", err)
	}
	nextScan := NextScanFrom(now, sched.ScanIntervalM, sched.StartHour, sched.StartMinute, sched.StartDay)
	_, err = d.db.ExecContext(ctx,
		`UPDATE hunt_schedules SET next_scan_at = ?, updated_at = ? WHERE hunt_name = ?`,
		nextScan.Format(time.RFC3339), now.Format(time.RFC3339), huntName)
	return err
}

// SeedScheduleIfNotExists creates a schedule row for a hunt only if one doesn't already exist.
func (d *DB) SeedScheduleIfNotExists(ctx context.Context, huntName string, intervalMinutes int, now time.Time) error {
	_, err := d.GetSchedule(ctx, huntName)
	if err == nil {
		return nil // already exists
	}
	if err != sql.ErrNoRows {
		return err
	}
	return d.SaveSchedule(ctx, huntName, intervalMinutes, 6, 0, -1, now)
}

// NextScanFrom computes the next scan time after now.
//
// For sub-weekly intervals (startDay == -1), slots repeat from startHour:startMinute:
//   - start 6:00 + 12h → 6:00, 18:00, 6:00+1d, ...
//   - start 9:00 + 6h  → 9:00, 15:00, 21:00, 3:00+1d, ...
//
// For weekly intervals (startDay 0-6), the next occurrence is the chosen
// day-of-week at startHour:startMinute.
func NextScanFrom(now time.Time, intervalMinutes, startHour, startMinute, startDay int) time.Time {
	if intervalMinutes <= 0 {
		intervalMinutes = 720 // default 12h
	}

	loc := now.Location()

	// Weekly: find the next occurrence of the target weekday at the target time.
	if startDay >= 0 && startDay <= 6 && intervalMinutes >= 10080 {
		target := time.Weekday(startDay)
		candidate := time.Date(now.Year(), now.Month(), now.Day(), startHour, startMinute, 0, 0, loc)

		// Advance to the target weekday.
		daysUntil := (int(target) - int(candidate.Weekday()) + 7) % 7
		candidate = candidate.AddDate(0, 0, daysUntil)

		// If that's not after now, jump ahead a week.
		if !candidate.After(now) {
			candidate = candidate.AddDate(0, 0, 7)
		}
		return candidate
	}

	// Sub-weekly: walk from start time in interval steps.
	interval := time.Duration(intervalMinutes) * time.Minute
	candidate := time.Date(now.Year(), now.Month(), now.Day(), startHour, startMinute, 0, 0, loc)

	for candidate.After(now) {
		candidate = candidate.Add(-interval)
	}
	for !candidate.After(now) {
		candidate = candidate.Add(interval)
	}
	return candidate
}
