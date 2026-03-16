package performing_test

import (
	"testing"
	"time"

	"github.com/seanmeyer/opportunity-hunter/core"
	"github.com/seanmeyer/opportunity-hunter/hunts/performing"
)

// Compile-time interface checks.
var _ core.Hunt = (*performing.PerformingHunt)(nil)
var _ core.Grouper = (*performing.PerformingHunt)(nil)
var _ core.VenueEnricher = (*performing.PerformingHunt)(nil)
var _ core.DefaultPreferencer = (*performing.PerformingHunt)(nil)
var _ core.WebHunt = (*performing.PerformingHunt)(nil)
var _ core.NotifyHunt = (*performing.PerformingHunt)(nil)

func TestDedupeKey(t *testing.T) {
	h := &performing.PerformingHunt{}
	raw := core.RawItem{
		Title:     "Hamilton",
		VenueName: "Buell Theatre",
		StartTime: "2026-03-15T19:30:00-06:00",
	}
	key := h.DedupeKey(raw)
	if key != "hamilton|Buell Theatre|2026-03-15" {
		t.Fatalf("unexpected dedupe key: %q", key)
	}
}

func TestDefaultSchedule(t *testing.T) {
	h := &performing.PerformingHunt{}
	s := h.DefaultSchedule()
	if s.ScanInterval != 12*time.Hour {
		t.Fatalf("expected 12h scan interval, got %v", s.ScanInterval)
	}
	if len(s.RemindBefore) != 2 {
		t.Fatalf("expected 2 remind windows, got %d", len(s.RemindBefore))
	}
}

func TestFeedbackOptions(t *testing.T) {
	h := &performing.PerformingHunt{}
	opts := h.FeedbackOptions()
	if len(opts) != 3 {
		t.Fatalf("expected 3 feedback options, got %d", len(opts))
	}
	values := map[string]bool{}
	for _, o := range opts {
		values[o.Value] = true
	}
	for _, want := range []string{"loved", "not_for_me", "already_seen"} {
		if !values[want] {
			t.Fatalf("missing feedback option: %s", want)
		}
	}
}

func TestGroupForEval(t *testing.T) {
	h := &performing.PerformingHunt{}
	opps := []core.Opportunity{
		{ID: 1, Title: "Show A", StartTime: time.Date(2026, 3, 10, 19, 0, 0, 0, time.UTC)},
		{ID: 2, Title: "Show B", StartTime: time.Date(2026, 3, 11, 20, 0, 0, 0, time.UTC)},
		{ID: 3, Title: "Show C", StartTime: time.Date(2026, 3, 18, 19, 0, 0, 0, time.UTC)}, // different week
	}
	groups := h.GroupForEval(opps)
	if len(groups) != 2 {
		t.Fatalf("expected 2 groups (2 weeks), got %d", len(groups))
	}
}

func TestCardRenderer(t *testing.T) {
	h := &performing.PerformingHunt{}
	renderer := h.CardRenderer()
	if renderer == nil {
		t.Fatal("expected non-nil CardRenderer")
	}

	price := 50.0
	card := renderer.RenderCard(
		core.Opportunity{Title: "Hamilton", Subtitle: "Musical", PriceMin: &price, StartTime: time.Now()},
		core.Pick{Score: 0.9, DisplayScore: "9/10", Reason: "Must see"},
		core.Venue{Name: "Buell Theatre"},
	)
	if card.Title != "Hamilton" {
		t.Fatalf("expected 'Hamilton', got %q", card.Title)
	}
	if card.ScoreTier != core.ScoreHigh {
		t.Fatalf("expected ScoreHigh, got %s", card.ScoreTier)
	}
}

func TestNotifyFormatter(t *testing.T) {
	h := &performing.PerformingHunt{}
	formatter := h.NotifyFormatter()
	if formatter == nil {
		t.Fatal("expected non-nil NotifyFormatter")
	}

	actions := formatter.FormatPicks(core.NotifyContext{
		Picks: []core.Pick{
			{OpportunityID: 1, DisplayScore: "8/10", Reason: "Great show"},
		},
		Opportunities: []core.Opportunity{
			{ID: 1, Title: "Hamilton"},
		},
	})
	if len(actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(actions))
	}
	if actions[0].Type != core.PostMessage {
		t.Fatalf("expected PostMessage, got %s", actions[0].Type)
	}
}
