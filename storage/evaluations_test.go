package storage_test

import (
	"context"
	"testing"
	"time"

	"github.com/seanmeyer/opportunity-hunter/core"
)

func TestSaveAndGetEvaluation(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	eval := core.Evaluation{
		HuntName:       "comedy",
		GroupKey:        "2026-W10",
		EvaluatedAt:    time.Now(),
		RawLLMResponse: `{"picks": []}`,
		CostUSD:        0.003,
	}

	id, err := db.SaveEvaluation(ctx, eval)
	if err != nil {
		t.Fatal(err)
	}

	got, err := db.GetEvaluation(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if got.HuntName != "comedy" {
		t.Fatalf("expected 'comedy', got %q", got.HuntName)
	}
	if got.GroupKey != "2026-W10" {
		t.Fatalf("expected '2026-W10', got %q", got.GroupKey)
	}
	if got.CostUSD != 0.003 {
		t.Fatalf("expected 0.003, got %f", got.CostUSD)
	}
}

func TestSaveEvaluationWithPicks(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	oppID := insertTestOpportunity(t, db, "comedy", "Nate Bargatze")

	eval := core.Evaluation{
		HuntName:    "comedy",
		GroupKey:     "2026-W10",
		EvaluatedAt: time.Now(),
		CostUSD:     0.005,
	}
	picks := []core.Pick{
		{OpportunityID: oppID, Score: 0.85, DisplayScore: "8/10", Reason: "Great comedian"},
	}

	evalID, err := db.SaveEvaluationWithPicks(ctx, eval, picks)
	if err != nil {
		t.Fatal(err)
	}

	gotPicks, err := db.GetPicksForEvaluation(ctx, evalID)
	if err != nil {
		t.Fatal(err)
	}
	if len(gotPicks) != 1 {
		t.Fatalf("expected 1 pick, got %d", len(gotPicks))
	}
	if gotPicks[0].Score != 0.85 {
		t.Fatalf("expected 0.85, got %f", gotPicks[0].Score)
	}
	if gotPicks[0].EvaluationID != evalID {
		t.Fatalf("expected evaluation_id %d, got %d", evalID, gotPicks[0].EvaluationID)
	}
}

func TestGetLatestEvaluation(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	db.SaveEvaluation(ctx, core.Evaluation{
		HuntName:    "comedy",
		GroupKey:     "2026-W10",
		EvaluatedAt: time.Now().Add(-1 * time.Hour),
	})
	db.SaveEvaluation(ctx, core.Evaluation{
		HuntName:    "comedy",
		GroupKey:     "2026-W10",
		EvaluatedAt: time.Now(),
		CostUSD:     0.01,
	})

	latest, err := db.GetLatestEvaluation(ctx, "comedy", "2026-W10")
	if err != nil {
		t.Fatal(err)
	}
	if latest.CostUSD != 0.01 {
		t.Fatalf("expected latest eval with cost 0.01, got %f", latest.CostUSD)
	}
}

func TestGetPicksForOpportunity(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	oppID := insertTestOpportunity(t, db, "comedy", "Show A")

	eval1ID, _ := db.SaveEvaluation(ctx, core.Evaluation{
		HuntName: "comedy", GroupKey: "W1", EvaluatedAt: time.Now(),
	})
	eval2ID, _ := db.SaveEvaluation(ctx, core.Evaluation{
		HuntName: "comedy", GroupKey: "W2", EvaluatedAt: time.Now(),
	})

	db.SavePick(ctx, core.Pick{EvaluationID: eval1ID, OpportunityID: oppID, Score: 0.7})
	db.SavePick(ctx, core.Pick{EvaluationID: eval2ID, OpportunityID: oppID, Score: 0.9})

	picks, err := db.GetPicksForOpportunity(ctx, oppID)
	if err != nil {
		t.Fatal(err)
	}
	if len(picks) != 2 {
		t.Fatalf("expected 2 picks, got %d", len(picks))
	}
}
