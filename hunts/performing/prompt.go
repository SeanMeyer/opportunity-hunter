package performing

import (
	"fmt"
	"strings"

	"github.com/seanmeyer/opportunity-hunter/core"
)

func buildPrompt(ec core.EvalContext) string {
	var b strings.Builder

	b.WriteString(`You are an expert performing arts recommender for Denver, CO. Evaluate the following shows and score each on a scale of 1-10.

Unlike comedy or movies, performing arts scoring should heavily weight EXTERNAL signals: critical acclaim, Tony awards, production company reputation, touring status, and cultural buzz. Use your knowledge of the arts world to assess production quality.

`)

	if ec.Preferences != "" {
		fmt.Fprintf(&b, "## User Preferences & Scoring Guidance\n\n%s\n\n", ec.Preferences)
	}

	b.WriteString("## Feedback on past recommendations\n\n" + core.FormatFeedback(ec.Feedback) + "\n\n")

	b.WriteString("## Shows to Evaluate\n\n")

	for i, opp := range ec.Opportunities {
		fmt.Fprintf(&b, "### Show %d: %s\n", i+1, opp.Title)
		if opp.Subtitle != "" {
			fmt.Fprintf(&b, "- Details: %s\n", opp.Subtitle)
		}
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
			fmt.Fprintf(&b, "- Dates: %s\n", strings.Join(dates, ", "))
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
- reason: why this is worth seeing (focus on production quality, cultural significance)
- genre: one of musical, play, opera, ballet, dance, symphony, other
- urgency: what the user should do (e.g., "buy tickets now — limited run")

If no shows score 7+, return an empty picks list.
Explain in skipped_reasoning why the remaining shows didn't make the cut.
`)

	return b.String()
}
