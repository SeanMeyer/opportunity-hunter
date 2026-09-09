package comedy

import (
	"fmt"
	"strings"

	"github.com/seanmeyer/opportunity-hunter/core"
)

// formatShowDates formats ShowDates for display in prompts and cards.
func formatShowDates(dates []string) string {
	switch len(dates) {
	case 0:
		return ""
	case 1:
		return dates[0]
	default:
		return strings.Join(dates, ", ")
	}
}

func buildPrompt(ec core.EvalContext) string {
	var b strings.Builder

	b.WriteString(`You are a comedy show recommendation engine for Denver, CO. Evaluate the following shows and score each on a scale of 1-10.

## Scoring Calibration
- **9-10**: Rare alignment with user preferences. Headliner the user specifically loves or a can't-miss touring act.
- **7-8**: Strong match. Well-known comedian or niche appeal aligned with stated preferences.
- **5-6**: Decent show, but not a strong match. Worth knowing about but not a must-see.
- **1-4**: Not a match. Generic open mic, repeated house show, or misaligned style.

`)

	if ec.Preferences != "" {
		fmt.Fprintf(&b, "## User Preferences\n\n%s\n\n", ec.Preferences)
	}

	b.WriteString("## Feedback on past recommendations\n\n" + core.FormatFeedback(ec.Feedback) + "\n\n")

	b.WriteString("## Shows to Evaluate\n\n")

	showIdx := 0
	for _, opp := range ec.Opportunities {
		if shouldSkipForEval(opp.Title) {
			continue
		}
		showIdx++
		fmt.Fprintf(&b, "### Show %d: %s\n", showIdx, opp.Title)
		if opp.VenueID != nil {
			if venue, ok := ec.Venues[*opp.VenueID]; ok {
				venueLine := venue.Name
				if venue.WalkingMinutes > 0 {
					venueLine += fmt.Sprintf(" (%d min walk)", venue.WalkingMinutes)
				}
				fmt.Fprintf(&b, "- Venue: %s\n", venueLine)
			}
		}

		// Format dates from ShowDates.
		if len(opp.ShowDates) <= 1 {
			fmt.Fprintf(&b, "- Date/Time: %s\n", opp.StartTime.Format("Mon Jan 2, 3:04 PM"))
		} else {
			var dates []string
			for _, d := range opp.ShowDates {
				dates = append(dates, d.Format("Mon Jan 2, 3:04 PM"))
			}
			fmt.Fprintf(&b, "- Dates: %s\n", formatShowDates(dates))
		}

		if opp.PriceMin != nil {
			if opp.PriceMax != nil {
				fmt.Fprintf(&b, "- Price: $%.0f - $%.0f\n", *opp.PriceMin, *opp.PriceMax)
			} else {
				fmt.Fprintf(&b, "- Price: from $%.0f\n", *opp.PriceMin)
			}
		}
		b.WriteString("\n")
	}

	b.WriteString(`## Instructions

For each show that scores 7+, provide:
- show_id: index from the list above
- score: 1-10 rating
- reason: why this matches the user's preferences
- sell_out_risk: low/medium/high
- urgency: what the user should do

If no shows score 7+, return an empty picks list.
`)

	return b.String()
}
