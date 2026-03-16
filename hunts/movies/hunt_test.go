package movies_test

import (
	"testing"
	"time"

	"github.com/seanmeyer/opportunity-hunter/core"
	"github.com/seanmeyer/opportunity-hunter/hunts/movies"
)

// Compile-time interface checks.
var _ core.Hunt = (*movies.MoviesHunt)(nil)
var _ core.Grouper = (*movies.MoviesHunt)(nil)
var _ core.Expirer = (*movies.MoviesHunt)(nil)
var _ core.VenueEnricher = (*movies.MoviesHunt)(nil)
var _ core.DefaultPreferencer = (*movies.MoviesHunt)(nil)
var _ core.WebHunt = (*movies.MoviesHunt)(nil)
var _ core.NotifyHunt = (*movies.MoviesHunt)(nil)

func TestDedupeKey_Theatrical(t *testing.T) {
	h := &movies.MoviesHunt{}
	key := h.DedupeKey(core.RawItem{
		Title:     "Dune: Part Three",
		StartTime: "2026-03-15T00:00:00Z",
	})
	if key != "Dune: Part Three|2026" {
		t.Fatalf("unexpected key: %q", key)
	}
}

func TestDedupeKey_Screening(t *testing.T) {
	h := &movies.MoviesHunt{}
	key := h.DedupeKey(core.RawItem{
		Title:     "The Godfather",
		VenueName: "Sie FilmCenter",
		StartTime: "2026-03-20T19:00:00-06:00",
	})
	if key != "The Godfather|Sie FilmCenter|2026-03-20" {
		t.Fatalf("unexpected key: %q", key)
	}
}

func TestShouldExpire_Theatrical(t *testing.T) {
	h := &movies.MoviesHunt{}

	// 9 weeks ago — should expire.
	old := core.Opportunity{
		StartTime:  time.Now().Add(-9 * 7 * 24 * time.Hour),
		Attributes: movies.MovieAttrs{ReleaseType: "theatrical"}.Encode(),
	}
	if !h.ShouldExpire(old) {
		t.Fatal("expected theatrical 9 weeks ago to expire")
	}

	// 2 weeks ago — should NOT expire.
	recent := core.Opportunity{
		StartTime:  time.Now().Add(-2 * 7 * 24 * time.Hour),
		Attributes: movies.MovieAttrs{ReleaseType: "theatrical"}.Encode(),
	}
	if h.ShouldExpire(recent) {
		t.Fatal("expected theatrical 2 weeks ago to NOT expire")
	}
}

func TestShouldExpire_Streaming(t *testing.T) {
	h := &movies.MoviesHunt{}

	// 6 months ago — streaming should NEVER expire.
	old := core.Opportunity{
		StartTime:  time.Now().Add(-180 * 24 * time.Hour),
		Attributes: movies.MovieAttrs{ReleaseType: "streaming"}.Encode(),
	}
	if h.ShouldExpire(old) {
		t.Fatal("streaming should never expire")
	}
}

func TestShouldExpire_Screening(t *testing.T) {
	h := &movies.MoviesHunt{}

	past := core.Opportunity{
		StartTime:  time.Now().Add(-24 * time.Hour),
		Attributes: movies.MovieAttrs{ReleaseType: "screening"}.Encode(),
	}
	if !h.ShouldExpire(past) {
		t.Fatal("expected past screening to expire")
	}
}

func TestFeedbackOptions(t *testing.T) {
	h := &movies.MoviesHunt{}
	opts := h.FeedbackOptions()
	if len(opts) != 4 {
		t.Fatalf("expected 4 feedback options, got %d", len(opts))
	}
}

func TestCardRenderer(t *testing.T) {
	h := &movies.MoviesHunt{}
	renderer := h.CardRenderer()

	attrs := movies.MovieAttrs{
		Genre:       []string{"Sci-Fi", "Drama"},
		Director:    "Denis Villeneuve",
		TMDBRating:  8.5,
		ReleaseType: "theatrical",
	}
	card := renderer.RenderCard(
		core.Opportunity{Title: "Dune: Part Three", Attributes: attrs.Encode(), StartTime: time.Now()},
		core.Pick{Score: 0.9, DisplayScore: "9/10"},
		core.Venue{},
	)
	if card.Title != "Dune: Part Three" {
		t.Fatalf("expected 'Dune: Part Three', got %q", card.Title)
	}
	if card.ScoreTier != core.ScoreHigh {
		t.Fatalf("expected ScoreHigh, got %s", card.ScoreTier)
	}
	// Should have genre, director, TMDB, release fields.
	if len(card.Fields) < 3 {
		t.Fatalf("expected at least 3 fields, got %d", len(card.Fields))
	}
	if card.DateDisplay == "" {
		t.Fatal("expected DateDisplay to be set")
	}
}

func TestCardRenderer_WithVenueDistance(t *testing.T) {
	h := &movies.MoviesHunt{}
	renderer := h.CardRenderer()

	attrs := movies.MovieAttrs{ReleaseType: "theatrical"}
	card := renderer.RenderCard(
		core.Opportunity{Title: "Test Movie", Attributes: attrs.Encode(), StartTime: time.Now()},
		core.Pick{Score: 0.8, DisplayScore: "8/10"},
		core.Venue{Name: "Alamo Drafthouse Sloans Lake", WalkingMinutes: 12, DistanceMi: 0.6},
	)

	hasDistance := false
	for _, f := range card.Fields {
		if f.Label == "Distance" {
			hasDistance = true
			break
		}
	}
	if !hasDistance {
		t.Fatal("expected Distance field for venue with WalkingMinutes")
	}
}

func TestNotifyFormatter_NoReminderForStreaming(t *testing.T) {
	h := &movies.MoviesHunt{}
	formatter := h.NotifyFormatter()

	actions := formatter.FormatReminder(
		core.Opportunity{
			Title:      "Some Movie",
			Attributes: movies.MovieAttrs{ReleaseType: "streaming"}.Encode(),
		},
		core.Pick{DisplayScore: "7/10"},
		"",
	)
	if len(actions) != 0 {
		t.Fatal("expected no reminder for streaming movie")
	}
}

func TestNotifyFormatter_IncludesUrgency(t *testing.T) {
	h := &movies.MoviesHunt{}
	formatter := h.NotifyFormatter()

	actions := formatter.FormatPicks(core.NotifyContext{
		Picks: []core.Pick{
			{OpportunityID: 1, DisplayScore: "9/10", Reason: "Great film", Urgency: "See it in IMAX this weekend"},
		},
		Opportunities: []core.Opportunity{
			{ID: 1, Title: "Dune: Part Three"},
		},
	})
	if len(actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(actions))
	}
	desc := actions[0].Message.Embeds[0].Description
	if !contains(desc, "See it in IMAX this weekend") {
		t.Fatalf("expected urgency in description, got: %s", desc)
	}
}

func TestDefaultPreferences(t *testing.T) {
	h := &movies.MoviesHunt{}
	prefs := h.DefaultPreferences()
	if prefs == "" {
		t.Fatal("expected non-empty default preferences")
	}
	if !contains(prefs, "Scoring Calibration") {
		t.Fatal("expected scoring calibration in default preferences")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
