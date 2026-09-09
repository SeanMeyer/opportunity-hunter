package powder

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/seanmeyer/opportunity-hunter/core"
	"github.com/seanmeyer/opportunity-hunter/hunts/powder/weather"
)

func tierDisplayName(tier weather.Tier) string {
	switch weather.NormalizeTier(tier) {
	case weather.TierDropEverything:
		return "Drop Everything"
	case weather.TierRecommended:
		return "Recommended"
	case weather.TierWatch:
		return "Watch"
	case weather.TierSkip:
		return "Skip"
	default:
		return string(tier)
	}
}

type powderCardRenderer struct{}

func (r *powderCardRenderer) RenderCard(opp core.Opportunity, pick core.Pick, venue core.Venue) core.CardData {
	// Lead with the overall judgment; use the old summary only as a fallback.
	reason := pick.Reason
	var rich map[string]any
	if pick.Attributes != nil {
		_ = json.Unmarshal(pick.Attributes, &rich)
	}
	if reason == "" && rich != nil {
		if s, ok := rich["summary"].(string); ok && s != "" {
			reason = s
		}
	}

	card := core.CardData{
		Title:     opp.Title,
		Subtitle:  opp.Subtitle,
		Score:     tierDisplayName(weather.Tier(pick.DisplayScore)),
		Reason:    reason,
		Urgency:   pick.Urgency,
		SortScore: weather.TierScore(weather.Tier(pick.DisplayScore)),
	}

	switch weather.NormalizeTier(weather.Tier(pick.DisplayScore)) {
	case weather.TierDropEverything:
		card.ScoreTier = core.ScoreHigh
	case weather.TierRecommended:
		card.ScoreTier = core.ScoreMedium
	case weather.TierSkip:
		card.ScoreTier = core.ScoreNone
	default:
		card.ScoreTier = core.ScoreLow
	}

	// Extract basic attrs.
	if opp.Attributes != nil {
		attrs, err := DecodePowderAttrs(opp.Attributes)
		if err == nil {
			if attrs.SnowfallIn > 0 {
				card.Fields = append(card.Fields, core.CardField{
					Icon: "\xe2\x9d\x84\xef\xb8\x8f", Label: "Snowfall", Value: fmt.Sprintf("%.0f inches", attrs.SnowfallIn),
				})
			}
			if attrs.FrictionTier != "" {
				card.Fields = append(card.Fields, core.CardField{
					Icon: "\xf0\x9f\x9a\x97", Label: "Friction", Value: attrs.FrictionTier,
				})
			}
		}
	}

	// rich was already parsed above for summary extraction.
	if rich != nil {
		if s, ok := rich["strategy"].(string); ok && s != "" {
			card.Fields = append(card.Fields, core.CardField{
				Icon: "\xf0\x9f\x8e\xaf", Label: "Strategy", Value: s,
			})
		}
		if s, ok := rich["snow_quality"].(string); ok && s != "" {
			card.Fields = append(card.Fields, core.CardField{
				Icon: "\xe2\x9d\x84\xef\xb8\x8f", Label: "Snow Quality", Value: s,
			})
		}
		if s, ok := rich["crowd_estimate"].(string); ok && s != "" {
			card.Fields = append(card.Fields, core.CardField{
				Icon: "\xf0\x9f\x91\xa5", Label: "Crowds", Value: s,
			})
		}
		if s, ok := rich["best_ski_day"].(string); ok && s != "" {
			val := s
			if reason, ok := rich["best_ski_day_reason"].(string); ok && reason != "" {
				val += " — " + reason
			}
			card.Fields = append(card.Fields, core.CardField{
				Icon: "\xf0\x9f\x93\x85", Label: "Best Day", Value: val,
			})
		}

		// Day-by-day summary.
		if dbd, ok := rich["day_by_day"].([]any); ok {
			var parts []string
			for _, item := range dbd {
				if entry, ok := item.(map[string]any); ok {
					date, _ := entry["date"].(string)
					snow, _ := entry["snowfall"].(string)
					cond, _ := entry["conditions"].(string)
					if snow != "" && snow != "0" && snow != "Trace" {
						parts = append(parts, fmt.Sprintf("%s: %s — %s", date, snow, cond))
					}
				}
			}
			if len(parts) > 0 {
				card.Fields = append(card.Fields, core.CardField{
					Icon: "\xf0\x9f\x93\x8a", Label: "Day by Day", Value: strings.Join(parts, "\n"),
				})
			}
		}

		// Resort insights.
		if insights, ok := rich["resort_insights"].([]any); ok && len(insights) > 0 {
			var insightParts []string
			for _, item := range insights {
				if entry, ok := item.(map[string]any); ok {
					resort, _ := entry["resort"].(string)
					insight, _ := entry["insight"].(string)
					if resort != "" && insight != "" {
						insightParts = append(insightParts, fmt.Sprintf("%s: %s", resort, insight))
					}
				}
			}
			if len(insightParts) > 0 {
				card.Fields = append(card.Fields, core.CardField{
					Icon: "\xf0\x9f\x8f\x94\xef\xb8\x8f", Label: "Resort Insights", Value: strings.Join(insightParts, "\n"),
				})
			}
		}

		// Logistics cost.
		if lod, ok := rich["logistics"].(map[string]any); ok {
			if s, ok := lod["TotalEstimatedCost"].(string); ok && s != "" && s != "N/A" {
				card.Fields = append(card.Fields, core.CardField{
					Icon: "\xf0\x9f\x92\xb0", Label: "Est. Cost", Value: s,
				})
			}
		}
	}

	if !opp.StartTime.IsZero() && opp.EndTime != nil {
		card.Fields = append(card.Fields, core.CardField{
			Icon: "\xf0\x9f\x93\x85", Label: "Window",
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
