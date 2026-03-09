package movies

import (
	"fmt"
	"strings"

	"github.com/seanmeyer/opportunity-hunter/core"
)

func buildPrompt(ec core.EvalContext) string {
	var b strings.Builder

	b.WriteString(`You are a movie recommendation engine. Evaluate movies based on the user's taste, learned from their feedback history.

## Scoring Calibration
- **9-10**: Perfect match for user's taste. Genre, director, or cast they consistently love.
- **7-8**: Strong match. Similar to movies they've rated highly.
- **5-6**: Could go either way. Worth mentioning but not a strong recommendation.
- **1-4**: Not a match. Genre or style they've consistently rated low.

`)

	if ec.Preferences != "" {
		fmt.Fprintf(&b, "## User Preferences\n\n%s\n\n", ec.Preferences)
	}

	if len(ec.Feedback) > 0 {
		b.WriteString("## Feedback History (learn taste from this)\n\n")
		loved := []string{}
		good := []string{}
		meh := []string{}
		notForMe := []string{}
		for _, fb := range ec.Feedback {
			entry := fb.OpportunityTitle
			if fb.Note != "" {
				entry += fmt.Sprintf(" (%s)", fb.Note)
			}
			switch fb.Rating {
			case "loved":
				loved = append(loved, entry)
			case "good":
				good = append(good, entry)
			case "meh":
				meh = append(meh, entry)
			case "not_for_me":
				notForMe = append(notForMe, entry)
			}
		}
		if len(loved) > 0 {
			fmt.Fprintf(&b, "**Loved**: %s\n", strings.Join(loved, ", "))
		}
		if len(good) > 0 {
			fmt.Fprintf(&b, "**Good**: %s\n", strings.Join(good, ", "))
		}
		if len(meh) > 0 {
			fmt.Fprintf(&b, "**Meh**: %s\n", strings.Join(meh, ", "))
		}
		if len(notForMe) > 0 {
			fmt.Fprintf(&b, "**Not for me**: %s\n", strings.Join(notForMe, ", "))
		}
		b.WriteString("\n")
	}

	b.WriteString("## Movies to Evaluate\n\n")
	for i, opp := range ec.Opportunities {
		fmt.Fprintf(&b, "### Movie %d: %s\n", i+1, opp.Title)
		if opp.Subtitle != "" {
			fmt.Fprintf(&b, "- Genre: %s\n", opp.Subtitle)
		}
		if opp.Attributes != nil {
			attrs, err := DecodeMovieAttrs(opp.Attributes)
			if err == nil {
				if attrs.Director != "" {
					fmt.Fprintf(&b, "- Director: %s\n", attrs.Director)
				}
				if len(attrs.Cast) > 0 {
					fmt.Fprintf(&b, "- Cast: %s\n", strings.Join(attrs.Cast, ", "))
				}
				if attrs.TMDBRating > 0 {
					fmt.Fprintf(&b, "- TMDB Rating: %.1f/10\n", attrs.TMDBRating)
				}
				fmt.Fprintf(&b, "- Release: %s\n", attrs.ReleaseType)
				if attrs.Service != "" {
					fmt.Fprintf(&b, "- Streaming on: %s\n", attrs.Service)
				}
			}
		}
		b.WriteString("\n")
	}

	return b.String()
}
