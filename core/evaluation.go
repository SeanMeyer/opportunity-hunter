package core

import (
	"context"
	"fmt"
	"time"
)

// Evaluator evaluates a group of opportunities and returns an evaluation with picks.
type Evaluator interface {
	Evaluate(ctx context.Context, ec EvalContext) (*Evaluation, error)
}

// EvalContext carries everything an evaluator needs.
type EvalContext struct {
	Opportunities []Opportunity
	Venues        map[int64]Venue
	Preferences   string
	Feedback      []FeedbackEntry
	CostTracker   *CostTracker
	PriorEval     *Evaluation
}

// Validate checks that the EvalContext has required fields.
func (ec *EvalContext) Validate() error {
	if len(ec.Opportunities) == 0 {
		return fmt.Errorf("evalcontext: at least one opportunity is required")
	}
	if ec.CostTracker == nil {
		return fmt.Errorf("evalcontext: CostTracker is required")
	}
	for _, opp := range ec.Opportunities {
		if opp.VenueID != nil {
			if _, ok := ec.Venues[*opp.VenueID]; !ok {
				return fmt.Errorf("evalcontext: venue ID %d not found in Venues map", *opp.VenueID)
			}
		}
	}
	return nil
}

// Evaluation is the output of an evaluator run.
type Evaluation struct {
	ID               int64
	HuntName         string
	GroupKey         string
	EvaluatedAt      time.Time
	SkippedReasoning string
	RawLLMResponse   string
	RenderedPrompt   string
	CostUSD          float64
}

// Pick is a single recommended opportunity from an evaluation.
type Pick struct {
	ID            int64
	EvaluationID  int64
	OpportunityID int64
	Score         float64
	DisplayScore  string
	Reason        string
	Urgency       string
	Attributes    Attributes
}

// FeedbackEntry is a single piece of user feedback, used in eval prompts.
type FeedbackEntry struct {
	OpportunityTitle string
	Rating           string
	Note             string
}

// Group is a batch of opportunities evaluated together.
type Group struct {
	Key           string
	Opportunities []Opportunity
	Venues        map[int64]Venue
}

// NotifyGroup is a batch of evaluations notified together (post-eval grouping).
type NotifyGroup struct {
	Key         string
	Evaluations []Evaluation
}
