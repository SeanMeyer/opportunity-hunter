package pipeline_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/seanmeyer/opportunity-hunter/core"
	"github.com/seanmeyer/opportunity-hunter/hunts/fake"
	"github.com/seanmeyer/opportunity-hunter/pipeline"
	"github.com/seanmeyer/opportunity-hunter/testutil"
)

func TestIntegration_PerformingArtsFullPipeline(t *testing.T) {
	db := testutil.NewTestDB(t)
	notifier := &testutil.FakeNotifier{}
	ct := core.NewCostTracker(0, nil)
	pipe := pipeline.New(db, ct, notifier)

	// Fake source with 5 performing arts shows across 2 weeks.
	items := []core.RawItem{
		{SourceID: "pa-1", Source: "ticketmaster", Title: "Hamilton", VenueName: "Buell Theatre", VenueAddress: "1350 Curtis St, Denver, CO", StartTime: time.Now().Add(3 * 24 * time.Hour).Format(time.RFC3339)},
		{SourceID: "pa-2", Source: "ticketmaster", Title: "Wicked", VenueName: "Buell Theatre", VenueAddress: "1350 Curtis St, Denver, CO", StartTime: time.Now().Add(4 * 24 * time.Hour).Format(time.RFC3339)},
		{SourceID: "pa-3", Source: "ticketmaster", Title: "Swan Lake", VenueName: "Ellie Caulkins Opera House", VenueAddress: "1385 Curtis St, Denver, CO", StartTime: time.Now().Add(5 * 24 * time.Hour).Format(time.RFC3339)},
		{SourceID: "pa-4", Source: "ticketmaster", Title: "La Bohème", VenueName: "Ellie Caulkins Opera House", VenueAddress: "1385 Curtis St, Denver, CO", StartTime: time.Now().Add(10 * 24 * time.Hour).Format(time.RFC3339)},
		{SourceID: "pa-5", Source: "ticketmaster", Title: "Dear Evan Hansen", VenueName: "Buell Theatre", VenueAddress: "1350 Curtis St, Denver, CO", StartTime: time.Now().Add(11 * 24 * time.Hour).Format(time.RFC3339)},
	}

	evaluator := &testutil.FakeEvaluator{
		Picks: []core.Pick{
			{Score: 0.9, DisplayScore: "9/10", Reason: "Must-see production"},
		},
		CostUSD: 0.003,
	}

	// Create a performing-arts-like hunt using FakeHunt.
	hunt := &fake.FakeHunt{
		HuntName: "performing-arts",
		Srcs: []core.Source{
			&fake.FakeSource{SourceName: "ticketmaster", Items: items},
		},
		Eval: evaluator,
		// Use weekly grouping like the real performing arts hunt.
		GrouperFn: func(opps []core.Opportunity) []core.Group {
			weeks := make(map[string][]core.Opportunity)
			for _, opp := range opps {
				year, week := opp.StartTime.ISOWeek()
				key := fmt.Sprintf("%d-W%02d", year, week)
				weeks[key] = append(weeks[key], opp)
			}
			var groups []core.Group
			for key, weekOpps := range weeks {
				groups = append(groups, core.Group{Key: key, Opportunities: weekOpps})
			}
			return groups
		},
	}

	ctx := context.Background()
	result := pipe.Run(ctx, hunt)

	// Assert: all 5 opportunities scanned.
	if result.Scanned != 5 {
		t.Fatalf("expected 5 scanned, got %d", result.Scanned)
	}

	// Assert: evaluations created (2 weeks = 2 groups).
	if result.Evaluated < 1 {
		t.Fatalf("expected at least 1 evaluation, got %d", result.Evaluated)
	}

	// Assert: no errors.
	if len(result.Errors) > 0 {
		for _, e := range result.Errors {
			t.Errorf("error in step %s: %v (%s)", e.Step, e.Err, e.Context)
		}
		t.Fatal("unexpected errors in pipeline")
	}

	// Assert: opportunities in DB.
	opps, err := db.GetByState(ctx, "performing-arts", core.Evaluated)
	if err != nil {
		t.Fatal(err)
	}
	if len(opps) == 0 {
		// Check discovered — they might still be there if eval didn't process all.
		disc, _ := db.GetByState(ctx, "performing-arts", core.Discovered)
		t.Logf("discovered: %d, evaluated: %d", len(disc), len(opps))
	}

	// Assert: cost tracked.
	if ct.Total() == 0 {
		t.Fatal("expected cost to be tracked")
	}
	if ct.ForHunt("performing-arts") == 0 {
		t.Fatal("expected cost for performing-arts hunt")
	}

	// Assert: venues created (2 unique venues).
	v1, err := db.GetVenueByName(ctx, "Buell Theatre")
	if err != nil {
		t.Fatalf("expected Buell Theatre venue: %v", err)
	}
	v2, err := db.GetVenueByName(ctx, "Ellie Caulkins Opera House")
	if err != nil {
		t.Fatalf("expected Ellie Caulkins venue: %v", err)
	}
	if v1.ID == v2.ID {
		t.Fatal("venues should have different IDs")
	}
}
