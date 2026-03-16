package performing

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	genai "google.golang.org/genai"

	"github.com/seanmeyer/opportunity-hunter/core"
	"github.com/seanmeyer/opportunity-hunter/llm"
)

type performingEvaluator struct {
	llm *llm.Client
}

func (e *performingEvaluator) Evaluate(ctx context.Context, ec core.EvalContext) (*core.EvalResult, error) {
	if len(ec.Opportunities) == 0 {
		return &core.EvalResult{
			Evaluation: core.Evaluation{
				HuntName:         "performing-arts",
				EvaluatedAt:      time.Now(),
				SkippedReasoning: "no opportunities to evaluate",
			},
		}, nil
	}

	prompt := buildPrompt(ec)

	twoStep, err := e.llm.TwoStep(ctx, prompt, performingEvalSchema())
	if err != nil {
		return nil, fmt.Errorf("performing evaluate: %w", err)
	}

	picks := parsePicks(twoStep.Structured, ec.Opportunities)

	slog.Info("performing evaluation complete",
		"shows", len(ec.Opportunities),
		"picks", len(picks),
	)

	return &core.EvalResult{
		Evaluation: core.Evaluation{
			HuntName:         "performing-arts",
			EvaluatedAt:      time.Now(),
			RawLLMResponse:   twoStep.Research,
			RenderedPrompt:   prompt,
			SkippedReasoning: stringField(twoStep.Structured, "skipped_reasoning"),
		},
		Picks: picks,
	}, nil
}

// parsePicks extracts picks from the LLM structured output.
// show_id maps 1:1 to the opportunity list (no group expansion needed
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
			slog.Warn("performing pick: show_id out of range", "show_id", showID, "max", len(opps))
			continue
		}
		opp := opps[showID-1]

		score := intField(entry, "score")
		reason := stringField(entry, "reason")
		genre := stringField(entry, "genre")
		urgency := stringField(entry, "urgency")

		attrs := PerformingAttrs{Genre: genre}

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

// performingEvalSchema defines the structured output schema for the performing arts evaluator.
func performingEvalSchema() *genai.Schema {
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
							Description: "Why this show is worth seeing (production quality, cultural significance)",
						},
						"genre": {
							Type: genai.TypeString,
							Enum: []string{"musical", "play", "opera", "ballet", "dance", "symphony", "other"},
						},
						"urgency": {
							Type:        genai.TypeString,
							Description: "What the user should do (e.g., buy tickets now — limited run)",
						},
					},
					Required: []string{"show_id", "score", "reason", "genre", "urgency"},
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
