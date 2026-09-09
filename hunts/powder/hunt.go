package powder

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/seanmeyer/opportunity-hunter/core"
	"github.com/seanmeyer/opportunity-hunter/hunts/powder/catalog"
	"github.com/seanmeyer/opportunity-hunter/hunts/powder/sources"
	"github.com/seanmeyer/opportunity-hunter/hunts/powder/weather"
	"github.com/seanmeyer/opportunity-hunter/llm"
)

// PowderHunt discovers and evaluates powder opportunities.
// Implements: Hunt + ReEvaluator + Briefer + WebHunt + NotifyHunt
type PowderHunt struct {
	llmC           *llm.Client
	weatherService *weather.Service
	catalog        []catalog.RegionWithResorts
	regions        []catalog.Region
	costTracker    *core.CostTracker

	// Forecast cache for weather change gating during re-evaluation.
	cacheMu       sync.RWMutex
	forecastCache map[string][]weather.Forecast
}

func (h *PowderHunt) Name() string { return "powder" }

func (h *PowderHunt) Init(ctx context.Context, lookup func(string) string) error {
	apiKey := lookup("GOOGLE_API_KEY")
	if apiKey == "" {
		return fmt.Errorf("powder: GOOGLE_API_KEY required")
	}
	client, err := llm.NewClient(ctx, apiKey, lookup("GEMINI_MODEL"))
	if err != nil {
		return fmt.Errorf("powder: create LLM client: %w", err)
	}
	h.llmC = client

	// Create weather clients.
	httpClient := &http.Client{Timeout: 30 * time.Second}
	openMeteo := weather.NewOpenMeteoClient(httpClient)
	nws := weather.NewNWSClient(httpClient)
	h.weatherService = weather.NewService(openMeteo, nws)

	// Load catalog with friction tiers based on user's home coordinates.
	homeLat := parseFloat(lookup("HOME_LATITUDE"), 39.75)
	homeLon := parseFloat(lookup("HOME_LONGITUDE"), -104.99)
	h.catalog = catalog.RegionsForUser(homeLat, homeLon)
	h.regions = make([]catalog.Region, 0, len(h.catalog))
	for _, rr := range h.catalog {
		h.regions = append(h.regions, rr.Region)
	}

	return nil
}

func (h *PowderHunt) Sources() []core.Source {
	return []core.Source{
		sources.NewWeatherSource(h.weatherService, h.catalog),
	}
}

func (h *PowderHunt) DedupeKey(raw core.RawItem) string {
	return raw.Title + "|" + raw.StartTime + "|" + raw.EndTime
}

func (h *PowderHunt) Evaluator() core.Evaluator {
	return &powderEvaluator{llm: h.llmC}
}

func (h *PowderHunt) DefaultSchedule() core.Schedule {
	budget := 10.0
	return core.Schedule{
		ScanInterval:       24 * time.Hour,
		EvalInterval:       0,
		RemindBefore:       []time.Duration{2 * 24 * time.Hour},
		MaxMonthlySpendUSD: &budget,
	}
}

func (h *PowderHunt) WebConfig() core.WebConfig {
	return core.WebConfig{
		SortOptions: []core.SortOption{
			{Value: core.SortByScore, Label: "Recommendation"},
			{Value: core.SortByTier, Label: "Tier"},
			{Value: core.SortByRegion, Label: "Region (A-Z)"},
		},
		FilterOptions: []core.FilterOption{
			{Value: "DROP_EVERYTHING", Label: "DROP EVERYTHING"},
			{Value: "RECOMMENDED", Label: "Recommended"},
			{Value: "WATCH", Label: "Watch"},
			{Value: "SKIP", Label: "Skip"},
		},
		DefaultSort: core.SortByScore,
	}
}

// SetCostTracker allows the pipeline to inject the cost tracker for budget checks.
func (h *PowderHunt) SetCostTracker(ct *core.CostTracker) {
	h.costTracker = ct
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
