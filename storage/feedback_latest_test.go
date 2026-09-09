package storage_test

import (
	"context"
	"github.com/seanmeyer/opportunity-hunter/core"
	"github.com/seanmeyer/opportunity-hunter/storage"
	"github.com/seanmeyer/opportunity-hunter/testutil"
	"testing"
	"time"
)

func TestRecentFeedbackUsesLatestChoiceBeforeLimit(t *testing.T) {
	db := testutil.NewTestDB(t)
	ctx := context.Background()
	now := time.Now()
	id, err := db.InsertOpportunity(ctx, core.Opportunity{HuntName: "comedy", SourceID: "one", Title: "Show", State: core.Evaluated, DiscoveredAt: now})
	if err != nil {
		t.Fatal(err)
	}
	// Distinct manual observations have no opportunity identity and must survive.
	for _, note := range []string{"Prefer weekends", "Prefer nearby venues"} {
		_, err = db.SaveFeedback(ctx, storage.FeedbackRow{HuntName: "comedy", Title: "General", Rating: "up", Note: note, CreatedAt: now})
		if err != nil {
			t.Fatal(err)
		}
	}
	for i := 0; i < 30; i++ {
		rating := "up"
		if i == 29 {
			rating = "down"
		}
		_, err = db.SaveFeedback(ctx, storage.FeedbackRow{HuntName: "comedy", OpportunityID: &id, Title: "Show", Rating: rating, Note: "Latest note", CreatedAt: now})
		if err != nil {
			t.Fatal(err)
		}
	}
	rows, err := db.GetRecentFeedback(ctx, "comedy", 20)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 3 {
		t.Fatalf("got %d effective feedback rows, want 3", len(rows))
	}
	if rows[0].Rating != "down" || rows[0].Note != "Latest note" {
		t.Fatalf("did not retain latest choice: %+v", rows[0])
	}
}
