package storage_test

import (
	"path/filepath"
	"testing"

	"github.com/seanmeyer/opportunity-hunter/storage"
)

func newTestDB(t *testing.T) *storage.DB {
	t.Helper()
	db, err := storage.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestOpen_CreatesTablesSuccessfully(t *testing.T) {
	db := newTestDB(t)

	tables := []string{
		"venues", "opportunities", "evaluations", "picks",
		"feedback", "preferences", "distance_cache",
		"notification_threads", "eval_costs",
	}
	for _, table := range tables {
		var count int
		err := db.RawDB().QueryRow(
			"SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?", table,
		).Scan(&count)
		if err != nil {
			t.Fatalf("query table %s: %v", table, err)
		}
		if count != 1 {
			t.Fatalf("table %s not found", table)
		}
	}
}

func TestOpen_Idempotent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.db")

	db1, err := storage.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	db1.Close()

	db2, err := storage.Open(path)
	if err != nil {
		t.Fatalf("second open failed: %v", err)
	}
	db2.Close()
}

func TestOpen_JSONValidConstraint(t *testing.T) {
	db := newTestDB(t)

	// Valid JSON should work.
	_, err := db.RawDB().Exec(
		`INSERT INTO opportunities (hunt_name, source_id, source, title, start_time, attributes, discovered_at)
		 VALUES ('test', 'src1', 'test', 'Show', '2026-01-01T00:00:00Z', '{"key":"val"}', '2026-01-01T00:00:00Z')`,
	)
	if err != nil {
		t.Fatalf("valid JSON insert failed: %v", err)
	}

	// Invalid JSON should fail.
	_, err = db.RawDB().Exec(
		`INSERT INTO opportunities (hunt_name, source_id, source, title, start_time, attributes, discovered_at)
		 VALUES ('test', 'src2', 'test', 'Show2', '2026-01-01T00:00:00Z', 'not json', '2026-01-01T00:00:00Z')`,
	)
	if err == nil {
		t.Fatal("expected error for invalid JSON in attributes")
	}
}

func TestOpen_ForeignKeys(t *testing.T) {
	db := newTestDB(t)

	// picks references evaluations — inserting a pick with nonexistent evaluation_id should fail.
	_, err := db.RawDB().Exec(
		`INSERT INTO picks (evaluation_id, opportunity_id, score) VALUES (999, 999, 0.5)`,
	)
	if err == nil {
		t.Fatal("expected foreign key error for nonexistent evaluation_id")
	}
}
