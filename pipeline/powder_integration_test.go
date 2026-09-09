package pipeline_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/seanmeyer/opportunity-hunter/core"
	"github.com/seanmeyer/opportunity-hunter/hunts/fake"
	"github.com/seanmeyer/opportunity-hunter/pipeline"
	"github.com/seanmeyer/opportunity-hunter/testutil"
)

func powderAttrs(snowfallIn float64, stormGroup, frictionTier string) core.Attributes {
	b, _ := json.Marshal(map[string]any{
		"snowfall_in":   snowfallIn,
		"friction_tier": frictionTier,
		"storm_group":   stormGroup,
	})
	return b
}

func TestIntegration_PowderFullPipeline(t *testing.T) {
	db := testutil.NewTestDB(t)
	notifier := &testutil.FakeNotifier{}
	ct := core.NewCostTracker(0, nil)
	pipe := pipeline.New(db, ct, notifier, core.ScanRegion{}, "")

	window := time.Now().Add(3 * 24 * time.Hour)
	windowEnd := window.Add(3 * 24 * time.Hour)

	items := []core.RawItem{
		{SourceID: "p-1", Source: "open-meteo", Title: "Front Range", Subtitle: "Mar 12-15", VenueName: "Front Range", StartTime: window.Format(time.RFC3339), EndTime: windowEnd.Format(time.RFC3339), Attributes: powderAttrs(14, "co_front_range", "local_drive")},
		{SourceID: "p-2", Source: "open-meteo", Title: "Summit County", Subtitle: "Mar 12-15", VenueName: "Summit County", StartTime: window.Format(time.RFC3339), EndTime: windowEnd.Format(time.RFC3339), Attributes: powderAttrs(18, "co_central", "local_drive")},
		{SourceID: "p-3", Source: "open-meteo", Title: "PNW Cascades", Subtitle: "Mar 12-15", VenueName: "PNW Cascades", StartTime: window.Format(time.RFC3339), EndTime: windowEnd.Format(time.RFC3339), Attributes: powderAttrs(30, "pnw_cascades", "flight")},
	}

	evaluator := &testutil.FakeEvaluator{
		Picks:   []core.Pick{{Score: 0.9, DisplayScore: "DROP EVERYTHING", Reason: "Epic storm"}},
		CostUSD: 0.005,
	}

	hunt := &fake.FakeHunt{
		HuntName: "powder",
		Srcs:     []core.Source{&fake.FakeSource{SourceName: "open-meteo", Items: items}},
		Eval:     evaluator,
	}

	ctx := context.Background()
	result := pipe.Run(ctx, hunt)

	if result.Scanned != 3 {
		t.Fatalf("expected 3 scanned, got %d", result.Scanned)
	}
	if result.Evaluated < 1 {
		t.Fatalf("expected at least 1 evaluation, got %d", result.Evaluated)
	}
	if len(result.Errors) > 0 {
		for _, e := range result.Errors {
			t.Errorf("step %s: %v (%s)", e.Step, e.Err, e.Context)
		}
		t.Fatal("unexpected errors")
	}

	// Cost should be tracked.
	if ct.ForHunt("powder") == 0 {
		t.Fatal("expected cost for powder hunt")
	}
}

