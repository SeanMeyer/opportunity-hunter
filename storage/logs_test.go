package storage_test

import (
	"context"
	"testing"
	"time"

	"github.com/seanmeyer/opportunity-hunter/storage"
	"github.com/seanmeyer/opportunity-hunter/testutil"
)

func TestInsertAndQueryLogs(t *testing.T) {
	db := testutil.NewTestDB(t)
	ctx := context.Background()

	logs := []storage.RunLog{
		{RunID: "run-1", HuntName: "comedy", Timestamp: time.Now(), Level: "info", Message: "scan complete", Attrs: "{}"},
		{RunID: "run-1", HuntName: "comedy", Timestamp: time.Now(), Level: "warn", Message: "rate limit", Attrs: "{}"},
		{RunID: "", HuntName: "", Timestamp: time.Now(), Level: "info", Message: "daemon started", Attrs: "{}"},
	}

	if err := db.InsertLogs(ctx, logs); err != nil {
		t.Fatal(err)
	}

	all, err := db.RecentLogs(ctx, "", "", 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 3 {
		t.Fatalf("expected 3, got %d", len(all))
	}

	comedyLogs, err := db.RecentLogs(ctx, "comedy", "", 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(comedyLogs) != 2 {
		t.Fatalf("expected 2 comedy logs, got %d", len(comedyLogs))
	}

	warns, err := db.RecentLogs(ctx, "", "warn", 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(warns) != 1 {
		t.Fatalf("expected 1 warn, got %d", len(warns))
	}
}

func TestPruneLogs(t *testing.T) {
	db := testutil.NewTestDB(t)
	ctx := context.Background()

	old := storage.RunLog{
		Timestamp: time.Now().Add(-8 * 24 * time.Hour),
		Level:     "info", Message: "old log", Attrs: "{}",
	}
	recent := storage.RunLog{
		Timestamp: time.Now(),
		Level:     "info", Message: "recent log", Attrs: "{}",
	}
	db.InsertLogs(ctx, []storage.RunLog{old, recent})

	deleted, err := db.PruneLogs(ctx, 7*24*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if deleted != 1 {
		t.Fatalf("expected 1 deleted, got %d", deleted)
	}

	remaining, _ := db.RecentLogs(ctx, "", "", 100)
	if len(remaining) != 1 {
		t.Fatalf("expected 1 remaining, got %d", len(remaining))
	}
}
