package powder

import (
	"fmt"
	"strings"

	"github.com/seanmeyer/opportunity-hunter/core"
)

func buildPrompt(ec core.EvalContext) string {
	var b strings.Builder

	b.WriteString(`You are a powder skiing advisor. Evaluate the following storm windows and rate each region.

## Tier System
- **DROP EVERYTHING**: Historic storm, high confidence, minimal friction. Go.
- **WORTH A LOOK**: Significant snowfall, good conditions, reasonable logistics.
- **ON THE RADAR**: Notable storm worth watching. May improve or fizzle.

`)

	if ec.Preferences != "" {
		fmt.Fprintf(&b, "## User Profile\n\n%s\n\n", ec.Preferences)
	}

	b.WriteString("## Storm Windows to Evaluate\n\n")
	for i, opp := range ec.Opportunities {
		fmt.Fprintf(&b, "### Region %d: %s\n", i+1, opp.Title)
		fmt.Fprintf(&b, "- Window: %s\n", opp.Subtitle)

		if opp.Attributes != nil {
			attrs, err := DecodePowderAttrs(opp.Attributes)
			if err == nil {
				if attrs.SnowfallIn > 0 {
					fmt.Fprintf(&b, "- Expected snowfall: %.0f inches\n", attrs.SnowfallIn)
				}
				if attrs.Consensus > 0 {
					fmt.Fprintf(&b, "- Model consensus: %.0f%%\n", attrs.Consensus*100)
				}
				fmt.Fprintf(&b, "- Friction: %s\n", attrs.FrictionTier)
			}
		}
		b.WriteString("\n")
	}

	return b.String()
}