func TestIntegration_PowderReEvaluation(t *testing.T) {
	db := testutil.NewTestDB(t)
	notifier := &testutil.FakeNotifier{}
	ct := core.NewCostTracker(0, nil)
	pipe := pipeline.New(db, ct, notifier, core.ScanRegion{}, "")

	window := time.Now().Add(3 * 24 * time.Hour)
	windowEnd := window.Add(3 * 24 * time.Hour)

	items := []core.RawItem{
		{SourceID: "p-1", Source: "open-meteo", Title: "Front Range", VenueName: "Front Range", StartTime: window.Format(time.RFC3339), EndTime: windowEnd.Format(time.RFC3339), Attributes: powderAttrs(14, "co_front_range", "local_drive")},
	}

	evaluator := &testutil.FakeEvaluator{
		Picks:   []core.Pick{{Score: 0.8, DisplayScore: "WORTH A LOOK"}},
		CostUSD: 0.003,
	}

	hunt := &fake.FakeHunt{
		HuntName: "powder",
		Srcs:     []core.Source{&fake.FakeSource{SourceName: "open-meteo", Items: items}},
		Eval:     evaluator,
		ReEvalFn: func(opp core.Opportunity, lastEval *core.Evaluation) bool {
			// Simulate: always re-evaluate for testing.
			return lastEval != nil
		},
	}

	ctx := context.Background()

	// First run: scan + evaluate.
	result1 := pipe.Run(ctx, hunt)
	if result1.Scanned != 1 {
		t.Fatalf("run 1: expected 1 scanned, got %d", result1.Scanned)
	}

	// Second run: should re-evaluate (no new items to scan, but re-eval kicks in).
	// Source returns same items which get deduped, but re-eval checks existing evaluated opps.
	result2 := pipe.Run(ctx, hunt)
	if len(evaluator.Calls) < 2 || evaluator.Calls[len(evaluator.Calls)-1].PriorEval == nil {
		t.Fatal("re-evaluation lost prior judgment")
	}
	// Second run should find 0 new items (deduped) but the re-evaluator should pick up existing ones.
	if result2.Evaluated < 1 {
		t.Logf("run 2: scanned=%d, evaluated=%d (re-eval may not trigger if no state change)", result2.Scanned, result2.Evaluated)
	}

	// Total cost should reflect both runs.
	if ct.ForHunt("powder") < 0.003 {
		t.Fatalf("expected cost >= 0.003, got %f", ct.ForHunt("powder"))
	}
}

func TestHistoryStaysWithOpportunityAcrossRegionWindows(t *testing.T) {
	db := testutil.NewTestDB(t)
	ctx := context.Background()
	var aID int64
	for i, tier := range []string{"RECOMMENDED", "WATCH"} {
		id, err := db.InsertOpportunity(ctx, core.Opportunity{HuntName: "powder", SourceID: fmt.Sprintf("window-%d", i), Title: "Front Range", State: core.Evaluated, StartTime: time.Now().Add(time.Duration(i+1) * 24 * time.Hour), DiscoveredAt: time.Now()})
		if err != nil {
			t.Fatal(err)
		}
		if i == 0 {
			aID = id
		}
		_, err = db.SaveEvaluationWithPicks(ctx, core.Evaluation{HuntName: "powder", GroupKey: "Front Range", EvaluatedAt: time.Now().Add(time.Duration(i-2) * time.Hour), StructuredResponse: `{"tier":"` + tier + `"}`}, []core.Pick{{OpportunityID: id, DisplayScore: tier}})
		if err != nil {
			t.Fatal(err)
		}
	}
	evaluator := &testutil.FakeEvaluator{}
	hunt := &fake.FakeHunt{HuntName: "powder", Eval: evaluator, ReEvalFn: func(opp core.Opportunity, prior *core.Evaluation) bool {
		if opp.ID != aID {
			return false
		}
		if prior == nil || prior.StructuredResponse != `{"tier":"RECOMMENDED"}` {
			t.Errorf("gate received another storm's history: %+v", prior)
		}
		return true
	}}
	pipe := pipeline.New(db, core.NewCostTracker(0, nil), &testutil.FakeNotifier{}, core.ScanRegion{}, "")
	pipe.Run(ctx, hunt)
	if len(evaluator.Calls) != 1 || evaluator.Calls[0].PriorEval == nil || len(evaluator.Calls[0].PriorPicks) != 1 || evaluator.Calls[0].PriorPicks[0].DisplayScore != "RECOMMENDED" {
		t.Fatalf("lost matching history: %+v", evaluator.Calls)
	}
}
