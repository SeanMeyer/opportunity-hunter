package movies

import (
	"context"
	"time"

	"github.com/seanmeyer/opportunity-hunter/core"
	"github.com/seanmeyer/opportunity-hunter/llm"
)

type moviesEvaluator struct {
	llm *llm.Client
}

func (e *moviesEvaluator) Evaluate(ctx context.Context, ec core.EvalContext) (*core.EvalResult, error) {
	prompt := buildPrompt(ec)
	_ = prompt // Full LLM integration deferred.

	var picks []core.Pick
	for _, opp := range ec.Opportunities {
		picks = append(picks, core.Pick{
			OpportunityID: opp.ID,
			Score:         0.5,
			DisplayScore:  "5/10",
			Reason:        "Evaluation pending",
		})
	}

	return &core.EvalResult{
		Evaluation: core.Evaluation{
			HuntName:    "movies",
			EvaluatedAt: time.Now(),
		},
		Picks: picks,
	}, nil
}
