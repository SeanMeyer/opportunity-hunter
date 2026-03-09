package performing

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/seanmeyer/opportunity-hunter/core"
	"github.com/seanmeyer/opportunity-hunter/hunts/performing/sources"
	"github.com/seanmeyer/opportunity-hunter/llm"
)

// PerformingHunt discovers and evaluates performing arts events.
// Implements: Hunt + Grouper + WebHunt + NotifyHunt
type PerformingHunt struct {
	tmKey string
	llmC  *llm.Client
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

	client, err := llm.NewClient(ctx, apiKey)
	if err != nil {
		return fmt.Errorf("performing-arts: create LLM client: %w", err)
	}
	h.llmC = client

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
	return raw.Title + "|" + raw.VenueName + "|" + date
}

func (h *PerformingHunt) Evaluator() core.Evaluator {
	return &performingEvaluator{llm: h.llmC}
}

func (h *PerformingHunt) DefaultSchedule() core.Schedule {
	return core.Schedule{
		ScanInterval: 12 * time.Hour,
		EvalInterval: 7 * 24 * time.Hour,
		RemindBefore: []time.Duration{7 * 24 * time.Hour, 24 * time.Hour},
	}
}
