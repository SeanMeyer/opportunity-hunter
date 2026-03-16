package core_test

import (
	"context"
	"testing"
	"time"

	"github.com/seanmeyer/opportunity-hunter/core"
)

// minimalHunt implements only the required Hunt interface.
type minimalHunt struct{}

func (h *minimalHunt) Name() string                                              { return "minimal" }
func (h *minimalHunt) Init(_ context.Context, _ func(string) string) error       { return nil }
func (h *minimalHunt) Sources() []core.Source                                    { return nil }
func (h *minimalHunt) DedupeKey(_ core.RawItem) string                           { return "key" }
func (h *minimalHunt) Evaluator() core.Evaluator                                 { return nil }
func (h *minimalHunt) DefaultSchedule() core.Schedule                            { return core.Schedule{} }

// fullHunt implements Hunt + all optional interfaces.
type fullHunt struct{ minimalHunt }

func (h *fullHunt) GroupForEval(_ []core.Opportunity) []core.Group        { return nil }
func (h *fullHunt) ShouldReEvaluate(_ core.Opportunity, _ *core.Evaluation) bool { return false }
func (h *fullHunt) GroupForNotify(_ []core.Evaluation) []core.NotifyGroup { return nil }
func (h *fullHunt) Synthesize(_ context.Context, _ core.NotifyGroup, _ *core.CostTracker) (string, error) {
	return "", nil
}
func (h *fullHunt) ShouldExpire(_ core.Opportunity) bool                       { return false }
func (h *fullHunt) CardRenderer() core.CardRenderer        { return nil }
func (h *fullHunt) FeedbackOptions() []core.FeedbackOption { return nil }
func (h *fullHunt) WebConfig() core.WebConfig              { return core.WebConfig{} }
func (h *fullHunt) EnrichVenues(_ context.Context, _ map[int64]core.Venue) {}
func (h *fullHunt) NotifyFormatter() core.NotifyFormatter  { return nil }

// Compile-time interface checks.
var _ core.Hunt = (*minimalHunt)(nil)
var _ core.Hunt = (*fullHunt)(nil)
var _ core.Grouper = (*fullHunt)(nil)
var _ core.ReEvaluator = (*fullHunt)(nil)
var _ core.Briefer = (*fullHunt)(nil)
var _ core.VenueEnricher = (*fullHunt)(nil)
var _ core.Expirer = (*fullHunt)(nil)
var _ core.WebHunt = (*fullHunt)(nil)
var _ core.NotifyHunt = (*fullHunt)(nil)

func TestValidateHunt_Minimal(t *testing.T) {
	caps, err := core.ValidateHunt(&minimalHunt{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if caps.HasGrouper || caps.HasReEvaluator || caps.HasBriefer || caps.HasExpirer || caps.HasWebHunt || caps.HasNotifyHunt {
		t.Fatal("minimal hunt should have no optional capabilities")
	}
}

func TestValidateHunt_Full(t *testing.T) {
	caps, err := core.ValidateHunt(&fullHunt{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !caps.HasGrouper {
		t.Error("expected HasGrouper")
	}
	if !caps.HasReEvaluator {
		t.Error("expected HasReEvaluator")
	}
	if !caps.HasBriefer {
		t.Error("expected HasBriefer")
	}
	if !caps.HasExpirer {
		t.Error("expected HasExpirer")
	}
	if !caps.HasWebHunt {
		t.Error("expected HasWebHunt")
	}
	if !caps.HasNotifyHunt {
		t.Error("expected HasNotifyHunt")
	}
}

func TestSchedule_Defaults(t *testing.T) {
	s := core.Schedule{
		ScanInterval: 12 * time.Hour,
		EvalInterval: 7 * 24 * time.Hour,
		RemindBefore: []time.Duration{24 * time.Hour},
	}
	if s.ScanInterval != 12*time.Hour {
		t.Fatal("ScanInterval mismatch")
	}
	if s.MaxMonthlySpendUSD != nil {
		t.Fatal("expected nil MaxMonthlySpendUSD")
	}
}

func TestSchedule_WithBudget(t *testing.T) {
	budget := 10.0
	s := core.Schedule{
		ScanInterval:       12 * time.Hour,
		MaxMonthlySpendUSD: &budget,
	}
	if s.MaxMonthlySpendUSD == nil || *s.MaxMonthlySpendUSD != 10.0 {
		t.Fatal("MaxMonthlySpendUSD mismatch")
	}
}
