package comedy

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	genai "google.golang.org/genai"

	"github.com/seanmeyer/opportunity-hunter/core"
	"github.com/seanmeyer/opportunity-hunter/distance"
	"github.com/seanmeyer/opportunity-hunter/llm"
)

type comedyEvaluator struct {
	llm         *llm.Client
	distClient  *distance.Client
	homeAddress string
}

func (e *comedyEvaluator) Evaluate(ctx context.Context, ec core.EvalContext) (*core.EvalResult, error) {
	// Filter out recurring house shows.
	var filtered []core.Opportunity
	for _, opp := range ec.Opportunities {
		if !shouldSkipForEval(opp.Title) {
			filtered = append(filtered, opp)
		}
	}
	if len(filtered) == 0 {
		return &core.EvalResult{
			Evaluation: core.Evaluation{
				HuntName:         "comedy",
				EvaluatedAt:      time.Now(),
				SkippedReasoning: "all shows filtered (recurring house shows)",
			},
		}, nil
	}

	prompt := buildPrompt(ec)

	twoStep, err := e.llm.TwoStep(ctx, prompt, comedyEvalSchema())
	if err != nil {
		return nil, fmt.Errorf("comedy evaluate: %w", err)
	}

	picks := parsePicks(twoStep.Structured, filtered)

	slog.Info("comedy evaluation complete",
		"shows", len(filtered),
		"picks", len(picks),
	)

	return &core.EvalResult{
		Evaluation: core.Evaluation{
			HuntName:           "comedy",
			EvaluatedAt:        time.Now(),
			RawLLMResponse:     twoStep.Research,
			StructuredResponse: twoStep.RawJSON,
			RenderedPrompt:     twoStep.RenderedPrompt,
			SkippedReasoning:   stringField(twoStep.Structured, "skipped_reasoning"),
			CostUSD:            twoStep.CostUSD,
		},
		Picks: picks,
	}, nil
}

// parsePicks extracts picks from the LLM structured output.
// show_id maps 1:1 to the filtered opportunity list (no group expansion needed
// since multi-date merging now happens at scan time).
func parsePicks(structured map[string]any, opps []core.Opportunity) []core.Pick {
	rawPicks, ok := structured["picks"]
	if !ok {
		return nil
	}
	items, ok := rawPicks.([]any)
	if !ok {
		return nil
	}

	var picks []core.Pick
	for _, item := range items {
		entry, ok := item.(map[string]any)
		if !ok {
			continue
		}

		showID := intField(entry, "show_id")
		if showID < 1 || showID > len(opps) {
			slog.Warn("comedy pick: show_id out of range", "show_id", showID, "max", len(opps))
			continue
		}
		opp := opps[showID-1]

		score := intField(entry, "score")
		reason := stringField(entry, "reason")
		sellOutRisk := stringField(entry, "sell_out_risk")
		urgency := stringField(entry, "urgency")

		attrs := ComedyAttrs{SellOutRisk: sellOutRisk}

		picks = append(picks, core.Pick{
			OpportunityID: opp.ID,
			Score:         float64(score) / 10.0,
			DisplayScore:  fmt.Sprintf("%d/10", score),
			Reason:        reason,
			Urgency:       urgency,
			Attributes:    attrs.Encode(),
		})
	}
	return picks
}

func intField(m map[string]any, key string) int {
	v, ok := m[key]
	if !ok {
		return 0
	}
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	default:
		return 0
	}
}

func stringField(m map[string]any, key string) string {
	v, ok := m[key]
	if !ok {
		return ""
	}
	s, _ := v.(string)
	return s
}

// comedyEvalSchema defines the structured output schema for the comedy evaluator.
func comedyEvalSchema() *genai.Schema {
	return &genai.Schema{
		Type: genai.TypeObject,
		Properties: map[string]*genai.Schema{
			"picks": {
				Type: genai.TypeArray,
				Items: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"show_id": {
							Type:        genai.TypeInteger,
							Description: "1-based index of the show from the list",
						},
						"score": {
							Type:        genai.TypeInteger,
							Description: "Score from 1-10",
						},
						"reason": {
							Type:        genai.TypeString,
							Description: "Why this show matches user preferences",
						},
						"sell_out_risk": {
							Type: genai.TypeString,
							Enum: []string{"low", "medium", "high"},
						},
						"urgency": {
							Type:        genai.TypeString,
							Description: "What the user should do (e.g., buy tickets now)",
						},
					},
					Required: []string{"show_id", "score", "reason", "sell_out_risk", "urgency"},
				},
			},
			"skipped_reasoning": {
				Type:        genai.TypeString,
				Description: "Explanation of why shows scoring below 7 were skipped",
			},
		},
		Required: []string{"picks", "skipped_reasoning"},
	}
}
