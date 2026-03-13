package storage

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func openTestDB(t *testing.T) *DB {
	t.Helper()
	db, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestNextScanFrom_SubWeekly(t *testing.T) {
	loc := time.Local
	tests := []struct {
		name        string
		now         time.Time
		interval    int
		startHour   int
		startMinute int
		want        time.Time
	}{
		{
			name:      "start 6am + 12h at 5am → 6am today",
			now:       time.Date(2026, 3, 13, 5, 0, 0, 0, loc),
			interval:  720,
			startHour: 6,
			want:      time.Date(2026, 3, 13, 6, 0, 0, 0, loc),
		},
		{
			name:      "start 6am + 12h at 10am → 6pm today",
			now:       time.Date(2026, 3, 13, 10, 0, 0, 0, loc),
			interval:  720,
			startHour: 6,
			want:      time.Date(2026, 3, 13, 18, 0, 0, 0, loc),
		},
		{
			name:      "start 6am + 12h at 19:00 → 6am next day",
			now:       time.Date(2026, 3, 13, 19, 0, 0, 0, loc),
			interval:  720,
			startHour: 6,
			want:      time.Date(2026, 3, 14, 6, 0, 0, 0, loc),
		},
		{
			name:      "start 9am + 6h at 10am → 3pm",
			now:       time.Date(2026, 3, 13, 10, 0, 0, 0, loc),
			interval:  360,
			startHour: 9,
			want:      time.Date(2026, 3, 13, 15, 0, 0, 0, loc),
		},
		{
			name:      "start 9am + 6h at 22:00 → 3am next day",
			now:       time.Date(2026, 3, 13, 22, 0, 0, 0, loc),
			interval:  360,
			startHour: 9,
			want:      time.Date(2026, 3, 14, 3, 0, 0, 0, loc),
		},
		{
			name:      "start 7am + 24h at 10am → 7am next day",
			now:       time.Date(2026, 3, 13, 10, 0, 0, 0, loc),
			interval:  1440,
			startHour: 7,
			want:      time.Date(2026, 3, 14, 7, 0, 0, 0, loc),
		},
		{
			name:      "start 7am + 24h at 5am → 7am today",
			now:       time.Date(2026, 3, 13, 5, 0, 0, 0, loc),
			interval:  1440,
			startHour: 7,
			want:      time.Date(2026, 3, 13, 7, 0, 0, 0, loc),
		},
		{
			name:        "start 6:30am + 12h at 10am → 6:30pm",
			now:         time.Date(2026, 3, 13, 10, 0, 0, 0, loc),
			interval:    720,
			startHour:   6,
			startMinute: 30,
			want:        time.Date(2026, 3, 13, 18, 30, 0, 0, loc),
		},
		{
			name:      "1h interval start 0:00 at 14:30 → 15:00",
			now:       time.Date(2026, 3, 13, 14, 30, 0, 0, loc),
			interval:  60,
			startHour: 0,
			want:      time.Date(2026, 3, 13, 15, 0, 0, 0, loc),
		},
		{
			name:      "zero interval defaults to 12h",
			now:       time.Date(2026, 3, 13, 5, 0, 0, 0, loc),
			interval:  0,
			startHour: 6,
			want:      time.Date(2026, 3, 13, 6, 0, 0, 0, loc),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NextScanFrom(tt.now, tt.interval, tt.startHour, tt.startMinute, -1)
			if !got.Equal(tt.want) {
				t.Errorf("NextScanFrom(%v, %d, %d, %d, -1) = %v, want %v",
					tt.now, tt.interval, tt.startHour, tt.startMinute, got, tt.want)
			}
		})
	}
}

func TestNextScanFrom_Weekly(t *testing.T) {
	loc := time.Local
	// 2026-03-13 is a Friday (weekday 5).
	tests := []struct {
		name      string
		now       time.Time
		startDay  int // 0=Sun..6=Sat
		startHour int
		want      time.Time
	}{
		{
			name:      "weekly Monday at 6am, now is Friday 10am → next Monday",
			now:       time.Date(2026, 3, 13, 10, 0, 0, 0, loc),
			startDay:  1, // Monday
			startHour: 6,
			want:      time.Date(2026, 3, 16, 6, 0, 0, 0, loc), // Mon Mar 16
		},
		{
			name:      "weekly Friday at 6am, now is Friday 5am → today 6am",
			now:       time.Date(2026, 3, 13, 5, 0, 0, 0, loc),
			startDay:  5, // Friday
			startHour: 6,
			want:      time.Date(2026, 3, 13, 6, 0, 0, 0, loc),
		},
		{
			name:      "weekly Friday at 6am, now is Friday 10am → next Friday",
			now:       time.Date(2026, 3, 13, 10, 0, 0, 0, loc),
			startDay:  5, // Friday
			startHour: 6,
			want:      time.Date(2026, 3, 20, 6, 0, 0, 0, loc),
		},
		{
			name:      "weekly Sunday at 9am, now is Friday → Sunday",
			now:       time.Date(2026, 3, 13, 10, 0, 0, 0, loc),
			startDay:  0, // Sunday
			startHour: 9,
			want:      time.Date(2026, 3, 15, 9, 0, 0, 0, loc),
		},
		{
			name:      "weekly Saturday at 8am, now is Friday → tomorrow",
			now:       time.Date(2026, 3, 13, 10, 0, 0, 0, loc),
			startDay:  6, // Saturday
			startHour: 8,
			want:      time.Date(2026, 3, 14, 8, 0, 0, 0, loc),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NextScanFrom(tt.now, 10080, tt.startHour, 0, tt.startDay)
			if !got.Equal(tt.want) {
				t.Errorf("NextScanFrom(%v, 10080, %d, 0, %d) = %v, want %v",
					tt.now, tt.startHour, tt.startDay, got, tt.want)
			}
		})
	}
}

