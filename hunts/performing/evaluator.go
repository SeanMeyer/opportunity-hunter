package performing

import (
	"context"
	"time"

	"github.com/seanmeyer/opportunity-hunter/core"
	"github.com/seanmeyer/opportunity-hunter/llm"
)

type performingEvaluator struct {
	llm *llm.Client
}

func (e *performingEvaluator) Evaluate(ctx context.Context, ec core.EvalContext) (*core.EvalResult, error) {
	prompt := buildPrompt(ec)

	// For now, return a placeholder evaluation.
	// Full LLM integration in Task 20.
	_ = prompt

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
			HuntName:    "performing-arts",
			EvaluatedAt: time.Now(),
		},
		Picks: picks,
	}, nil
}
