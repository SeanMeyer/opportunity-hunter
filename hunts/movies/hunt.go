package movies

import (
	"context"
	"fmt"
	"time"

	"github.com/seanmeyer/opportunity-hunter/core"
	"github.com/seanmeyer/opportunity-hunter/hunts/movies/sources"
	"github.com/seanmeyer/opportunity-hunter/llm"
)

// MoviesHunt discovers and evaluates movies.
// Implements: Hunt + Expirer + WebHunt + NotifyHunt
type MoviesHunt struct {
	tmdbKey string
	tmKey   string // optional Ticketmaster for local screenings
	llmC    *llm.Client
}

func (h *MoviesHunt) Name() string { return "movies" }

func (h *MoviesHunt) Init(ctx context.Context, lookup func(string) string) error {
	h.tmdbKey = lookup("TMDB_API_KEY")
	if h.tmdbKey == "" {
		return fmt.Errorf("movies: TMDB_API_KEY required")
	}
	h.tmKey = lookup("TICKETMASTER_API_KEY") // optional

	apiKey := lookup("GOOGLE_API_KEY")
	if apiKey == "" {
		return fmt.Errorf("movies: GOOGLE_API_KEY required")
	}
	client, err := llm.NewClient(ctx, apiKey)
	if err != nil {
		return fmt.Errorf("movies: create LLM client: %w", err)
	}
	h.llmC = client

	return nil
}

func (h *MoviesHunt) Sources() []core.Source {
	return []core.Source{
		sources.NewTMDB(h.tmdbKey, nil),
	}
}

func (h *MoviesHunt) DedupeKey(raw core.RawItem) string {
	// For general releases: title|year. For screenings: title|venue|date.
	if raw.VenueName != "" {
		t, _ := time.Parse(time.RFC3339, raw.StartTime)
		return raw.Title + "|" + raw.VenueName + "|" + t.Format("2006-01-02")
	}
	t, _ := time.Parse(time.RFC3339, raw.StartTime)
	return raw.Title + "|" + t.Format("2006")
}

func (h *MoviesHunt) Evaluator() core.Evaluator {
	return &moviesEvaluator{llm: h.llmC}
}

func (h *MoviesHunt) DefaultSchedule() core.Schedule {
	return core.Schedule{
		ScanInterval: 24 * time.Hour,
		EvalInterval: 7 * 24 * time.Hour,
		RemindBefore: []time.Duration{24 * time.Hour},
	}
}

// ShouldExpire implements Expirer.
// Theatrical: expire 8 weeks after release. Streaming: never expire. Screening: expire after date.
func (h *MoviesHunt) ShouldExpire(opp core.Opportunity) bool {
	attrs, err := DecodeMovieAttrs(opp.Attributes)
	if err != nil {
		// Default: expire if start time is in the past.
		return opp.StartTime.Before(time.Now())
	}

	switch attrs.ReleaseType {
	case "theatrical":
		return time.Since(opp.StartTime) > 8*7*24*time.Hour // 8 weeks
	case "streaming":
		return false // never expire
	case "screening":
		return opp.StartTime.Before(time.Now())
	default:
		return opp.StartTime.Before(time.Now())
	}
}
