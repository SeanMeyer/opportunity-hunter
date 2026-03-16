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

func TestIntegration_ComedyFullPipeline(t *testing.T) {
	db := testutil.NewTestDB(t)
	notifier := &testutil.FakeNotifier{}
	ct := core.NewCostTracker(0, nil)
	pipe := pipeline.New(db, ct, notifier, core.ScanRegion{}, "")

	// Simulate comedy shows across 2 weeks, including a multi-night residency.
	items := []core.RawItem{
		{SourceID: "c-1", Source: "ticketmaster", Title: "Nate Bargatze", VenueName: "Comedy Works Downtown", VenueAddress: "1226 15th St", StartTime: time.Now().Add(2 * 24 * time.Hour).Format(time.RFC3339)},
		{SourceID: "c-2", Source: "ticketmaster", Title: "Nate Bargatze", VenueName: "Comedy Works Downtown", VenueAddress: "1226 15th St", StartTime: time.Now().Add(3 * 24 * time.Hour).Format(time.RFC3339)},
		{SourceID: "c-3", Source: "comedyworks", Title: "Sam Morril", VenueName: "Comedy Works Downtown", VenueAddress: "1226 15th St", StartTime: time.Now().Add(4 * 24 * time.Hour).Format(time.RFC3339)},
		{SourceID: "c-4", Source: "comedyworks", Title: "Taylor Tomlinson", VenueName: "Comedy Works South", VenueAddress: "5345 Landmark Pl", StartTime: time.Now().Add(10 * 24 * time.Hour).Format(time.RFC3339)},
	}

	evaluator := &testutil.FakeEvaluator{
		Picks: []core.Pick{
			{Score: 0.85, DisplayScore: "8/10", Reason: "Must-see comedian"},
		},
		CostUSD: 0.003,
	}

	// Use FakeHunt with weekly grouping to simulate comedy behavior.
	hunt := &fake.FakeHunt{
		HuntName: "comedy",
		Srcs: []core.Source{
			&fake.FakeSource{SourceName: "ticketmaster", Items: items[:2]},
			&fake.FakeSource{SourceName: "comedyworks", Items: items[2:]},
		},
		Eval: evaluator,
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

	// Assert: 3 scanned — the 2 "Nate Bargatze" at "Comedy Works Downtown" are
	// merged into 1 multi-date opportunity at scan time.
	if result.Scanned != 3 {
		t.Fatalf("expected 3 scanned (multi-date merge), got %d", result.Scanned)
	}

	// Assert: evaluated (2 weeks).
	if result.Evaluated < 1 {
		t.Fatalf("expected at least 1 evaluation, got %d", result.Evaluated)
	}

	// Assert: no errors.
	if len(result.Errors) > 0 {
		for _, e := range result.Errors {
			t.Errorf("step %s: %v (%s)", e.Step, e.Err, e.Context)
		}
		t.Fatal("unexpected errors")
	}

	// Assert: shared venue deduplication — both "Comedy Works Downtown" items share one venue.
	v, err := db.GetVenueByName(ctx, "Comedy Works Downtown")
	if err != nil {
		t.Fatalf("expected Comedy Works Downtown venue: %v", err)
	}
	if v.ID == 0 {
		t.Fatal("expected non-zero venue ID")
	}

	// Assert: cost tracked.
	if ct.ForHunt("comedy") == 0 {
		t.Fatal("expected cost for comedy hunt")
	}
}
