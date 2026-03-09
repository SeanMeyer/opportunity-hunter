package performing

import (
	"fmt"

	"github.com/seanmeyer/opportunity-hunter/core"
)

type performingCardRenderer struct{}

func (r *performingCardRenderer) RenderCard(opp core.Opportunity, pick core.Pick, venue core.Venue) core.CardData {
	card := core.CardData{
		Title:    opp.Title,
		Subtitle: opp.Subtitle,
		Score:    pick.DisplayScore,
		Reason:   pick.Reason,
		Urgency:  pick.Urgency,
	}

	// Score tier based on normalized score.
	switch {
	case pick.Score >= 0.7:
		card.ScoreTier = core.ScoreHigh
	case pick.Score >= 0.4:
		card.ScoreTier = core.ScoreMedium
	default:
		card.ScoreTier = core.ScoreLow
	}

	// Fields.
	if venue.Name != "" {
		card.Fields = append(card.Fields, core.CardField{
			Icon: "📍", Label: "Venue", Value: venue.Name,
		})
	}
	card.Fields = append(card.Fields, core.CardField{
		Icon: "📅", Label: "Date", Value: opp.StartTime.Format("Mon Jan 2, 3:04 PM"),
	})
	if opp.PriceMin != nil {
		price := fmt.Sprintf("$%.0f", *opp.PriceMin)
		if opp.PriceMax != nil {
			price = fmt.Sprintf("$%.0f - $%.0f", *opp.PriceMin, *opp.PriceMax)
		}
		card.Fields = append(card.Fields, core.CardField{
			Icon: "💰", Label: "Price", Value: price,
		})
	}

	// Decode performing attrs for genre.
	if opp.Attributes != nil {
		attrs, err := DecodePerformingAttrs(opp.Attributes)
		if err == nil && attrs.Genre != "" {
			card.Fields = append(card.Fields, core.CardField{
				Icon: "🎭", Label: "Genre", Value: attrs.Genre,
			})
		}
	}

	if opp.TicketURL != "" {
		card.ActionURL = opp.TicketURL
		card.ActionLabel = "Get Tickets"
	}

	return card
}

// CardRenderer returns the performing arts card renderer.
func (h *PerformingHunt) CardRenderer() core.CardRenderer {
	return &performingCardRenderer{}
}

// FeedbackOptions returns the performing arts feedback options.
func (h *PerformingHunt) FeedbackOptions() []core.FeedbackOption {
	return []core.FeedbackOption{
		{Value: "loved", Label: "Loved it"},
		{Value: "not_for_me", Label: "Not for me"},
		{Value: "already_seen", Label: "Already seen"},
	}
}
