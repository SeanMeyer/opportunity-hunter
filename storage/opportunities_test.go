package storage_test

import (
	"context"
	"testing"
	"time"

	"github.com/seanmeyer/opportunity-hunter/core"
	"github.com/seanmeyer/opportunity-hunter/storage"
)

func insertTestOpportunity(t *testing.T, db *storage.DB, huntName, title string) int64 {
	t.Helper()
	ctx := context.Background()
	id, err := db.InsertOpportunity(ctx, core.Opportunity{
		HuntName:     huntName,
		SourceID:     "src-" + title,
		Source:       "test",
		Title:        title,
		State:        core.Discovered,
		StartTime:    time.Now().Add(24 * time.Hour),
		DiscoveredAt: time.Now(),
	})
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func TestInsertAndGetOpportunity(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	id := insertTestOpportunity(t, db, "comedy", "Nate Bargatze")

	opp, err := db.GetOpportunity(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if opp.Title != "Nate Bargatze" {
		t.Fatalf("expected 'Nate Bargatze', got %q", opp.Title)
	}
	if opp.HuntName != "comedy" {
		t.Fatalf("expected 'comedy', got %q", opp.HuntName)
	}
	if opp.State != core.Discovered {
		t.Fatalf("expected Discovered, got %s", opp.State)
	}
}

func TestGetByState(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	insertTestOpportunity(t, db, "comedy", "Show A")
	insertTestOpportunity(t, db, "comedy", "Show B")
	insertTestOpportunity(t, db, "performing-arts", "Ballet")

	opps, err := db.GetByState(ctx, "comedy", core.Discovered)
	if err != nil {
		t.Fatal(err)
	}
	if len(opps) != 2 {
		t.Fatalf("expected 2 comedy discovered, got %d", len(opps))
	}

	// performing-arts should be separate.
	opps, err = db.GetByState(ctx, "performing-arts", core.Discovered)
	if err != nil {
		t.Fatal(err)
	}
	if len(opps) != 1 {
		t.Fatalf("expected 1 performing-arts discovered, got %d", len(opps))
	}
}

func TestOpportunityExists(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	insertTestOpportunity(t, db, "comedy", "Show A")

	exists, err := db.OpportunityExists(ctx, "comedy", "src-Show A")
	if err != nil {
		t.Fatal(err)
	}
	if !exists {
		t.Fatal("expected opportunity to exist")
	}

	exists, err = db.OpportunityExists(ctx, "comedy", "src-nonexistent")
	if err != nil {
		t.Fatal(err)
	}
	if exists {
		t.Fatal("expected opportunity NOT to exist")
	}
}

func TestUpdateState(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	id := insertTestOpportunity(t, db, "comedy", "Show A")
	now := time.Now()

	err := db.UpdateState(ctx, id, core.Evaluated, &now)
	if err != nil {
		t.Fatal(err)
	}

	opp, _ := db.GetOpportunity(ctx, id)
	if opp.State != core.Evaluated {
		t.Fatalf("expected Evaluated, got %s", opp.State)
	}
}

func TestGetUpcoming(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	// Insert one upcoming and one past opportunity.
	futureID := insertTestOpportunity(t, db, "comedy", "Future Show")
	db.UpdateState(ctx, futureID, core.Notified, timePtr(time.Now()))

	pastID, _ := db.InsertOpportunity(ctx, core.Opportunity{
		HuntName:     "comedy",
		SourceID:     "src-past",
		Source:       "test",
		Title:        "Past Show",
		State:        core.Notified,
		StartTime:    time.Now().Add(-24 * time.Hour),
		DiscoveredAt: time.Now(),
	})
	db.UpdateState(ctx, pastID, core.Notified, timePtr(time.Now()))

	opps, err := db.GetUpcoming(ctx, "comedy", 48*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	// Only the future show should be returned.
	if len(opps) != 1 {
		t.Fatalf("expected 1 upcoming, got %d", len(opps))
	}
	if opps[0].Title != "Future Show" {
		t.Fatalf("expected 'Future Show', got %q", opps[0].Title)
	}
}

func timePtr(t time.Time) *time.Time { return &t }
