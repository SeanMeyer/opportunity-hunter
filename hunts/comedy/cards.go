package comedy

import (
	"fmt"
	"strings"

	"github.com/seanmeyer/opportunity-hunter/core"
)

// formatDateDisplay formats an opportunity's dates for card display.
//   - 1 date (or no ShowDates): "Sat Mar 28, 6:30 PM"
//   - 2-3 dates: "Fri Mar 27, Sat Mar 28, Sun Mar 29"
//   - 4+ dates: "Mar 27 – Apr 15 (12 shows)"
func formatDateDisplay(opp core.Opportunity) string {
	if len(opp.ShowDates) <= 1 {
		return opp.StartTime.Format("Mon Jan 2, 3:04 PM")
	}
	if len(opp.ShowDates) <= 3 {
		var parts []string
		for _, d := range opp.ShowDates {
			parts = append(parts, d.Format("Mon Jan 2"))
		}
		return strings.Join(parts, ", ")
	}
	first := opp.ShowDates[0]
	last := opp.ShowDates[len(opp.ShowDates)-1]
	return fmt.Sprintf("%s – %s (%d shows)", first.Format("Jan 2"), last.Format("Jan 2"), len(opp.ShowDates))
}

type comedyCardRenderer struct{}

func (r *comedyCardRenderer) RenderCard(opp core.Opportunity, pick core.Pick, venue core.Venue) core.CardData {
	card := core.CardData{
		Title:       opp.Title,
		Subtitle:    venue.Name,
		Score:       pick.DisplayScore,
		Reason:      pick.Reason,
		Urgency:     pick.Urgency,
		SortScore:   pick.Score,
		DateSort:    opp.StartTime.Unix(),
		DateDisplay: formatDateDisplay(opp),
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
	if venue.WalkingMinutes > 0 || venue.DrivingMinutes > 0 {
		var distVal, icon string
		if venue.WalkingMinutes > 0 && venue.WalkingMinutes <= 30 {
			icon = "🚶"
			distVal = fmt.Sprintf("%d min walk", venue.WalkingMinutes)
		} else {
			icon = "🚗"
			distVal = fmt.Sprintf("%d min drive", venue.DrivingMinutes)
		}
		if venue.DistanceMi > 0 {
			distVal += fmt.Sprintf(" · %.1f mi", venue.DistanceMi)
		}
		card.Fields = append(card.Fields, core.CardField{
			Icon: icon, Label: "Distance", Value: distVal,
		})
	}
	card.Fields = append(card.Fields, core.CardField{
		Icon: "📅", Label: "Date", Value: card.DateDisplay,
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
