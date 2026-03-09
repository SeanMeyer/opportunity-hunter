package performing

import (
	"fmt"
	"strings"

	"github.com/seanmeyer/opportunity-hunter/core"
)

func buildPrompt(ec core.EvalContext) string {
	var b strings.Builder

	b.WriteString(`You are an expert performing arts recommender. Evaluate the following shows and score each on a scale of 1-10.

Consider:
- Quality of production company/performers
- Genre appeal and reviews
- Value for price
- Venue quality
- Timing and accessibility

`)

	if ec.Preferences != "" {
		fmt.Fprintf(&b, "## User Preferences\n\n%s\n\n", ec.Preferences)
	}

	if len(ec.Feedback) > 0 {
		b.WriteString("## Past Feedback\n\n")
		for _, fb := range ec.Feedback {
			fmt.Fprintf(&b, "- %s: %s", fb.OpportunityTitle, fb.Rating)
			if fb.Note != "" {
				fmt.Fprintf(&b, " (%s)", fb.Note)
			}
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}

	b.WriteString("## Shows to Evaluate\n\n")
	for i, opp := range ec.Opportunities {
		fmt.Fprintf(&b, "### Show %d: %s\n", i+1, opp.Title)
		if opp.Subtitle != "" {
			fmt.Fprintf(&b, "- Details: %s\n", opp.Subtitle)
		}
		if opp.VenueID != nil {
			if venue, ok := ec.Venues[*opp.VenueID]; ok {
				fmt.Fprintf(&b, "- Venue: %s (%s)\n", venue.Name, venue.Address)
			}
		}
		fmt.Fprintf(&b, "- Date: %s\n", opp.StartTime.Format("Mon Jan 2, 2006 3:04 PM"))
		if opp.PriceMin != nil {
			if opp.PriceMax != nil {
				fmt.Fprintf(&b, "- Price: $%.0f - $%.0f\n", *opp.PriceMin, *opp.PriceMax)
			} else {
				fmt.Fprintf(&b, "- Price: from $%.0f\n", *opp.PriceMin)
			}
		}
		b.WriteString("\n")
	}

	return b.String()
}
