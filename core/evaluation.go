package core

import (
	"context"
	"fmt"
	"time"
)

// EvalResult bundles an evaluation with its picks.
type EvalResult struct {
	Evaluation Evaluation
	Picks      []Pick
}

// Evaluator evaluates a group of opportunities and returns an evaluation with picks.
type Evaluator interface {
	Evaluate(ctx context.Context, ec EvalContext) (*EvalResult, error)
}

// EvalContext carries everything an evaluator needs.
type EvalContext struct {
	Opportunities []Opportunity
	Venues        map[int64]Venue
	Preferences   string
	Profile       *UserProfile
	Feedback      []FeedbackEntry
	CostTracker   *CostTracker
	PriorEval     *Evaluation
}

// UserProfile holds structured subscriber profile data (hunt-agnostic).
type UserProfile struct {
	HuntName      string    `json:"hunt_name"`
	HomeBase      string    `json:"home_base"`
	HomeLat       float64   `json:"home_lat"`
	HomeLon       float64   `json:"home_lon"`
	Passes        []string  `json:"passes"`
	SkillLevel    string    `json:"skill_level"`
	Preferences   string    `json:"preferences"`
	RemoteWork    bool      `json:"remote_work"`
	PTODays       int       `json:"pto_days"`
	BlackoutDates []time.Time `json:"blackout_dates"`
	Extra         map[string]any `json:"extra,omitempty"`
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
	Rating           string // "up" or "down"
	Note             string
	EvalSummary      string // LLM's summary at the time of feedback
	EvalScore        string // tier/display score at the time of feedback
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
