package movies_test

import (
	"testing"
	"time"

	"github.com/seanmeyer/opportunity-hunter/core"
	"github.com/seanmeyer/opportunity-hunter/hunts/movies"
)

var _ core.Hunt = (*movies.MoviesHunt)(nil)
var _ core.Expirer = (*movies.MoviesHunt)(nil)
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
		Genre:      []string{"Sci-Fi", "Drama"},
		Director:   "Denis Villeneuve",
		TMDBRating: 8.5,
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
