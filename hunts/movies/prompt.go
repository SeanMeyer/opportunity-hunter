package movies

import (
	"fmt"
	"math"
	"strings"

	"github.com/seanmeyer/opportunity-hunter/core"
	"github.com/seanmeyer/opportunity-hunter/hunts/movies/catalog"
)

func buildPrompt(ec core.EvalContext, theaters []catalog.Theater, homeLat, homeLon float64) string {
	var b strings.Builder

	b.WriteString(`You are a movie recommendation engine for Denver, CO. Evaluate movies based on the user's taste AND the viewing experience (theater quality, pricing, convenience).

`)

	if ec.Preferences != "" {
		fmt.Fprintf(&b, "## User Preferences & Scoring Guidance\n\n%s\n\n", ec.Preferences)
	}

	b.WriteString("## Feedback on past recommendations\n\n" + core.FormatFeedback(ec.Feedback) + "\n\n")

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
		// Add venue/distance context for theatrical showings.
		if opp.VenueID != nil {
			if venue, ok := ec.Venues[*opp.VenueID]; ok {
				venueLine := venue.Name
				if venue.WalkingMinutes > 0 {
					venueLine += fmt.Sprintf(" (%d min walk)", venue.WalkingMinutes)
				}
				fmt.Fprintf(&b, "- Showing at: %s\n", venueLine)
			}
		}
		b.WriteString("\n")
	}

	// Include theater catalog as context for theatrical recommendations.
	if len(theaters) > 0 {
		b.WriteString("## Nearby Theaters\n\n")
		b.WriteString("Use this context when recommending theatrical movies. Match films to the best viewing experience.\n\n")
		for _, t := range theaters {
			distMi := haversineMi(homeLat, homeLon, t.Coords.Lat, t.Coords.Lon)
			distLabel := formatDistance(distMi)
			fmt.Fprintf(&b, "**%s** (%s)\n", t.Name, distLabel)
			if v, ok := t.Metadata["pricing"]; ok {
				fmt.Fprintf(&b, "- Pricing: %s\n", v)
			}
			if v, ok := t.Metadata["screens"]; ok {
				fmt.Fprintf(&b, "- Screens: %s\n", v)
			}
			if v, ok := t.Metadata["vibe"]; ok {
				fmt.Fprintf(&b, "- Vibe: %s\n", v)
			}
			if v, ok := t.Metadata["best_for"]; ok {
				fmt.Fprintf(&b, "- Best for: %s\n", v)
			}
			b.WriteString("\n")
		}
	}

	b.WriteString(`## Instructions

For each movie that scores 7+, provide:
- movie_id: index from the list above
- score: 1-10 rating
- reason: why this is worth seeing (taste match, critical reception, viewing experience)
- urgency: what the user should do — reference specific nearby theaters by name with distance (e.g., "see it in RPX at Regal Denver Pavilions, 5 min walk", "catch $8 Terror Tuesday at Alamo Sloans Lake")

If no movies score 7+, return an empty picks list.
Explain in skipped_reasoning why the remaining movies didn't make the cut.
`)

	return b.String()
}

// haversineMi returns the straight-line distance in miles between two coordinates.
func haversineMi(lat1, lon1, lat2, lon2 float64) float64 {
	const earthRadiusMi = 3958.8
	dLat := (lat2 - lat1) * math.Pi / 180
	dLon := (lon2 - lon1) * math.Pi / 180
	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(lat1*math.Pi/180)*math.Cos(lat2*math.Pi/180)*
			math.Sin(dLon/2)*math.Sin(dLon/2)
	return earthRadiusMi * 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
}

// formatDistance returns a human-friendly distance string.
func formatDistance(miles float64) string {
	if miles < 1.0 {
		return fmt.Sprintf("%.1f mi walk", miles)
	}
	if miles < 5.0 {
		return fmt.Sprintf("%.1f mi", miles)
	}
	return fmt.Sprintf("%.0f mi drive", miles)
}
