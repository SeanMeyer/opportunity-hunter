package comedy_test

import (
	"testing"
	"time"

	"github.com/seanmeyer/opportunity-hunter/core"
	"github.com/seanmeyer/opportunity-hunter/hunts/comedy"
)

// Compile-time interface checks.
var _ core.Hunt = (*comedy.ComedyHunt)(nil)
var _ core.Grouper = (*comedy.ComedyHunt)(nil)
var _ core.WebHunt = (*comedy.ComedyHunt)(nil)
var _ core.NotifyHunt = (*comedy.ComedyHunt)(nil)

func TestDedupeKey(t *testing.T) {
	h := &comedy.ComedyHunt{}
	raw := core.RawItem{
		Title:     "Nate Bargatze",
		VenueName: "Comedy Works Downtown",
		StartTime: "2026-03-15T20:00:00-06:00",
	}
	key := h.DedupeKey(raw)
	if key != "nate bargatze|Comedy Works Downtown|2026-03-15" {
		t.Fatalf("unexpected dedupe key: %q", key)
	}
}

func TestDefaultSchedule(t *testing.T) {
	h := &comedy.ComedyHunt{}
	s := h.DefaultSchedule()
	if s.ScanInterval != 7*24*time.Hour {
		t.Fatalf("expected 7d scan interval, got %v", s.ScanInterval)
	}
	if len(s.RemindBefore) != 1 {
		t.Fatalf("expected 1 remind window, got %d", len(s.RemindBefore))
	}
}

func TestFeedbackOptions(t *testing.T) {
	h := &comedy.ComedyHunt{}
	opts := h.FeedbackOptions()
	if len(opts) != 2 {
		t.Fatalf("expected 2 feedback options, got %d", len(opts))
	}
}

func TestGroupForEval_WeeklyGrouping(t *testing.T) {
	h := &comedy.ComedyHunt{}
	opps := []core.Opportunity{
		{ID: 1, Title: "Show A", StartTime: time.Date(2026, 3, 10, 20, 0, 0, 0, time.UTC)},
		{ID: 2, Title: "Show B", StartTime: time.Date(2026, 3, 12, 20, 0, 0, 0, time.UTC)},
		{ID: 3, Title: "Show C", StartTime: time.Date(2026, 3, 20, 20, 0, 0, 0, time.UTC)}, // different week
	}
	groups := h.GroupForEval(opps)
	if len(groups) != 2 {
		t.Fatalf("expected 2 groups, got %d", len(groups))
	}
}

func TestCardRenderer(t *testing.T) {
	h := &comedy.ComedyHunt{}
	renderer := h.CardRenderer()
	if renderer == nil {
		t.Fatal("expected non-nil CardRenderer")
	}

	card := renderer.RenderCard(
		core.Opportunity{Title: "Nate Bargatze", StartTime: time.Now()},
		core.Pick{Score: 0.85, DisplayScore: "8/10", Reason: "Great comedian"},
		core.Venue{Name: "Comedy Works Downtown"},
	)
	if card.Title != "Nate Bargatze" {
		t.Fatalf("expected 'Nate Bargatze', got %q", card.Title)
	}
	if card.ScoreTier != core.ScoreHigh {
		t.Fatalf("expected ScoreHigh, got %s", card.ScoreTier)
	}
}

func TestNotifyFormatter(t *testing.T) {
	h := &comedy.ComedyHunt{}
	formatter := h.NotifyFormatter()
	if formatter == nil {
		t.Fatal("expected non-nil NotifyFormatter")
	}

	actions := formatter.FormatPicks(core.NotifyContext{
		Picks:         []core.Pick{{OpportunityID: 1, DisplayScore: "9/10", Reason: "Must see"}},
		Opportunities: []core.Opportunity{{ID: 1, Title: "Nate Bargatze"}},
	})
	if len(actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(actions))
	}
	if actions[0].Type != core.PostMessage {
		t.Fatalf("expected PostMessage, got %s", actions[0].Type)
	}
}