func TestSaveAndGetSchedule(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	now := time.Date(2026, 3, 13, 10, 0, 0, 0, time.UTC)

	err := db.SaveSchedule(ctx, "powder", 720, 6, 0, -1, now)
	if err != nil {
		t.Fatal(err)
	}

	sched, err := db.GetSchedule(ctx, "powder")
	if err != nil {
		t.Fatal(err)
	}
	if sched.HuntName != "powder" {
		t.Errorf("got hunt %q, want powder", sched.HuntName)
	}
	if sched.ScanIntervalM != 720 {
		t.Errorf("got interval %d, want 720", sched.ScanIntervalM)
	}
	if sched.StartHour != 6 || sched.StartMinute != 0 {
		t.Errorf("got start %d:%d, want 6:00", sched.StartHour, sched.StartMinute)
	}
	if sched.StartDay != -1 {
		t.Errorf("got start_day %d, want -1", sched.StartDay)
	}
	if !sched.NextScanAt.After(now) {
		t.Errorf("next_scan_at %v should be after %v", sched.NextScanAt, now)
	}
}

func TestSaveScheduleUpsert(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	now := time.Date(2026, 3, 13, 10, 0, 0, 0, time.UTC)

	db.SaveSchedule(ctx, "powder", 720, 6, 0, -1, now)
	db.SaveSchedule(ctx, "powder", 10080, 9, 30, 1, now) // weekly Monday 9:30

	sched, _ := db.GetSchedule(ctx, "powder")
	if sched.ScanIntervalM != 10080 {
		t.Errorf("upsert failed: got interval %d, want 10080", sched.ScanIntervalM)
	}
	if sched.StartHour != 9 || sched.StartMinute != 30 {
		t.Errorf("upsert failed: got start %d:%d, want 9:30", sched.StartHour, sched.StartMinute)
	}
	if sched.StartDay != 1 {
		t.Errorf("upsert failed: got start_day %d, want 1", sched.StartDay)
	}
}

func TestGetDueHunts(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	past := time.Date(2026, 3, 12, 0, 0, 0, 0, time.UTC)
	future := time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC)

	db.SaveSchedule(ctx, "powder", 720, 6, 0, -1, past)
	db.SaveSchedule(ctx, "comedy", 720, 6, 0, -1, future)

	now := time.Date(2026, 3, 13, 10, 0, 0, 0, time.UTC)
	due, err := db.GetDueHunts(ctx, now)
	if err != nil {
		t.Fatal(err)
	}

	if len(due) != 1 || due[0] != "powder" {
		t.Errorf("got due hunts %v, want [powder]", due)
	}
}

func TestSeedScheduleIfNotExists(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	now := time.Date(2026, 3, 13, 10, 0, 0, 0, time.UTC)

	err := db.SeedScheduleIfNotExists(ctx, "powder", 720, now)
	if err != nil {
		t.Fatal(err)
	}

	err = db.SeedScheduleIfNotExists(ctx, "powder", 360, now)
	if err != nil {
		t.Fatal(err)
	}

	sched, _ := db.GetSchedule(ctx, "powder")
	if sched.ScanIntervalM != 720 {
		t.Errorf("seed overwrote: got interval %d, want 720", sched.ScanIntervalM)
	}
	if sched.StartHour != 6 || sched.StartMinute != 0 {
		t.Errorf("seed default start: got %d:%d, want 6:00", sched.StartHour, sched.StartMinute)
	}
	if sched.StartDay != -1 {
		t.Errorf("seed default day: got %d, want -1", sched.StartDay)
	}
}

func TestAdvanceNextScan(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	now := time.Date(2026, 3, 13, 10, 0, 0, 0, time.UTC)

	db.SaveSchedule(ctx, "powder", 720, 6, 0, -1, now)
	before, _ := db.GetSchedule(ctx, "powder")

	later := time.Date(2026, 3, 13, 20, 0, 0, 0, time.UTC)
	err := db.AdvanceNextScan(ctx, "powder", later)
	if err != nil {
		t.Fatal(err)
	}

	after, _ := db.GetSchedule(ctx, "powder")
	if !after.NextScanAt.After(before.NextScanAt) {
		t.Errorf("advance didn't move forward: before=%v after=%v", before.NextScanAt, after.NextScanAt)
	}
}

func TestGetAllSchedules(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	now := time.Date(2026, 3, 13, 10, 0, 0, 0, time.UTC)

	db.SaveSchedule(ctx, "powder", 720, 6, 0, -1, now)
	db.SaveSchedule(ctx, "comedy", 10080, 9, 0, 1, now)

	all, err := db.GetAllSchedules(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 2 {
		t.Errorf("got %d schedules, want 2", len(all))
	}
}
