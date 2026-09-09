package powder

import (
	"context"
	"fmt"
	"strings"

	"github.com/seanmeyer/opportunity-hunter/core"
)

const briefingPrompt = `You are a powder skiing briefing writer. Synthesize the following region-level storm evaluations into a concise 2-3 sentence cross-region briefing.

Lead with the actual verdicts: which trips are exceptional, recommended, watch, or skip and why.
Preserve negative judgments and uncertainty; a strong storm is not necessarily a good trip.
Compare viable regions and say clearly when none is worth pursuing.
Keep it conversational and actionable. No JSON needed — just write the briefing text.

## Region Summaries

%s`

// Synthesize creates a cross-region briefing for a notification group.
func (h *PowderHunt) Synthesize(ctx context.Context, group core.NotifyGroup, ct *core.CostTracker) (string, error) {
	if h.llmC == nil || len(group.Evaluations) == 0 {
		return fmt.Sprintf("Storm system impacting %s with %d regions showing activity. Monitor for updates.",
			group.Key, len(group.Evaluations)), nil
	}

	// Build per-region summaries from evaluation data.
	var summaries strings.Builder
	for _, eval := range group.Evaluations {
		decision := eval.StructuredResponse
		if decision == "" {
			decision = eval.RawLLMResponse
		}
		summaries.WriteString(fmt.Sprintf("**%s**: %s\n", eval.GroupKey, decision))
	}

	prompt := fmt.Sprintf(briefingPrompt, summaries.String())
	result, err := h.llmC.Generate(ctx, prompt, nil)
	if err != nil {
		// Fall back to simple briefing on LLM failure.
		return fmt.Sprintf("Storm system impacting %s with %d regions showing activity. Monitor for updates.",
			group.Key, len(group.Evaluations)), nil
	}

	if ct != nil {
		ct.Add(h.Name(), result.CostUSD)
	}

	return strings.TrimSpace(result.Text), nil
}
