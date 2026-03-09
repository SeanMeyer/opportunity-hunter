package powder

import (
	"context"
	"fmt"
	"time"

	"github.com/seanmeyer/opportunity-hunter/core"
	"github.com/seanmeyer/opportunity-hunter/hunts/powder/catalog"
	"github.com/seanmeyer/opportunity-hunter/hunts/powder/sources"
	"github.com/seanmeyer/opportunity-hunter/llm"
)

// PowderHunt discovers and evaluates powder opportunities.
// Implements: Hunt + ReEvaluator + Briefer + WebHunt + NotifyHunt
type PowderHunt struct {
	llmC        *llm.Client
	regions     []catalog.Region
	costTracker *core.CostTracker
}

func (h *PowderHunt) Name() string { return "powder" }

func (h *PowderHunt) Init(ctx context.Context, lookup func(string) string) error {
	apiKey := lookup("GOOGLE_API_KEY")
	if apiKey == "" {
		return fmt.Errorf("powder: GOOGLE_API_KEY required")
	}
	client, err := llm.NewClient(ctx, apiKey)
	if err != nil {
		return fmt.Errorf("powder: create LLM client: %w", err)
	}
	h.llmC = client
	h.regions = catalog.DefaultRegions()

	return nil
}

func (h *PowderHunt) Sources() []core.Source {
	return []core.Source{
		sources.NewOpenMeteo(h.regions),
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
		ScanInterval:       12 * time.Hour,
		EvalInterval:       0, // eval after every scan
		RemindBefore:       []time.Duration{2 * 24 * time.Hour},
		MaxMonthlySpendUSD: &budget,
	}
}

// SetCostTracker allows the pipeline to inject the cost tracker for budget checks.
func (h *PowderHunt) SetCostTracker(ct *core.CostTracker) {
	h.costTracker = ct
}
