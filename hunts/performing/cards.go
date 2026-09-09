package performing

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
		return core.FormatListingTime(opp.StartTime)
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

type performingCardRenderer struct{}

func (r *performingCardRenderer) RenderCard(opp core.Opportunity, pick core.Pick, venue core.Venue) core.CardData {
	card := core.CardData{
		Title:       opp.Title,
		Subtitle:    opp.Subtitle,
		Score:       pick.DisplayScore,
		Reason:      pick.Reason,
		Urgency:     pick.Urgency,
		SortScore:   pick.Score,
		DateSort:    opp.StartTime.Unix(),
		DateDisplay: formatDateDisplay(opp),
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
	if venue.WalkingMinutes > 0 || venue.DrivingMinutes > 0 {
		var distVal, icon string
		if venue.WalkingMinutes > 0 && (venue.WalkingMinutes <= 30 || venue.DrivingMinutes == 0) {
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

func (h *PerformingHunt) WebConfig() core.WebConfig {
	return core.WebConfig{
		SortOptions: []core.SortOption{
			{Value: core.SortByScore, Label: "Score (high to low)"},
			{Value: core.SortByDate, Label: "Date (soonest)"},
		},
		FilterOptions: []core.FilterOption{
			{Value: "8", Label: "8+ only"},
			{Value: "7", Label: "7+ only"},
		},
		DefaultSort: core.SortByScore,
	}
}

// FeedbackOptions returns the performing arts feedback options.
func (h *PerformingHunt) FeedbackOptions() []core.FeedbackOption {
	return []core.FeedbackOption{
		{Value: "loved", Label: "Loved it"},
		{Value: "not_for_me", Label: "Not for me"},
		{Value: "already_seen", Label: "Already seen"},
	}
}
