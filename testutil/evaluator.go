package testutil

import (
	"context"
	"time"

	"github.com/seanmeyer/opportunity-hunter/core"
)

// FakeEvaluator returns configurable evaluations and picks.
type FakeEvaluator struct {
	Picks    []core.Pick   // picks to return
	CostUSD  float64       // cost to report
	Err      error         // error to return
	Calls    []core.EvalContext // recorded calls
}

// Evaluate records the call and returns configured results.
func (e *FakeEvaluator) Evaluate(_ context.Context, ec core.EvalContext) (*core.Evaluation, error) {
	e.Calls = append(e.Calls, ec)
	if e.Err != nil {
		return nil, e.Err
	}

	eval := &core.Evaluation{
		HuntName:    ec.Opportunities[0].HuntName,
		EvaluatedAt: time.Now(),
		CostUSD:     e.CostUSD,
	}

	return eval, nil
}

// GetPicks returns the configured picks. Called separately by pipeline.
func (e *FakeEvaluator) GetPicks() []core.Pick {
	return e.Picks
}
