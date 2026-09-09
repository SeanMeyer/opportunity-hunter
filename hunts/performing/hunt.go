package performing

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/seanmeyer/opportunity-hunter/core"
	"github.com/seanmeyer/opportunity-hunter/distance"
	"github.com/seanmeyer/opportunity-hunter/hunts/performing/sources"
	"github.com/seanmeyer/opportunity-hunter/llm"
)

// PerformingHunt discovers and evaluates performing arts events.
// Implements: Hunt + Grouper + VenueEnricher + DefaultPreferencer + WebHunt + NotifyHunt
type PerformingHunt struct {
	tmKey       string
	llmC        *llm.Client
	distClient  *distance.Client
	homeAddress string
}

func (h *PerformingHunt) Name() string { return "performing-arts" }

func (h *PerformingHunt) Init(ctx context.Context, lookup func(string) string) error {
	h.tmKey = lookup("TICKETMASTER_API_KEY")
	if h.tmKey == "" {
		return fmt.Errorf("performing-arts: TICKETMASTER_API_KEY required")
	}

	apiKey := lookup("GOOGLE_API_KEY")
	if apiKey == "" {
		return fmt.Errorf("performing-arts: GOOGLE_API_KEY required")
	}

	client, err := llm.NewClient(ctx, apiKey, lookup("GEMINI_MODEL"))
	if err != nil {
		return fmt.Errorf("performing-arts: create LLM client: %w", err)
	}
	h.llmC = client

	h.homeAddress = lookup("HOME_ADDRESS")
	if h.homeAddress != "" {
		h.distClient = distance.NewClient(apiKey, &http.Client{Timeout: 10 * time.Second})
	}

	return nil
}

func (h *PerformingHunt) Sources() []core.Source {
	return []core.Source{
		sources.NewTicketmaster(h.tmKey, &http.Client{Timeout: 30 * time.Second}),
	}
}

func (h *PerformingHunt) DedupeKey(raw core.RawItem) string {
	t, _ := time.Parse(time.RFC3339, raw.StartTime)
	date := t.Format("2006-01-02")
	return core.NormalizeTitleForDedup(raw.Title) + "|" + raw.VenueName + "|" + date
}

func (h *PerformingHunt) MultiDateKey(raw core.RawItem) string {
	return core.NormalizeTitleForDedup(raw.Title) + "|" + raw.VenueName
}

func (h *PerformingHunt) Evaluator() core.Evaluator {
	return &performingEvaluator{llm: h.llmC}
}

func (h *PerformingHunt) DefaultSchedule() core.Schedule {
	return core.Schedule{
		ScanInterval: 7 * 24 * time.Hour,
		EvalInterval: 7 * 24 * time.Hour,
		RemindBefore: []time.Duration{7 * 24 * time.Hour, 24 * time.Hour},
	}
}

// GroupForEval groups opportunities by week for batch evaluation.
func (h *PerformingHunt) GroupForEval(items []core.Opportunity) []core.Group {
	weeks := make(map[string][]core.Opportunity)
	for _, opp := range items {
		year, week := opp.StartTime.ISOWeek()
		key := fmt.Sprintf("%d-W%02d", year, week)
		weeks[key] = append(weeks[key], opp)
	}

	var groups []core.Group
	for key, opps := range weeks {
		groups = append(groups, core.Group{
			Key:           key,
			Opportunities: opps,
		})
	}
	return groups
}

// EnrichVenues fetches walking distances for venues that don't have them yet.
func (h *PerformingHunt) EnrichVenues(ctx context.Context, venues map[int64]core.Venue) {
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

// DefaultPreferences returns sensible starting preferences for performing arts.
// These are seeded into the DB on first run and editable by the user via the web UI.
func (h *PerformingHunt) DefaultPreferences() string {
	return strings.TrimSpace(`
I love Broadway musicals most. Interested in orchestra and curious about ballet, but only if something is truly notable.

## Scoring Calibration
- 9-10: Major touring Broadway (Hamilton, Wicked, Hadestown) or once-in-a-generation cultural event
- 7-8: Well-reviewed touring show, acclaimed company, or notable revival. Strong cultural value.
- 5-6: Solid local production or less-known touring show. Worth considering.
- 3-4: Standard community theater, recurring local shows, or low-match genres
- 1-2: Student recitals, amateur productions, very niche content

## Genre Guidance
- Musicals: No penalty. Score on production quality and cultural significance.
- Plays/Theater: Slight discount unless critically acclaimed or notable cast.
- Orchestra/Symphony: High bar — only 7+ for truly notable programs or performers.
- Ballet/Dance: Very high bar — only 7+ for major companies or landmark productions.
- Opera: Same as ballet — only surface truly exceptional productions.
`)
}
