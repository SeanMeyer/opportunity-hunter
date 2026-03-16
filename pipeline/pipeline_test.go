package pipeline_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/seanmeyer/opportunity-hunter/core"
	"github.com/seanmeyer/opportunity-hunter/hunts/fake"
	"github.com/seanmeyer/opportunity-hunter/pipeline"
	"github.com/seanmeyer/opportunity-hunter/testutil"
)

func newTestPipeline(t *testing.T) (*pipeline.Pipeline, *testutil.FakeNotifier) {
	t.Helper()
	db := testutil.NewTestDB(t)
	notifier := &testutil.FakeNotifier{}
	ct := core.NewCostTracker(0, nil)
	p := pipeline.New(db, ct, notifier, core.ScanRegion{}, "")
	return p, notifier
}

func makeRawItems(n int) []core.RawItem {
	items := make([]core.RawItem, n)
	for i := range n {
		items[i] = core.RawItem{
			SourceID:  fmt.Sprintf("src-%d", i),
			Source:    "test",
			Title:     fmt.Sprintf("Show %d", i),
			StartTime: time.Now().Add(time.Duration(i+1) * 24 * time.Hour).Format(time.RFC3339),
		}
	}
	return items
}

func TestPipeline_ScanStoresOpportunities(t *testing.T) {
	p, _ := newTestPipeline(t)
	hunt := &fake.FakeHunt{
		HuntName: "test",
		Srcs: []core.Source{
			&fake.FakeSource{SourceName: "fake", Items: makeRawItems(3)},
		},
		Eval: &testutil.FakeEvaluator{
			Picks: []core.Pick{{Score: 0.8, DisplayScore: "8/10", Reason: "Good"}},
		},
	}

	result := p.Run(context.Background(), hunt)
	if result.Scanned != 3 {
		t.Fatalf("expected 3 scanned, got %d", result.Scanned)
	}
}

func TestPipeline_DeduplicatesOpportunities(t *testing.T) {
	p, _ := newTestPipeline(t)
	items := makeRawItems(2)
	hunt := &fake.FakeHunt{
		HuntName: "test",
		Srcs: []core.Source{
			&fake.FakeSource{SourceName: "src1", Items: items},
			&fake.FakeSource{SourceName: "src2", Items: items}, // same items
		},
		Eval: &testutil.FakeEvaluator{},
	}

	result := p.Run(context.Background(), hunt)
	// Should deduplicate — only 2 unique items stored.
	if result.Scanned != 2 {
		t.Fatalf("expected 2 scanned (deduped), got %d", result.Scanned)
	}
}

func TestPipeline_EvaluateCallsEvaluator(t *testing.T) {
	p, _ := newTestPipeline(t)
	evaluator := &testutil.FakeEvaluator{
		Picks:   []core.Pick{{Score: 0.9, DisplayScore: "9/10", Reason: "Amazing"}},
		CostUSD: 0.003,
	}
	hunt := &fake.FakeHunt{
		HuntName: "test",
		Srcs: []core.Source{
			&fake.FakeSource{SourceName: "fake", Items: makeRawItems(2)},
		},
		Eval: evaluator,
	}

	result := p.Run(context.Background(), hunt)
	if result.Evaluated == 0 {
		t.Fatal("expected evaluations")
	}
	if len(evaluator.Calls) == 0 {
		t.Fatal("expected evaluator to be called")
	}
}

func TestPipeline_ErrorsCollectedInResult(t *testing.T) {
	p, _ := newTestPipeline(t)
	hunt := &fake.FakeHunt{
		HuntName: "test",
		Srcs: []core.Source{
			&fake.FakeSource{SourceName: "fail", Err: errors.New("source failed")},
		},
		Eval: &testutil.FakeEvaluator{},
	}

	result := p.Run(context.Background(), hunt)
	if len(result.Errors) == 0 {
		t.Fatal("expected errors in result")
	}
}

func TestPipeline_BudgetGating(t *testing.T) {
	db := testutil.NewTestDB(t)
	notifier := &testutil.FakeNotifier{}
	ct := core.NewCostTracker(100, map[string]float64{"test": 100}) // already at budget
	p := pipeline.New(db, ct, notifier, core.ScanRegion{}, "")

	budget := 10.0
	evaluator := &testutil.FakeEvaluator{
		Picks: []core.Pick{{Score: 0.5}},
	}
	hunt := &fake.FakeHunt{
		HuntName: "test",
		Sched:    core.Schedule{MaxMonthlySpendUSD: &budget},
		Srcs: []core.Source{
			&fake.FakeSource{SourceName: "fake", Items: makeRawItems(1)},
		},
		Eval: evaluator,
	}

	result := p.Run(context.Background(), hunt)
	// Should have scanned but NOT evaluated due to budget.
	if result.Scanned != 1 {
		t.Fatalf("expected 1 scanned, got %d", result.Scanned)
	}
	if result.Evaluated != 0 {
		t.Fatalf("expected 0 evaluated (budget exceeded), got %d", result.Evaluated)
	}
}
