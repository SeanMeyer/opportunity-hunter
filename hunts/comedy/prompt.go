package comedy

import (
	"fmt"
	"strings"

	"github.com/seanmeyer/opportunity-hunter/core"
)

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

	if len(ec.Feedback) > 0 {
		b.WriteString("## Past Feedback on Recommendations\n\n")
		for _, fb := range ec.Feedback {
			if fb.Rating == "loved" {
				fmt.Fprintf(&b, "- **Loved**: %s", fb.OpportunityTitle)
			} else {
				fmt.Fprintf(&b, "- **Passed on**: %s", fb.OpportunityTitle)
			}
			if fb.Note != "" {
				fmt.Fprintf(&b, " — \"%s\"", fb.Note)
			}
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}

	b.WriteString("## Shows to Evaluate\n\n")

	// Consolidate multi-night residencies.
	type showGroup struct {
		opp   core.Opportunity
		dates []string
	}
	type groupKey struct {
		title   string
		venueID int64
	}
	order := []groupKey{}
	groups := map[groupKey]*showGroup{}

	for _, opp := range ec.Opportunities {
		if shouldSkipForEval(opp.Title) {
			continue
		}
		vid := int64(0)
		if opp.VenueID != nil {
			vid = *opp.VenueID
		}
		k := groupKey{title: opp.Title, venueID: vid}
		if g, ok := groups[k]; ok {
			g.dates = append(g.dates, opp.StartTime.Format("Mon Jan 2, 3:04 PM"))
		} else {
			groups[k] = &showGroup{
				opp:   opp,
				dates: []string{opp.StartTime.Format("Mon Jan 2, 3:04 PM")},
			}
			order = append(order, k)
		}
	}

	for i, k := range order {
		g := groups[k]
		fmt.Fprintf(&b, "### Show %d: %s\n", i+1, g.opp.Title)
		if g.opp.VenueID != nil {
			if venue, ok := ec.Venues[*g.opp.VenueID]; ok {
				fmt.Fprintf(&b, "- Venue: %s\n", venue.Name)
			}
		}
		if len(g.dates) == 1 {
			fmt.Fprintf(&b, "- Date/Time: %s\n", g.dates[0])
		} else {
			fmt.Fprintf(&b, "- Dates: %s\n", strings.Join(g.dates, ", "))
		}
		if g.opp.PriceMin != nil {
			if g.opp.PriceMax != nil {
				fmt.Fprintf(&b, "- Price: $%.0f - $%.0f\n", *g.opp.PriceMin, *g.opp.PriceMax)
			} else {
				fmt.Fprintf(&b, "- Price: from $%.0f\n", *g.opp.PriceMin)
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
