package powder

import (
	"context"
	"fmt"
	"strings"

	"github.com/seanmeyer/opportunity-hunter/core"
)

const briefingPrompt = `You are a powder skiing briefing writer. Synthesize the following region-level storm evaluations into a concise 2-3 sentence cross-region briefing.

Focus on: which regions look best, how they compare, and the overall storm pattern.
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
		summaries.WriteString(fmt.Sprintf("**%s**: %s\n", eval.GroupKey, truncate(eval.RawLLMResponse, 500)))
	}

	prompt := fmt.Sprintf(briefingPrompt, summaries.String())
	text, err := h.llmC.Generate(ctx, prompt, nil)
	if err != nil {
		// Fall back to simple briefing on LLM failure.
		return fmt.Sprintf("Storm system impacting %s with %d regions showing activity. Monitor for updates.",
			group.Key, len(group.Evaluations)), nil
	}

	if ct != nil {
		ct.Add(h.Name(), 0.001) // Approximate cost for briefing call.
	}

	return strings.TrimSpace(text), nil
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
