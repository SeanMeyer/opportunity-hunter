package fake

import (
	"context"

	"github.com/seanmeyer/opportunity-hunter/core"
)

// FakeHunt implements all required and optional interfaces for testing.
type FakeHunt struct {
	HuntName string
	Sched    core.Schedule
	Srcs     []core.Source
	Eval     core.Evaluator
	Dedupe   func(core.RawItem) string

	// Optional interface toggles — set to nil to disable.
	GrouperFn       func([]core.Opportunity) []core.Group
	ReEvalFn        func(core.Opportunity, *core.Evaluation) bool
	BrieferGroupFn  func([]core.Evaluation) []core.NotifyGroup
	SynthesizeFn    func(context.Context, core.NotifyGroup, *core.CostTracker) (string, error)
	ExpirerFn       func(core.Opportunity) bool
	CardRendererFn  func() core.CardRenderer
	FeedbackOptsFn  func() []core.FeedbackOption
	NotifyFmtFn     func() core.NotifyFormatter

	// Call recording.
	InitCalls    []func(string) string
	SourceCalls  int
	DedupeCalls  []core.RawItem
}

// Compile-time interface checks.
var _ core.Hunt = (*FakeHunt)(nil)
var _ core.Grouper = (*FakeHunt)(nil)
var _ core.ReEvaluator = (*FakeHunt)(nil)
var _ core.Briefer = (*FakeHunt)(nil)
var _ core.Expirer = (*FakeHunt)(nil)
var _ core.WebHunt = (*FakeHunt)(nil)
var _ core.NotifyHunt = (*FakeHunt)(nil)

// Hunt interface.
func (h *FakeHunt) Name() string { return h.HuntName }
func (h *FakeHunt) Init(_ context.Context, lookup func(string) string) error {
	h.InitCalls = append(h.InitCalls, lookup)
	return nil
}
func (h *FakeHunt) Sources() []core.Source {
	h.SourceCalls++
	return h.Srcs
}
func (h *FakeHunt) DedupeKey(raw core.RawItem) string {
	h.DedupeCalls = append(h.DedupeCalls, raw)
	if h.Dedupe != nil {
		return h.Dedupe(raw)
	}
	return raw.SourceID
}
func (h *FakeHunt) Evaluator() core.Evaluator       { return h.Eval }
func (h *FakeHunt) DefaultSchedule() core.Schedule   { return h.Sched }

// Grouper interface.
func (h *FakeHunt) GroupForEval(items []core.Opportunity) []core.Group {
	if h.GrouperFn != nil {
		return h.GrouperFn(items)
	}
	// Default: one group per opportunity.
	var groups []core.Group
	for _, item := range items {
		groups = append(groups, core.Group{
			Key:           item.Title,
			Opportunities: []core.Opportunity{item},
		})
	}
	return groups
}

// ReEvaluator interface.
func (h *FakeHunt) ShouldReEvaluate(opp core.Opportunity, lastEval *core.Evaluation) bool {
	if h.ReEvalFn != nil {
		return h.ReEvalFn(opp, lastEval)
	}
	return false
}

// Briefer interface.
func (h *FakeHunt) GroupForNotify(evals []core.Evaluation) []core.NotifyGroup {
	if h.BrieferGroupFn != nil {
		return h.BrieferGroupFn(evals)
	}
	return nil
}

func (h *FakeHunt) Synthesize(ctx context.Context, group core.NotifyGroup, ct *core.CostTracker) (string, error) {
	if h.SynthesizeFn != nil {
		return h.SynthesizeFn(ctx, group, ct)
	}
	return "fake synthesis", nil
}

// Expirer interface.
func (h *FakeHunt) ShouldExpire(opp core.Opportunity) bool {
	if h.ExpirerFn != nil {
		return h.ExpirerFn(opp)
	}
	return false
}

// WebHunt interface.
func (h *FakeHunt) CardRenderer() core.CardRenderer {
	if h.CardRendererFn != nil {
		return h.CardRendererFn()
	}
	return nil
}

func (h *FakeHunt) FeedbackOptions() []core.FeedbackOption {
	if h.FeedbackOptsFn != nil {
		return h.FeedbackOptsFn()
	}
	return []core.FeedbackOption{{Value: "liked", Label: "Liked"}}
}

// NotifyHunt interface.
func (h *FakeHunt) NotifyFormatter() core.NotifyFormatter {
	if h.NotifyFmtFn != nil {
		return h.NotifyFmtFn()
	}
	return nil
}

// FakeSource is a test source returning canned items.
type FakeSource struct {
	SourceName string
	Items      []core.RawItem
	Err        error
}

func (s *FakeSource) Name() string { return s.SourceName }
func (s *FakeSource) Scan(_ context.Context, _ core.ScanRegion) ([]core.RawItem, error) {
	return s.Items, s.Err
}
