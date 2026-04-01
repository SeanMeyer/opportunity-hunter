package movies

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	genai "google.golang.org/genai"

	"github.com/seanmeyer/opportunity-hunter/core"
	"github.com/seanmeyer/opportunity-hunter/hunts/movies/catalog"
	"github.com/seanmeyer/opportunity-hunter/llm"
)

type moviesEvaluator struct {
	llm      *llm.Client
	theaters []catalog.Theater
	homeLat  float64
	homeLon  float64
}

func (e *moviesEvaluator) Evaluate(ctx context.Context, ec core.EvalContext) (*core.EvalResult, error) {
	if len(ec.Opportunities) == 0 {
		return &core.EvalResult{
			Evaluation: core.Evaluation{
				HuntName:         "movies",
				EvaluatedAt:      time.Now(),
				SkippedReasoning: "no opportunities to evaluate",
			},
		}, nil
	}

	prompt := buildPrompt(ec, e.theaters, e.homeLat, e.homeLon)

	twoStep, err := e.llm.TwoStep(ctx, prompt, moviesEvalSchema())
	if err != nil {
		return nil, fmt.Errorf("movies evaluate: %w", err)
	}

	picks := parsePicks(twoStep.Structured, ec.Opportunities)

	slog.Info("movies evaluation complete",
		"movies", len(ec.Opportunities),
		"picks", len(picks),
	)

	return &core.EvalResult{
		Evaluation: core.Evaluation{
			HuntName:         "movies",
			EvaluatedAt:      time.Now(),
			RawLLMResponse:   twoStep.Research,
			RenderedPrompt:   prompt,
			SkippedReasoning: stringField(twoStep.Structured, "skipped_reasoning"),
			CostUSD:          twoStep.CostUSD,
		},
		Picks: picks,
	}, nil
}

// parsePicks extracts picks from the LLM structured output, mapping movie_id
// back to opportunity IDs.
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

		movieID := intField(entry, "movie_id")
		if movieID < 1 || movieID > len(opps) {
			slog.Warn("movies pick: movie_id out of range", "movie_id", movieID, "max", len(opps))
			continue
		}
		opp := opps[movieID-1] // 1-based index from LLM

		score := intField(entry, "score")
		reason := stringField(entry, "reason")
		urgency := stringField(entry, "urgency")

		picks = append(picks, core.Pick{
			OpportunityID: opp.ID,
			Score:         float64(score) / 10.0,
			DisplayScore:  fmt.Sprintf("%d/10", score),
			Reason:        reason,
			Urgency:       urgency,
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

// moviesEvalSchema defines the structured output schema for the movies evaluator.
func moviesEvalSchema() *genai.Schema {
	return &genai.Schema{
		Type: genai.TypeObject,
		Properties: map[string]*genai.Schema{
			"picks": {
				Type: genai.TypeArray,
				Items: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"movie_id": {
							Type:        genai.TypeInteger,
							Description: "1-based index of the movie from the list",
						},
						"score": {
							Type:        genai.TypeInteger,
							Description: "Score from 1-10",
						},
						"reason": {
							Type:        genai.TypeString,
							Description: "Why this movie is worth seeing (taste match, critical reception, viewing experience)",
						},
						"urgency": {
							Type:        genai.TypeString,
							Description: "What the user should do (e.g., see it in IMAX this weekend, catch $8 Terror Tuesday)",
						},
					},
					Required: []string{"movie_id", "score", "reason", "urgency"},
				},
			},
			"skipped_reasoning": {
				Type:        genai.TypeString,
				Description: "Explanation of why movies scoring below 7 were skipped",
			},
		},
		Required: []string{"picks", "skipped_reasoning"},
	}
}
