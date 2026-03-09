package movies

import (
	"fmt"
	"strings"

	"github.com/seanmeyer/opportunity-hunter/core"
)

type moviesCardRenderer struct{}

func (r *moviesCardRenderer) RenderCard(opp core.Opportunity, pick core.Pick, venue core.Venue) core.CardData {
	card := core.CardData{
		Title:    opp.Title,
		Subtitle: opp.Subtitle,
		Score:    pick.DisplayScore,
		Reason:   pick.Reason,
		Urgency:  pick.Urgency,
	}

	switch {
	case pick.Score >= 0.7:
		card.ScoreTier = core.ScoreHigh
	case pick.Score >= 0.4:
		card.ScoreTier = core.ScoreMedium
	default:
		card.ScoreTier = core.ScoreLow
	}

	if opp.Attributes != nil {
		attrs, err := DecodeMovieAttrs(opp.Attributes)
		if err == nil {
			if len(attrs.Genre) > 0 {
				card.Fields = append(card.Fields, core.CardField{
					Icon: "🎬", Label: "Genre", Value: strings.Join(attrs.Genre, ", "),
				})
			}
			if attrs.Director != "" {
				card.Fields = append(card.Fields, core.CardField{
					Icon: "🎥", Label: "Director", Value: attrs.Director,
				})
			}
			if attrs.TMDBRating > 0 {
				card.Fields = append(card.Fields, core.CardField{
					Icon: "⭐", Label: "TMDB", Value: fmt.Sprintf("%.1f/10", attrs.TMDBRating),
				})
			}
			card.Fields = append(card.Fields, core.CardField{
				Icon: "📽️", Label: "Release", Value: attrs.ReleaseType,
			})
			if attrs.Service != "" {
				card.Fields = append(card.Fields, core.CardField{
					Icon: "📺", Label: "Streaming", Value: attrs.Service,
				})
			}
		}
	}

	if venue.Name != "" {
		card.Fields = append(card.Fields, core.CardField{
			Icon: "📍", Label: "Venue", Value: venue.Name,
		})
	}

	if opp.TicketURL != "" {
		card.ActionURL = opp.TicketURL
		card.ActionLabel = "Get Tickets"
	}

	return card
}

func (h *MoviesHunt) CardRenderer() core.CardRenderer {
	return &moviesCardRenderer{}
}

func (h *MoviesHunt) FeedbackOptions() []core.FeedbackOption {
	return []core.FeedbackOption{
		{Value: "loved", Label: "Loved it"},
		{Value: "good", Label: "Good"},
		{Value: "meh", Label: "Meh"},
		{Value: "not_for_me", Label: "Not for me"},
	}
}
