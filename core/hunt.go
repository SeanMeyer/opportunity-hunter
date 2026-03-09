package core

import (
	"context"
	"log/slog"
	"time"
)

// Hunt is the required interface. Every hunt must implement this.
type Hunt interface {
	Name() string
	Init(ctx context.Context, lookup func(string) string) error
	Sources() []Source
	DedupeKey(raw RawItem) string
	Evaluator() Evaluator
	DefaultSchedule() Schedule
}

// Schedule controls pipeline timing and budget for a hunt.
type Schedule struct {
	ScanInterval       time.Duration
	EvalInterval       time.Duration
	RemindBefore       []time.Duration
	MaxMonthlySpendUSD *float64
}

// Grouper controls how opportunities are batched before evaluation.
// Default: each opportunity evaluated individually.
type Grouper interface {
	GroupForEval(items []Opportunity) []Group
}

// ReEvaluator controls whether an opportunity should be re-evaluated.
// Default: no re-evaluation.
type ReEvaluator interface {
	ShouldReEvaluate(opp Opportunity, lastEval *Evaluation) bool
}

// Briefer controls post-eval grouping and synthesis.
// Default: one notification per evaluation, no synthesis.
type Briefer interface {
	GroupForNotify(evals []Evaluation) []NotifyGroup
	Synthesize(ctx context.Context, group NotifyGroup, costTracker *CostTracker) (string, error)
}

// Expirer controls when opportunities should be marked as expired.
// Default: expire when StartTime is in the past.
type Expirer interface {
	ShouldExpire(opp Opportunity) bool
}

// WebHunt provides UI customization.
// Default: generic card rendering, no feedback options.
type WebHunt interface {
	CardRenderer() CardRenderer
	FeedbackOptions() []FeedbackOption
}

// NotifyHunt provides notification formatting.
// Default: simple text message with picks.
type NotifyHunt interface {
	NotifyFormatter() NotifyFormatter
}

// HuntCapabilities records which optional interfaces a hunt implements.
type HuntCapabilities struct {
	HasGrouper      bool
	HasReEvaluator  bool
	HasBriefer      bool
	HasExpirer      bool
	HasWebHunt      bool
	HasNotifyHunt   bool
}

// ValidateHunt checks a hunt's interfaces and returns its capabilities.
func ValidateHunt(h Hunt) (HuntCapabilities, error) {
	var caps HuntCapabilities

	if _, ok := h.(Grouper); ok {
		caps.HasGrouper = true
	}
	if _, ok := h.(ReEvaluator); ok {
		caps.HasReEvaluator = true
	}
	if _, ok := h.(Briefer); ok {
		caps.HasBriefer = true
	}
	if _, ok := h.(Expirer); ok {
		caps.HasExpirer = true
	}
	if _, ok := h.(WebHunt); ok {
		caps.HasWebHunt = true
	}
	if _, ok := h.(NotifyHunt); ok {
		caps.HasNotifyHunt = true
	}

	slog.Info("hunt capabilities",
		"hunt", h.Name(),
		"grouper", caps.HasGrouper,
		"re_evaluator", caps.HasReEvaluator,
		"briefer", caps.HasBriefer,
		"expirer", caps.HasExpirer,
		"web_hunt", caps.HasWebHunt,
		"notify_hunt", caps.HasNotifyHunt,
	)

	return caps, nil
}
