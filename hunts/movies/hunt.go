package movies

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/seanmeyer/opportunity-hunter/core"
	"github.com/seanmeyer/opportunity-hunter/distance"
	"github.com/seanmeyer/opportunity-hunter/hunts/movies/catalog"
	"github.com/seanmeyer/opportunity-hunter/hunts/movies/sources"
	"github.com/seanmeyer/opportunity-hunter/llm"
)

// MoviesHunt discovers and evaluates movies.
// Implements: Hunt + Grouper + Expirer + VenueEnricher + DefaultPreferencer + WebHunt + NotifyHunt
type MoviesHunt struct {
	tmdbKey     string
	tmKey       string // optional Ticketmaster for local screenings
	llmC        *llm.Client
	distClient  *distance.Client
	homeAddress string
	homeLat     float64
	homeLon     float64
	theaters    []catalog.Theater
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

	h.homeAddress = lookup("HOME_ADDRESS")
	if h.homeAddress != "" {
		h.distClient = distance.NewClient(apiKey, &http.Client{Timeout: 10 * time.Second})
	}

	h.homeLat = parseFloat(lookup("HOME_LATITUDE"), 39.75)
	h.homeLon = parseFloat(lookup("HOME_LONGITUDE"), -104.99)

	h.theaters = catalog.Theaters()
	slog.Info("movies: loaded theater catalog", "theaters", len(h.theaters))

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
	return &moviesEvaluator{llm: h.llmC, theaters: h.theaters, homeLat: h.homeLat, homeLon: h.homeLon}
}

func (h *MoviesHunt) DefaultSchedule() core.Schedule {
	return core.Schedule{
		ScanInterval: 24 * time.Hour,
		EvalInterval: 7 * 24 * time.Hour,
		RemindBefore: []time.Duration{24 * time.Hour},
	}
}

// GroupForEval batches all movies into a single evaluation group.
// Unlike comedy/performing (which group by week), movies are all "currently available"
// so one LLM call evaluates the whole batch — saving ~20x in API calls.
func (h *MoviesHunt) GroupForEval(items []core.Opportunity) []core.Group {
	if len(items) == 0 {
		return nil
	}
	return []core.Group{{
		Key:           "all-movies",
		Opportunities: items,
	}}
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

// EnrichVenues fetches walking distances for venues that don't have them yet.
func (h *MoviesHunt) EnrichVenues(ctx context.Context, venues map[int64]core.Venue) {
	if h.distClient == nil || h.homeAddress == "" {
		return
	}
	for id, venue := range venues {
		if venue.WalkingMinutes > 0 || venue.Address == "" {
			continue
		}
		result, err := h.distClient.GetDistance(ctx, h.homeAddress, venue.Address, "WALK")
		if err != nil {
			slog.Warn("distance lookup failed", "venue", venue.Name, "err", err)
			continue
		}
		venue.WalkingMinutes = result.Minutes
		venue.DistanceMi = result.DistanceMi
		venues[id] = venue
	}
}

// DefaultPreferences returns sensible starting preferences for movies.
// These are seeded into the DB on first run and editable by the user via the web UI.
func (h *MoviesHunt) DefaultPreferences() string {
	return strings.TrimSpace(`
I enjoy sci-fi, thriller, and well-crafted drama. Open to horror if critically acclaimed.
Less interested in rom-coms and animated films unless they're exceptional.

## Scoring Calibration
- 9-10: Perfect taste match AND great viewing experience (walkable theater, good price, IMAX for spectacle films)
- 7-8: Strong taste match or exceptional film regardless of genre. Good theater option available.
- 5-6: Decent match. Worth knowing about but not rushing to see.
- 3-4: Weak match or only available at inconvenient theaters.
- 1-2: Not a match for user's taste.

## Theater Preferences
- Prefer walkable theaters for casual viewing
- Willing to drive for IMAX/Dolby on spectacle films (Marvel, sci-fi epics, Nolan)
- Discount nights are a plus for "maybe" films — cheap Tuesday can push a 5 to a 7
- Alamo Drafthouse experience is a bonus for any film
`)
}

func parseFloat(s string, fallback float64) float64 {
	if s == "" {
		return fallback
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return fallback
	}
	return v
}
