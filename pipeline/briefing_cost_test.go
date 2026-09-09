package pipeline_test

import (
	"context"
	"github.com/seanmeyer/opportunity-hunter/core"
	"github.com/seanmeyer/opportunity-hunter/hunts/fake"
	"github.com/seanmeyer/opportunity-hunter/pipeline"
	"github.com/seanmeyer/opportunity-hunter/testutil"
	"testing"
	"time"
)

func TestBriefingSpendPersists(t *testing.T) {
	db := testutil.NewTestDB(t)
	ct := core.NewCostTracker(0, nil)
	h := &fake.FakeHunt{HuntName: "powder", Srcs: []core.Source{&fake.FakeSource{SourceName: "fake", Items: makeRawItems(1)}}, Eval: &testutil.FakeEvaluator{}, BrieferGroupFn: func(e []core.Evaluation) []core.NotifyGroup { return []core.NotifyGroup{{Evaluations: e}} }, SynthesizeFn: func(_ context.Context, _ core.NotifyGroup, ct *core.CostTracker) (string, error) {
		ct.Add("powder", .02)
		return "Briefing", nil
	}}
	pipe := pipeline.New(db, ct, &testutil.FakeNotifier{}, core.ScanRegion{}, "")
	result := pipe.Run(context.Background(), h)
	if len(result.Errors) > 0 {
		t.Fatal(result.Errors)
	}
	spend, err := db.MonthlySpend(context.Background(), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if spend.ByHunt["powder"] != .02 {
		t.Fatalf("lost briefing spend: %v", spend)
	}
}
