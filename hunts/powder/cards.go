package powder

import (
	"fmt"

	"github.com/seanmeyer/opportunity-hunter/core"
)

type powderCardRenderer struct{}

func (r *powderCardRenderer) RenderCard(opp core.Opportunity, pick core.Pick, venue core.Venue) core.CardData {
	card := core.CardData{
		Title:    opp.Title,
		Subtitle: opp.Subtitle,
		Score:    pick.DisplayScore,
		Reason:   pick.Reason,
		Urgency:  pick.Urgency,
	}

	switch pick.DisplayScore {
	case string(TierDropEverything):
		card.ScoreTier = core.ScoreHigh
	case string(TierWorthALook):
		card.ScoreTier = core.ScoreMedium
	default:
		card.ScoreTier = core.ScoreLow
	}

	if opp.Attributes != nil {
		attrs, err := DecodePowderAttrs(opp.Attributes)
		if err == nil {
			if attrs.SnowfallIn > 0 {
				card.Fields = append(card.Fields, core.CardField{
					Icon: "❄️", Label: "Snowfall", Value: fmt.Sprintf("%.0f inches", attrs.SnowfallIn),
				})
			}
			if attrs.FrictionTier != "" {
				card.Fields = append(card.Fields, core.CardField{
					Icon: "🚗", Label: "Friction", Value: attrs.FrictionTier,
				})
			}
			if attrs.Consensus > 0 {
				card.Fields = append(card.Fields, core.CardField{
					Icon: "📊", Label: "Consensus", Value: fmt.Sprintf("%.0f%%", attrs.Consensus*100),
				})
			}
		}
	}

	if opp.StartTime.IsZero() == false && opp.EndTime != nil {
		card.Fields = append(card.Fields, core.CardField{
			Icon: "📅", Label: "Window",
			Value: fmt.Sprintf("%s — %s", opp.StartTime.Format("Jan 2"), opp.EndTime.Format("Jan 2")),
		})
	}

	return card
}

func (h *PowderHunt) CardRenderer() core.CardRenderer {
	return &powderCardRenderer{}
}

func (h *PowderHunt) FeedbackOptions() []core.FeedbackOption {
	return []core.FeedbackOption{
		{Value: "went_great", Label: "Went great"},
		{Value: "went_ok", Label: "Went ok"},
		{Value: "skipped", Label: "Skipped"},
	}
}
