package pipeline_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/seanmeyer/opportunity-hunter/core"
	"github.com/seanmeyer/opportunity-hunter/hunts/fake"
	"github.com/seanmeyer/opportunity-hunter/pipeline"
	"github.com/seanmeyer/opportunity-hunter/storage"
	"github.com/seanmeyer/opportunity-hunter/testutil"
)

func movieAttrs(releaseType string) core.Attributes {
	b, _ := json.Marshal(map[string]any{
		"release_type": releaseType,
		"tmdb_rating":  7.5,
		"genre":        []string{"Drama"},
	})
	return b
}

func TestIntegration_MoviesFullPipeline(t *testing.T) {
	db := testutil.NewTestDB(t)
	notifier := &testutil.FakeNotifier{}
	ct := core.NewCostTracker(0, nil)
	pipe := pipeline.New(db, ct, notifier)

	items := []core.RawItem{
		{SourceID: "m-1", Source: "tmdb", Title: "Dune: Part Three", StartTime: time.Now().Add(7 * 24 * time.Hour).Format(time.RFC3339), Attributes: movieAttrs("theatrical")},
		{SourceID: "m-2", Source: "tmdb", Title: "The Bear S4", StartTime: time.Now().Add(14 * 24 * time.Hour).Format(time.RFC3339), Attributes: movieAttrs("streaming")},
		{SourceID: "m-3", Source: "tmdb", Title: "Nosferatu", StartTime: time.Now().Add(3 * 24 * time.Hour).Format(time.RFC3339), Attributes: movieAttrs("theatrical")},
	}

	evaluator := &testutil.FakeEvaluator{
		Picks:   []core.Pick{{Score: 0.8, DisplayScore: "8/10", Reason: "Great movie"}},
		CostUSD: 0.002,
	}

	// Movies hunt: no Grouper, has Expirer.
	hunt := &fake.FakeHunt{
		HuntName: "movies",
		Srcs:     []core.Source{&fake.FakeSource{SourceName: "tmdb", Items: items}},
		Eval:     evaluator,
		ExpirerFn: func(opp core.Opportunity) bool {
			var attrs struct {
				ReleaseType string `json:"release_type"`
			}
			json.Unmarshal(opp.Attributes, &attrs)
			switch attrs.ReleaseType {
			case "theatrical":
				return time.Since(opp.StartTime) > 8*7*24*time.Hour
			case "streaming":
				return false
			default:
				return opp.StartTime.Before(time.Now())
			}
		},
	}

	ctx := context.Background()
	result := pipe.Run(ctx, hunt)

	if result.Scanned != 3 {
		t.Fatalf("expected 3 scanned, got %d", result.Scanned)
	}
	if len(result.Errors) > 0 {
		t.Fatalf("unexpected errors: %v", result.Errors)
	}

	// All opportunities should have nil VenueID (no venue for TMDB movies).
	opps, _ := db.GetByState(ctx, "movies", core.Evaluated)
	disc, _ := db.GetByState(ctx, "movies", core.Discovered)
	allOpps := append(opps, disc...)
	for _, opp := range allOpps {
		if opp.VenueID != nil {
			t.Fatalf("expected nil VenueID for movie %q", opp.Title)
		}
	}
}

func TestIntegration_MoviesExpiration(t *testing.T) {
	db := testutil.NewTestDB(t)
	notifier := &testutil.FakeNotifier{}
	ct := core.NewCostTracker(0, nil)
	pipe := pipeline.New(db, ct, notifier)
	ctx := context.Background()

	// Manually insert opportunities at various ages.
	insertOpp := func(title string, startTime time.Time, attrs core.Attributes) {
		db.InsertOpportunity(ctx, core.Opportunity{
			HuntName:     "movies",
			SourceID:     "test-" + title,
			Source:       "tmdb",
			Title:        title,
			State:        core.Notified,
			StartTime:    startTime,
			Attributes:   attrs,
			DiscoveredAt: time.Now(),
		})
		now := time.Now()
		// Get the ID and update state to notified.
		opps, _ := db.GetByState(ctx, "movies", core.Discovered)
		for _, opp := range opps {
			if opp.Title == title {
				db.UpdateState(ctx, opp.ID, core.Notified, &now)
			}
		}
	}

	insertOpp("Old Theatrical", time.Now().Add(-9*7*24*time.Hour), movieAttrs("theatrical"))
	insertOpp("Recent Theatrical", time.Now().Add(-2*7*24*time.Hour), movieAttrs("theatrical"))
	insertOpp("Old Streaming", time.Now().Add(-180*24*time.Hour), movieAttrs("streaming"))

	// Run pipeline with expirer.
	hunt := &fake.FakeHunt{
		HuntName: "movies",
		Srcs:     []core.Source{&fake.FakeSource{SourceName: "tmdb"}}, // no new items
		Eval:     &testutil.FakeEvaluator{},
		ExpirerFn: func(opp core.Opportunity) bool {
			var attrs struct {
				ReleaseType string `json:"release_type"`
			}
			json.Unmarshal(opp.Attributes, &attrs)
			switch attrs.ReleaseType {
			case "theatrical":
				return time.Since(opp.StartTime) > 8*7*24*time.Hour
			case "streaming":
				return false
			default:
				return opp.StartTime.Before(time.Now())
			}
		},
	}

	pipe.Run(ctx, hunt)

	// Old theatrical should be expired.
	expired, _ := db.GetByState(ctx, "movies", core.Expired)
	expiredTitles := map[string]bool{}
	for _, opp := range expired {
		expiredTitles[opp.Title] = true
	}

	if !expiredTitles["Old Theatrical"] {
		t.Fatal("expected 'Old Theatrical' to be expired")
	}

	// Recent theatrical should NOT be expired.
	notified, _ := db.GetByState(ctx, "movies", core.Notified)
	notifiedTitles := map[string]bool{}
	for _, opp := range notified {
		notifiedTitles[opp.Title] = true
	}
	if !notifiedTitles["Recent Theatrical"] {
		t.Fatal("expected 'Recent Theatrical' to still be notified")
	}
	if !notifiedTitles["Old Streaming"] {
		t.Fatal("expected 'Old Streaming' to still be notified (streaming never expires)")
	}
}

func TestIntegration_MoviesManualFeedback(t *testing.T) {
	db := testutil.NewTestDB(t)
	ctx := context.Background()

	// Save manual feedback (no opportunity_id).
	_, err := db.SaveFeedback(ctx, storage.FeedbackRow{
		HuntName:  "movies",
		Title:     "The Godfather",
		Rating:    "loved",
		Note:      "Classic masterpiece",
		CreatedAt: time.Now(),
	})
	if err != nil {
		t.Fatal(err)
	}

	// Retrieve and verify.
	fb, _ := db.GetRecentFeedback(ctx, "movies", 10)
	if len(fb) != 1 {
		t.Fatalf("expected 1 feedback, got %d", len(fb))
	}
	if fb[0].OpportunityID != nil {
		t.Fatal("expected nil OpportunityID for manual feedback")
	}
	if fb[0].Title != "The Godfather" {
		t.Fatalf("expected 'The Godfather', got %q", fb[0].Title)
	}
}
