package comedy

import (
	"fmt"

	"github.com/seanmeyer/opportunity-hunter/core"
)

type comedyCardRenderer struct{}

func (r *comedyCardRenderer) RenderCard(opp core.Opportunity, pick core.Pick, venue core.Venue) core.CardData {
	card := core.CardData{
		Title:    opp.Title,
		Subtitle: venue.Name,
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

	// Sell-out risk from attrs.
	if opp.Attributes != nil {
		attrs, err := DecodeComedyAttrs(opp.Attributes)
		if err == nil && attrs.SellOutRisk != "" {
			card.Fields = append(card.Fields, core.CardField{
				Icon: "🔥", Label: "Sell-out Risk", Value: attrs.SellOutRisk,
			})
		}
	}

	if opp.TicketURL != "" {
		card.ActionURL = opp.TicketURL
		card.ActionLabel = "Get Tickets"
	}

	return card
}

func (h *ComedyHunt) CardRenderer() core.CardRenderer {
	return &comedyCardRenderer{}
}

func (h *ComedyHunt) FeedbackOptions() []core.FeedbackOption {
	return []core.FeedbackOption{
		{Value: "loved", Label: "Loved it"},
		{Value: "not_for_me", Label: "Not for me"},
	}
}
