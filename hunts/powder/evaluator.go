package powder

import (
	"context"
	"time"

	"github.com/seanmeyer/opportunity-hunter/core"
	"github.com/seanmeyer/opportunity-hunter/llm"
)

type powderEvaluator struct {
	llm *llm.Client
}

func (e *powderEvaluator) Evaluate(ctx context.Context, ec core.EvalContext) (*core.EvalResult, error) {
	prompt := buildPrompt(ec)
	_ = prompt // Full LLM integration deferred.

	var picks []core.Pick
	for _, opp := range ec.Opportunities {
		attrs, _ := DecodePowderAttrs(opp.Attributes)
		displayScore := string(TierOnTheRadar)
		score := 0.5
		if attrs.SnowfallIn >= 18 {
			displayScore = string(TierDropEverything)
			score = 0.95
		} else if attrs.SnowfallIn >= 10 {
			displayScore = string(TierWorthALook)
			score = 0.75
		}

		picks = append(picks, core.Pick{
			OpportunityID: opp.ID,
			Score:         score,
			DisplayScore:  displayScore,
			Reason:        "Evaluation pending — LLM integration in progress",
			Attributes:    attrs.Encode(),
		})
	}

	return &core.EvalResult{
		Evaluation: core.Evaluation{
			HuntName:    "powder",
			EvaluatedAt: time.Now(),
		},
		Picks: picks,
	}, nil
}
