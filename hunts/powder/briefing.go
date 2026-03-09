package powder

import (
	"context"
	"fmt"

	"github.com/seanmeyer/opportunity-hunter/core"
)

// Synthesize creates a cross-region briefing for a notification group.
func (h *PowderHunt) Synthesize(ctx context.Context, group core.NotifyGroup, ct *core.CostTracker) (string, error) {
	if h.llmC == nil {
		return fmt.Sprintf("Storm briefing for %s (%d regions)", group.Key, len(group.Evaluations)), nil
	}

	// In production, this calls the LLM with a cross-region comparison prompt.
	// For now, return a placeholder briefing.
	return fmt.Sprintf("Storm system impacting %s with %d regions showing activity. Monitor for updates.",
		group.Key, len(group.Evaluations)), nil
}
