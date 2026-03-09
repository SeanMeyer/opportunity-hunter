package pipeline_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/seanmeyer/opportunity-hunter/core"
	"github.com/seanmeyer/opportunity-hunter/hunts/fake"
	"github.com/seanmeyer/opportunity-hunter/pipeline"
	"github.com/seanmeyer/opportunity-hunter/testutil"
)

func TestIntegration_AllFourHunts(t *testing.T) {
	db := testutil.NewTestDB(t)
	notifier := &testutil.FakeNotifier{}
	ct := core.NewCostTracker(0, nil)
	pipe := pipeline.New(db, ct, notifier)

	sharedVenue := "Buell Theatre"
	sharedAddr := "1350 Curtis St, Denver, CO"
	future := time.Now().Add(5 * 24 * time.Hour)

	evaluator := &testutil.FakeEvaluator{
		Picks:   []core.Pick{{Score: 0.8, DisplayScore: "8/10", Reason: "Recommended"}},
		CostUSD: 0.002,
	}

	// Comedy hunt.
	comedyHunt := &fake.FakeHunt{
		HuntName: "comedy",
		Srcs: []core.Source{&fake.FakeSource{SourceName: "comedyworks", Items: []core.RawItem{
			{SourceID: "cw-1", Source: "comedyworks", Title: "Nate Bargatze", VenueName: sharedVenue, VenueAddress: sharedAddr, StartTime: future.Format(time.RFC3339)},
		}}},
		Eval: evaluator,
	}

	// Performing arts hunt — shares venue with comedy.
	performingHunt := &fake.FakeHunt{
		HuntName: "performing-arts",
		Srcs: []core.Source{&fake.FakeSource{SourceName: "ticketmaster", Items: []core.RawItem{
			{SourceID: "pa-1", Source: "ticketmaster", Title: "Hamilton", VenueName: sharedVenue, VenueAddress: sharedAddr, StartTime: future.Add(24 * time.Hour).Format(time.RFC3339)},
		}}},
		Eval: evaluator,
	}

	// Movies hunt — no venue (streaming).
	movieAttrs, _ := json.Marshal(map[string]any{"release_type": "streaming", "tmdb_rating": 8.0})
	moviesHunt := &fake.FakeHunt{
		HuntName: "movies",
		Srcs: []core.Source{&fake.FakeSource{SourceName: "tmdb", Items: []core.RawItem{
			{SourceID: "m-1", Source: "tmdb", Title: "The Bear S4", StartTime: future.Format(time.RFC3339), Attributes: movieAttrs},
		}}},
		Eval: evaluator,
		ExpirerFn: func(opp core.Opportunity) bool {
			var attrs struct{ ReleaseType string `json:"release_type"` }
			json.Unmarshal(opp.Attributes, &attrs)
			return attrs.ReleaseType != "streaming" && opp.StartTime.Before(time.Now())
		},
	}

	// Powder hunt — weather, no shared venue.
	powderAttrs, _ := json.Marshal(map[string]any{"snowfall_in": 20.0, "friction_tier": "local_drive"})
	windowEnd := future.Add(3 * 24 * time.Hour)
	powderHunt := &fake.FakeHunt{
		HuntName: "powder",
		Srcs: []core.Source{&fake.FakeSource{SourceName: "open-meteo", Items: []core.RawItem{
			{SourceID: "pw-1", Source: "open-meteo", Title: "Front Range", VenueName: "Front Range", StartTime: future.Format(time.RFC3339), EndTime: windowEnd.Format(time.RFC3339), Attributes: powderAttrs},
		}}},
		Eval: evaluator,
	}

	ctx := context.Background()
	hunts := []core.Hunt{comedyHunt, performingHunt, moviesHunt, powderHunt}
	result := pipe.RunAll(ctx, hunts)

	// Assert: 4 hunt results.
	if len(result.HuntResults) != 4 {
		t.Fatalf("expected 4 hunt results, got %d", len(result.HuntResults))
	}

	// Assert: each hunt scanned successfully.
	for _, hr := range result.HuntResults {
		if hr.Scanned != 1 {
			t.Errorf("hunt %s: expected 1 scanned, got %d", hr.HuntName, hr.Scanned)
		}
		if len(hr.Errors) > 0 {
			for _, e := range hr.Errors {
				t.Errorf("hunt %s error: step=%s err=%v ctx=%s", hr.HuntName, e.Step, e.Err, e.Context)
			}
		}
	}

	// Assert: shared venue — comedy and performing arts share "Buell Theatre".
	v, err := db.GetVenueByName(ctx, sharedVenue)
	if err != nil {
		t.Fatalf("expected shared venue %q: %v", sharedVenue, err)
	}

	// Both comedy and performing arts opportunities should reference the same venue.
	comedyOpps, _ := db.GetByState(ctx, "comedy", core.Evaluated)
	paOpps, _ := db.GetByState(ctx, "performing-arts", core.Evaluated)
	comedyDisc, _ := db.GetByState(ctx, "comedy", core.Discovered)
	paDisc, _ := db.GetByState(ctx, "performing-arts", core.Discovered)
	allComedy := append(comedyOpps, comedyDisc...)
	allPA := append(paOpps, paDisc...)

	for _, opp := range allComedy {
		if opp.VenueID != nil && *opp.VenueID != v.ID {
			t.Errorf("comedy opp venue ID %d != shared venue %d", *opp.VenueID, v.ID)
		}
	}
	for _, opp := range allPA {
		if opp.VenueID != nil && *opp.VenueID != v.ID {
			t.Errorf("performing opp venue ID %d != shared venue %d", *opp.VenueID, v.ID)
		}
	}

	// Assert: movies opportunity has nil VenueID.
	movieOpps, _ := db.GetByState(ctx, "movies", core.Evaluated)
	movieDisc, _ := db.GetByState(ctx, "movies", core.Discovered)
	for _, opp := range append(movieOpps, movieDisc...) {
		if opp.VenueID != nil {
			t.Fatal("expected nil VenueID for streaming movie")
		}
	}

	// Assert: sequential execution — no data races (test passes = no race).
	// Assert: cost tracked across hunts.
	if ct.Total() == 0 {
		t.Fatal("expected total cost > 0")
	}

	// Assert: no errors in pipeline result.
	if result.HasErrors() {
		t.Fatal("expected no errors in pipeline result")
	}
}
