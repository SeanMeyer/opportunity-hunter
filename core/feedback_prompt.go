package core

import (
	"fmt"
	"strings"
)

// FormatFeedback preserves current recommendation feedback and historical outcome labels.
// Skipping an activity does not necessarily mean the recommendation was bad.
func FormatFeedback(entries []FeedbackEntry) string {
	if len(entries) == 0 {
		return "No feedback yet."
	}
	var b strings.Builder
	b.WriteString("Use this feedback to understand the person's taste; distinguish recommendation quality from attendance outcomes.\n")
	for _, fb := range entries {
		label := fb.Rating
		switch fb.Rating {
		case "up":
			label = "Good recommendation"
		case "down":
			label = "Bad recommendation"
		case "loved":
			label = "Loved"
		case "good":
			label = "Good experience"
		case "meh":
			label = "Unenthusiastic"
		case "not_for_me":
			label = "Not for me"
		case "went_great":
			label = "Went, great experience"
		case "went_ok":
			label = "Went, okay experience"
		case "skipped":
			label = "Skipped (reason unknown unless noted)"
		}
		fmt.Fprintf(&b, "- %s: %s\n", label, fb.OpportunityTitle)
		if fb.Note != "" {
			fmt.Fprintf(&b, "  Note: %s\n", fb.Note)
		}
		if fb.EvalScore != "" || fb.EvalSummary != "" {
			fmt.Fprintf(&b, "  Previous assessment: %s — %s\n", fb.EvalScore, fb.EvalSummary)
		}
	}
	return b.String()
}
