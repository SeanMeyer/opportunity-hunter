package storage_test

import (
	"context"
	"testing"
	"time"

	"github.com/seanmeyer/opportunity-hunter/storage"
)

func TestSaveAndGetFeedback(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	oppID := insertTestOpportunity(t, db, "comedy", "Nate Bargatze")

	_, err := db.SaveFeedback(ctx, storage.FeedbackRow{
		OpportunityID: &oppID,
		HuntName:      "comedy",
		Title:         "Nate Bargatze",
		Rating:        "loved",
		Note:          "Great show",
		CreatedAt:     time.Now(),
	})
	if err != nil {
		t.Fatal(err)
	}

	fb, err := db.GetRecentFeedback(ctx, "comedy", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(fb) != 1 {
		t.Fatalf("expected 1 feedback, got %d", len(fb))
	}
	if fb[0].Rating != "loved" {
		t.Fatalf("expected 'loved', got %q", fb[0].Rating)
	}
}

func TestSaveFeedback_NullOpportunityID(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	_, err := db.SaveFeedback(ctx, storage.FeedbackRow{
		HuntName:  "movies",
		Title:     "The Godfather",
		Rating:    "loved",
		CreatedAt: time.Now(),
	})
	if err != nil {
		t.Fatal(err)
	}

	fb, err := db.GetRecentFeedback(ctx, "movies", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(fb) != 1 {
		t.Fatalf("expected 1 feedback, got %d", len(fb))
	}
	if fb[0].OpportunityID != nil {
		t.Fatal("expected nil OpportunityID for manual feedback")
	}
}
