package storage_test

import (
	"context"
	"log/slog"
	"testing"

	"github.com/seanmeyer/opportunity-hunter/storage"
	"github.com/seanmeyer/opportunity-hunter/testutil"
)

func TestDBLogHandler(t *testing.T) {
	db := testutil.NewTestDB(t)
	ctx := context.Background()

	handler := storage.NewDBLogHandler(db, slog.LevelInfo)
	defer handler.Stop()
	logger := slog.New(handler)

	logger.InfoContext(ctx, "scan complete", "hunt", "comedy")
	logger.WarnContext(ctx, "rate limit hit", "hunt", "movies")
	logger.InfoContext(ctx, "daemon started")

	handler.Flush()

	logs, err := db.RecentLogs(ctx, "", "", 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(logs) != 3 {
		t.Fatalf("expected 3 logs, got %d", len(logs))
	}

	comedyLogs, _ := db.RecentLogs(ctx, "comedy", "", 100)
	if len(comedyLogs) != 1 {
		t.Fatalf("expected 1 comedy log, got %d", len(comedyLogs))
	}
}

func TestDBLogHandlerRunID(t *testing.T) {
	db := testutil.NewTestDB(t)

	handler := storage.NewDBLogHandler(db, slog.LevelInfo)
	defer handler.Stop()
	logger := slog.New(handler)

	ctx := storage.ContextWithRunID(context.Background(), "run-abc")
	logger.InfoContext(ctx, "pipeline step", "hunt", "powder")

	handler.Flush()

	logs, _ := db.RecentLogs(context.Background(), "powder", "", 100)
	if len(logs) != 1 {
		t.Fatalf("expected 1 log, got %d", len(logs))
	}
	if logs[0].RunID != "run-abc" {
		t.Errorf("expected run_id run-abc, got %q", logs[0].RunID)
	}
}

func TestTriggerContext(t *testing.T) {
	ctx := context.Background()
	if got := storage.TriggerFromContext(ctx); got != "scheduled" {
		t.Errorf("default trigger should be scheduled, got %q", got)
	}

	ctx = storage.ContextWithTrigger(ctx, "manual")
	if got := storage.TriggerFromContext(ctx); got != "manual" {
		t.Errorf("expected manual, got %q", got)
	}
}
