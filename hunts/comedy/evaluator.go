package comedy

import (
	"context"
	"time"

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
	_ = prompt // Full LLM integration deferred — placeholder picks for now.

	var picks []core.Pick
	for _, opp := range filtered {
		picks = append(picks, core.Pick{
			OpportunityID: opp.ID,
			Score:         0.5,
			DisplayScore:  "5/10",
			Reason:        "Evaluation pending — LLM integration in progress",
		})
	}

	return &core.EvalResult{
		Evaluation: core.Evaluation{
			HuntName:    "comedy",
			EvaluatedAt: time.Now(),
		},
		Picks: picks,
	}, nil
}
