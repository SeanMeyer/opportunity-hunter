package comedy

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/seanmeyer/opportunity-hunter/core"
	"github.com/seanmeyer/opportunity-hunter/distance"
	"github.com/seanmeyer/opportunity-hunter/hunts/comedy/sources"
	"github.com/seanmeyer/opportunity-hunter/llm"
)

// ComedyHunt discovers and evaluates comedy shows.
// Implements: Hunt + Grouper + WebHunt + NotifyHunt
type ComedyHunt struct {
	tmKey       string
	ebToken     string
	llmC        *llm.Client
	distClient  *distance.Client
	homeAddress string
}

func (h *ComedyHunt) Name() string { return "comedy" }

func (h *ComedyHunt) Init(ctx context.Context, lookup func(string) string) error {
	h.tmKey = lookup("TICKETMASTER_API_KEY")
	h.ebToken = lookup("EVENTBRITE_API_TOKEN")

	apiKey := lookup("GOOGLE_API_KEY")
	if apiKey == "" {
		return fmt.Errorf("comedy: GOOGLE_API_KEY required")
	}

	client, err := llm.NewClient(ctx, apiKey, lookup("GEMINI_MODEL"))
	if err != nil {
		return fmt.Errorf("comedy: create LLM client: %w", err)
	}
	h.llmC = client

	h.homeAddress = lookup("HOME_ADDRESS")
	if h.homeAddress != "" {
		h.distClient = distance.NewClient(apiKey, &http.Client{Timeout: 10 * time.Second})
	}

	return nil
}

func (h *ComedyHunt) Sources() []core.Source {
	var srcs []core.Source
	if h.tmKey != "" {
		srcs = append(srcs, sources.NewTicketmaster(h.tmKey, nil))
	}
	if h.ebToken != "" {
		srcs = append(srcs, sources.NewEventbrite(h.ebToken, nil))
	}
	srcs = append(srcs, sources.NewComedyWorks()) // always included
	return srcs
}

func (h *ComedyHunt) DedupeKey(raw core.RawItem) string {
	t, _ := time.Parse(time.RFC3339, raw.StartTime)
	date := core.EventLocalDate(t)
	return core.NormalizeTitleForDedup(raw.Title) + "|" + core.EventVenueKey(raw.VenueName, raw.VenueAddress) + "|" + date
}

func (h *ComedyHunt) MultiDateKey(raw core.RawItem) string {
	return core.NormalizeTitleForDedup(raw.Title) + "|" + core.EventVenueKey(raw.VenueName, raw.VenueAddress)
}

func (h *ComedyHunt) Evaluator() core.Evaluator {
	return &comedyEvaluator{
		llm:         h.llmC,
		distClient:  h.distClient,
		homeAddress: h.homeAddress,
	}
}

func (h *ComedyHunt) DefaultSchedule() core.Schedule {
	return core.Schedule{
		ScanInterval: 7 * 24 * time.Hour,
		EvalInterval: 7 * 24 * time.Hour,
		RemindBefore: []time.Duration{24 * time.Hour},
	}
}

// GroupForEval groups opportunities by week.
func (h *ComedyHunt) GroupForEval(items []core.Opportunity) []core.Group {
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

func (h *ComedyHunt) WebConfig() core.WebConfig {
	return core.WebConfig{
		SortOptions: []core.SortOption{
			{Value: core.SortByScore, Label: "Score (high to low)"},
			{Value: core.SortByDate, Label: "Date (soonest)"},
		},
		FilterOptions: []core.FilterOption{
			{Value: "8", Label: "8+ only"},
			{Value: "7", Label: "7+ only"},
		},
		DefaultSort: core.SortByScore,
	}
}

// EnrichVenues fetches walking distances for venues that don't have them yet.
func (h *ComedyHunt) EnrichVenues(ctx context.Context, venues map[int64]core.Venue) {
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

// shouldSkipForEval filters recurring house shows.
func shouldSkipForEval(name string) bool {
	lower := strings.ToLower(name)
	skipPatterns := []string{"thick skin", "new talent night", "new faces contest"}
	for _, pattern := range skipPatterns {
		if strings.Contains(lower, pattern) {
			return true
		}
	}
	return false
}
