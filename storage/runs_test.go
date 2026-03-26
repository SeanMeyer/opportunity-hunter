package storage_test

import (
	"context"
	"testing"

	"github.com/seanmeyer/opportunity-hunter/storage"
	"github.com/seanmeyer/opportunity-hunter/testutil"
)

func TestInsertAndFinishRun(t *testing.T) {
	db := testutil.NewTestDB(t)
	ctx := context.Background()

	runID, err := db.InsertRun(ctx, "comedy", "scheduled")
	if err != nil {
		t.Fatal(err)
	}
	if runID == "" {
		t.Fatal("expected non-empty run ID")
	}

	err = db.FinishRun(ctx, runID, storage.RunResult{
		Status:       "ok",
		Scanned:      4,
		Evaluated:    4,
		Notified:     2,
		CostUSD:      0.04,
		ErrorSummary: "",
	})
	if err != nil {
		t.Fatal(err)
	}

	run, err := db.LatestRun(ctx, "comedy")
	if err != nil {
		t.Fatal(err)
	}
	if run == nil {
		t.Fatal("expected a run")
	}
	if run.Status != "ok" || run.Scanned != 4 || run.Notified != 2 {
		t.Errorf("got %+v", run)
	}
}

func TestRecentRuns(t *testing.T) {
	db := testutil.NewTestDB(t)
	ctx := context.Background()

	db.InsertRun(ctx, "comedy", "scheduled")
	db.InsertRun(ctx, "powder", "scheduled")
	db.InsertRun(ctx, "comedy", "manual")

	all, err := db.RecentRuns(ctx, "", 50)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 3 {
		t.Fatalf("expected 3 runs, got %d", len(all))
	}

	comedyOnly, err := db.RecentRuns(ctx, "comedy", 50)
	if err != nil {
		t.Fatal(err)
	}
	if len(comedyOnly) != 2 {
		t.Fatalf("expected 2 comedy runs, got %d", len(comedyOnly))
	}
}

func TestLatestRunNone(t *testing.T) {
	db := testutil.NewTestDB(t)
	ctx := context.Background()

	run, err := db.LatestRun(ctx, "nonexistent")
	if err != nil {
		t.Fatal(err)
	}
	if run != nil {
		t.Fatal("expected nil for hunt with no runs")
	}
}
