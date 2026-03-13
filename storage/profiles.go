package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/seanmeyer/opportunity-hunter/core"
)

// GetProfile returns the user profile for a hunt, falling back to the global profile.
func (d *DB) GetProfile(ctx context.Context, huntName string) (*core.UserProfile, error) {
	p, err := d.getProfileRow(ctx, huntName)
	if err == nil {
		return p, nil
	}
	if !errors.Is(err, ErrNotFound) {
		return nil, err
	}
	// Fall back to global profile.
	if huntName != "" {
		p, err = d.getProfileRow(ctx, "")
		if err == nil {
			return p, nil
		}
		if !errors.Is(err, ErrNotFound) {
			return nil, err
		}
	}
	return nil, ErrNotFound
}

func (d *DB) getProfileRow(ctx context.Context, huntName string) (*core.UserProfile, error) {
	var (
		homeBase, skillLevel, preferences string
		homeLat, homeLon                  float64
		passesJSON, blackoutJSON, extraJSON string
		remoteWork, ptoDays               int
	)
	err := d.db.QueryRowContext(ctx,
		`SELECT home_base, home_lat, home_lon, passes, skill_level, preferences,
		        remote_work, pto_days, blackout_dates, extra
		 FROM user_profiles WHERE hunt_name = ?`, huntName,
	).Scan(&homeBase, &homeLat, &homeLon, &passesJSON, &skillLevel, &preferences,
		&remoteWork, &ptoDays, &blackoutJSON, &extraJSON)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	var passes []string
	json.Unmarshal([]byte(passesJSON), &passes)

	var blackoutStrs []string
	json.Unmarshal([]byte(blackoutJSON), &blackoutStrs)
	var blackoutDates []time.Time
	for _, s := range blackoutStrs {
		if t, err := time.Parse("2006-01-02", s); err == nil {
			blackoutDates = append(blackoutDates, t)
		}
	}

	var extra map[string]any
	json.Unmarshal([]byte(extraJSON), &extra)

	return &core.UserProfile{
		HuntName:      huntName,
		HomeBase:      homeBase,
		HomeLat:       homeLat,
		HomeLon:       homeLon,
		Passes:        passes,
		SkillLevel:    skillLevel,
		Preferences:   preferences,
		RemoteWork:    remoteWork != 0,
		PTODays:       ptoDays,
		BlackoutDates: blackoutDates,
		Extra:         extra,
	}, nil
}

// SaveProfile upserts a user profile. Uses INSERT ... ON CONFLICT DO UPDATE.
func (d *DB) SaveProfile(ctx context.Context, p *core.UserProfile) error {
	passesJSON, _ := json.Marshal(p.Passes)
	blackoutStrs := make([]string, len(p.BlackoutDates))
	for i, d := range p.BlackoutDates {
		blackoutStrs[i] = d.Format("2006-01-02")
	}
	blackoutJSON, _ := json.Marshal(blackoutStrs)
	extraJSON, _ := json.Marshal(p.Extra)

	remoteWork := 0
	if p.RemoteWork {
		remoteWork = 1
	}

	_, err := d.db.ExecContext(ctx,
		`INSERT INTO user_profiles (hunt_name, home_base, home_lat, home_lon, passes,
		 skill_level, preferences, remote_work, pto_days, blackout_dates, extra)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(hunt_name) DO UPDATE SET
		   home_base = excluded.home_base,
		   home_lat = excluded.home_lat,
		   home_lon = excluded.home_lon,
		   passes = excluded.passes,
		   skill_level = excluded.skill_level,
		   preferences = excluded.preferences,
		   remote_work = excluded.remote_work,
		   pto_days = excluded.pto_days,
		   blackout_dates = excluded.blackout_dates,
		   extra = excluded.extra`,
		p.HuntName, p.HomeBase, p.HomeLat, p.HomeLon, string(passesJSON),
		p.SkillLevel, p.Preferences, remoteWork, p.PTODays,
		string(blackoutJSON), string(extraJSON),
	)
	return err
}

// SaveProfileIfNotExists inserts a profile only if one doesn't already exist.
func (d *DB) SaveProfileIfNotExists(ctx context.Context, p *core.UserProfile) error {
	_, err := d.db.ExecContext(ctx,
		`INSERT INTO user_profiles (hunt_name, home_base, home_lat, home_lon, passes,
		 skill_level, preferences, remote_work, pto_days, blackout_dates, extra)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(hunt_name) DO NOTHING`,
		p.HuntName, p.HomeBase, p.HomeLat, p.HomeLon,
		func() string { b, _ := json.Marshal(p.Passes); return string(b) }(),
		p.SkillLevel, p.Preferences,
		func() int { if p.RemoteWork { return 1 }; return 0 }(),
		p.PTODays,
		func() string {
			ss := make([]string, len(p.BlackoutDates))
			for i, d := range p.BlackoutDates { ss[i] = d.Format("2006-01-02") }
			b, _ := json.Marshal(ss); return string(b)
		}(),
		func() string { b, _ := json.Marshal(p.Extra); return string(b) }(),
	)
	return err
}
