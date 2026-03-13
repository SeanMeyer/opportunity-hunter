package powder_test

import (
	"testing"
	"time"

	"github.com/seanmeyer/opportunity-hunter/core"
	"github.com/seanmeyer/opportunity-hunter/hunts/powder"
)

var _ core.Hunt = (*powder.PowderHunt)(nil)
var _ core.ReEvaluator = (*powder.PowderHunt)(nil)
var _ core.Briefer = (*powder.PowderHunt)(nil)
var _ core.WebHunt = (*powder.PowderHunt)(nil)
var _ core.NotifyHunt = (*powder.PowderHunt)(nil)

func TestDedupeKey(t *testing.T) {
	h := &powder.PowderHunt{}
	key := h.DedupeKey(core.RawItem{
		Title:     "Front Range",
		StartTime: "2026-03-15T00:00:00Z",
		EndTime:   "2026-03-17T00:00:00Z",
	})
	if key != "Front Range|2026-03-15T00:00:00Z|2026-03-17T00:00:00Z" {
		t.Fatalf("unexpected dedupe key: %q", key)
	}
}

func TestDefaultSchedule_HasBudget(t *testing.T) {
	h := &powder.PowderHunt{}
	s := h.DefaultSchedule()
	if s.MaxMonthlySpendUSD == nil {
		t.Fatal("expected budget cap")
	}
	if *s.MaxMonthlySpendUSD != 10.0 {
		t.Fatalf("expected $10 budget, got %f", *s.MaxMonthlySpendUSD)
	}
}

func TestFeedbackOptions(t *testing.T) {
	h := &powder.PowderHunt{}
	opts := h.FeedbackOptions()
	if len(opts) != 3 {
		t.Fatalf("expected 3 feedback options, got %d", len(opts))
	}
}

func TestCardRenderer_TierMapping(t *testing.T) {
	h := &powder.PowderHunt{}
	renderer := h.CardRenderer()

	card := renderer.RenderCard(
		core.Opportunity{Title: "Summit County", StartTime: time.Now()},
		core.Pick{Score: 0.95, DisplayScore: string(powder.TierDropEverything)},
		core.Venue{},
	)
	if card.ScoreTier != core.ScoreHigh {
		t.Fatalf("expected ScoreHigh for DROP_EVERYTHING, got %s", card.ScoreTier)
	}
}

func TestNotifyFormatter_ThreadedActions(t *testing.T) {
	h := &powder.PowderHunt{}
	formatter := h.NotifyFormatter()

	actions := formatter.FormatPicks(core.NotifyContext{
		Picks: []core.Pick{
			{OpportunityID: 1, DisplayScore: string(powder.TierDropEverything), Reason: "Epic storm"},
			{OpportunityID: 2, DisplayScore: string(powder.TierWorthALook), Reason: "Solid storm"},
		},
		Opportunities: []core.Opportunity{
			{ID: 1, Title: "Front Range", Subtitle: "Mar 15-17"},
			{ID: 2, Title: "Summit County", Subtitle: "Mar 15-17"},
		},
		Synthesis: "Major storm system incoming",
	})

	if len(actions) != 3 {
		t.Fatalf("expected 3 actions (1 thread + 2 details), got %d", len(actions))
	}
	if actions[0].Type != core.CreateThread {
		t.Fatalf("expected CreateThread, got %s", actions[0].Type)
	}
	if actions[1].Type != core.PostToThread {
		t.Fatalf("expected PostToThread, got %s", actions[1].Type)
	}

	// Validate thread references.
	if err := core.ValidateActions(actions); err != nil {
		t.Fatalf("invalid actions: %v", err)
	}
}

func TestGroupForNotify(t *testing.T) {
	h := &powder.PowderHunt{}
	evals := []core.Evaluation{
		{GroupKey: "co_front_range"},
		{GroupKey: "co_front_range"},
		{GroupKey: "pnw_cascades"},
	}
	groups := h.GroupForNotify(evals)
	if len(groups) != 2 {
		t.Fatalf("expected 2 groups, got %d", len(groups))
	}
}

func TestShouldReEvaluate_NilLastEval(t *testing.T) {
	h := &powder.PowderHunt{}
	opp := core.Opportunity{Attributes: powder.PowderAttrs{FrictionTier: "local_drive"}.Encode()}
	if h.ShouldReEvaluate(opp, nil) {
		t.Fatal("expected false for nil lastEval")
	}
}

func TestFrictionTier_Thresholds(t *testing.T) {
	near, ext := powder.FrictionLocal.Thresholds()
	if near != 6 || ext != 12 {
		t.Fatalf("expected 6/12 for local, got %.0f/%.0f", near, ext)
	}

	near, ext = powder.FrictionFlight.Thresholds()
	if near != 24 || ext != 36 {
		t.Fatalf("expected 24/36 for flight, got %.0f/%.0f", near, ext)
	}
}
