package powder

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/seanmeyer/opportunity-hunter/core"
	"github.com/seanmeyer/opportunity-hunter/hunts/powder/weather"
)

const (
	colorDropEverything = 0xFF0000 // red
	colorWorthALook     = 0xFF8C00 // orange
	colorOnTheRadar     = 0x4169E1 // blue
)

type powderNotifyFormatter struct{}

func (f *powderNotifyFormatter) FormatPicks(ctx core.NotifyContext) []core.NotifyAction {
	if len(ctx.Picks) == 0 {
		return nil
	}

	// Separate new evaluations from updates.
	var newPicks, updatePicks []core.Pick
	for _, pick := range ctx.Picks {
		cc := changeClassFromPick(pick)
		if cc == string(weather.ChangeNew) || cc == "" {
			newPicks = append(newPicks, pick)
		} else {
			updatePicks = append(updatePicks, pick)
		}
	}

	var actions []core.NotifyAction

	// Format new evaluations with full thread + embeds.
	if len(newPicks) > 0 {
		actions = append(actions, f.formatNewPicks(ctx, newPicks)...)
	}

	// Format updates based on change class.
	for _, pick := range updatePicks {
		actions = append(actions, f.formatUpdatePick(ctx, pick)...)
	}

	return actions
}

func (f *powderNotifyFormatter) formatNewPicks(ctx core.NotifyContext, picks []core.Pick) []core.NotifyAction {
	threadName := buildThreadName(ctx)
	highestTier := f.highestTierFromPicks(picks)
	emoji := tierEmoji(highestTier)

	var actions []core.NotifyAction

	briefing := ctx.Synthesis
	if briefing == "" {
		briefing = "Storm activity detected"
	}

	threadTitle := fmt.Sprintf("%s %s", emoji, threadName)

	// Use existing thread ID if available.
	if ctx.ExistingThreadID != "" {
		threadTitle = ctx.ExistingThreadID
	}

	pingContent := briefing
	var ping bool
	if highestTier == string(weather.TierDropEverything) {
		pingContent = "@here\n" + briefing
		ping = true
	}

	if ctx.ExistingThreadID == "" {
		actions = append(actions, core.NotifyAction{
			Type:       core.CreateThread,
			ThreadName: threadTitle,
			Ping:       ping,
			Message: core.NotifyMessage{
				Content: pingContent,
				Embeds: []core.Embed{
					{
						Title:       threadTitle,
						Description: briefing,
						Color:       tierColorFromString(highestTier),
					},
				},
			},
		})
	}

	threadRef := threadTitle
	for _, pick := range picks {
		opp := f.findOpp(ctx, pick)
		embed := buildDetailEmbed(opp, pick)
		embed.Title = fmt.Sprintf("%s %s", tierEmoji(pick.DisplayScore), opp.Title)

		actions = append(actions, core.NotifyAction{
			Type:      core.PostToThread,
			ThreadRef: threadRef,
			Message: core.NotifyMessage{
				Content: getSummary(pick),
				Embeds:  []core.Embed{embed},
			},
		})
	}

	return actions
}

func (f *powderNotifyFormatter) formatUpdatePick(ctx core.NotifyContext, pick core.Pick) []core.NotifyAction {
	cc := changeClassFromPick(pick)
	opp := f.findOpp(ctx, pick)

	switch weather.ChangeClass(cc) {
	case weather.ChangeMaterial:
		// Material change: full embed, ping for DROP_EVERYTHING.
		embed := buildDetailEmbed(opp, pick)
		embed.Title = fmt.Sprintf("%s UPDATE: %s", tierEmoji(pick.DisplayScore), opp.Title)
		ping := pick.DisplayScore == string(weather.TierDropEverything)
		return []core.NotifyAction{
			{
				Type: core.PostMessage,
				Ping: ping,
				Message: core.NotifyMessage{
					Content: fmt.Sprintf("**Weather Update** — %s: %s", opp.Title, getSummary(pick)),
					Embeds:  []core.Embed{embed},
				},
			},
		}

	case weather.ChangeDowngrade:
		// Downgrade: brief note, no ping.
		return []core.NotifyAction{
			{
				Type: core.PostMessage,
				Message: core.NotifyMessage{
					Content: fmt.Sprintf("**Downgrade** — %s: %s", opp.Title, getSummary(pick)),
				},
			},
		}

	default:
		// Minor: shorter update, no ping.
		return []core.NotifyAction{
			{
				Type: core.PostMessage,
				Message: core.NotifyMessage{
					Content: fmt.Sprintf("Minor update — %s: %s", opp.Title, getSummary(pick)),
				},
			},
		}
	}
}

func (f *powderNotifyFormatter) findOpp(ctx core.NotifyContext, pick core.Pick) core.Opportunity {
	for _, o := range ctx.Opportunities {
		if o.ID == pick.OpportunityID {
			return o
		}
	}
	return core.Opportunity{}
}

func (f *powderNotifyFormatter) highestTierFromPicks(picks []core.Pick) string {
	best := string(weather.TierOnTheRadar)
	for _, p := range picks {
		if weather.TierRank(weather.Tier(p.DisplayScore)) > weather.TierRank(weather.Tier(best)) {
			best = p.DisplayScore
		}
	}
	return best
}

func changeClassFromPick(pick core.Pick) string {
	var rich map[string]any
	if pick.Attributes != nil {
		_ = json.Unmarshal(pick.Attributes, &rich)
	}
	if rich != nil {
		if cc, ok := rich["change_class"].(string); ok {
			return cc
		}
	}
	return ""
}

func (f *powderNotifyFormatter) FormatReminder(opp core.Opportunity, pick core.Pick, _ core.ReminderType) []core.NotifyAction {
	return []core.NotifyAction{
		{
			Type: core.PostMessage,
			Message: core.NotifyMessage{
				Content: fmt.Sprintf("Reminder: **%s** — %s starting %s",
					opp.Title, pick.DisplayScore,
					opp.StartTime.Format("Mon Jan 2")),
			},
		},
	}
}

func (h *PowderHunt) NotifyFormatter() core.NotifyFormatter {
	return &powderNotifyFormatter{}
}

func buildThreadName(ctx core.NotifyContext) string {
	if len(ctx.Opportunities) > 0 {
		opp := ctx.Opportunities[0]
		return fmt.Sprintf("%s — %s", opp.Title, opp.Subtitle)
	}
	return "Storm Update"
}

func buildDetailEmbed(opp core.Opportunity, pick core.Pick) core.Embed {
	embed := core.Embed{
		Color: tierColorFromString(pick.DisplayScore),
	}

	// Build description from recommendation + key factors.
	var desc strings.Builder
	desc.WriteString(pick.Reason)

	// Try to extract rich data from pick attributes.
	var rich map[string]any
	if pick.Attributes != nil {
		_ = json.Unmarshal(pick.Attributes, &rich)
	}

	if rich != nil {
		pros := stringSliceFromAny(rich["key_factors"])
		if kf, ok := rich["key_factors"].(map[string]any); ok {
			pros = stringSliceFromAny(kf["Pros"])
			cons := stringSliceFromAny(kf["Cons"])
			if len(pros) > 0 {
				desc.WriteString("\n\n**Pros**\n")
				for _, p := range pros {
					desc.WriteString("+ " + p + "\n")
				}
			}
			if len(cons) > 0 {
				desc.WriteString("\n**Cons**\n")
				for _, c := range cons {
					desc.WriteString("- " + c + "\n")
				}
			}
		}
	}
	embed.Description = strings.TrimRight(desc.String(), "\n")

	// Add fields for strategy, insights, quality, etc.
	if rich != nil {
		if s, ok := rich["strategy"].(string); ok && s != "" {
			embed.Fields = append(embed.Fields, core.EmbedField{Name: "Strategy", Value: s})
		}
		if s, ok := rich["snow_quality"].(string); ok && s != "" {
			embed.Fields = append(embed.Fields, core.EmbedField{Name: "Snow Quality", Value: s})
		}
		if s, ok := rich["crowd_estimate"].(string); ok && s != "" {
			embed.Fields = append(embed.Fields, core.EmbedField{Name: "Crowd Estimate", Value: s})
		}
		if s, ok := rich["information_edge"].(string); ok && s != "" {
			embed.Fields = append(embed.Fields, core.EmbedField{Name: "Information Edge", Value: s})
		}
		if s, ok := rich["closure_risk"].(string); ok && s != "" {
			embed.Fields = append(embed.Fields, core.EmbedField{Name: "Closure Risk", Value: s})
		}
		if s, ok := rich["best_ski_day"].(string); ok && s != "" {
			val := s
			if reason, ok := rich["best_ski_day_reason"].(string); ok && reason != "" {
				val += " — " + reason
			}
			embed.Fields = append(embed.Fields, core.EmbedField{Name: "Best Day", Value: val})
		}

		// Day-by-day snowfall summary.
		if dbd, ok := rich["day_by_day"].([]any); ok {
			var parts []string
			for _, item := range dbd {
				if entry, ok := item.(map[string]any); ok {
					date, _ := entry["date"].(string)
					snow, _ := entry["snowfall"].(string)
					if snow != "" && snow != "0" && snow != "Trace" {
						parts = append(parts, fmt.Sprintf("%s: %s", date, snow))
					}
				}
			}
			if len(parts) > 0 {
				embed.Fields = append(embed.Fields, core.EmbedField{
					Name: "Total Snowfall", Value: strings.Join(parts, "\n"),
				})
			}
		}

		// Logistics fields.
		if lod, ok := rich["logistics"].(map[string]any); ok {
			if s, ok := lod["Lodging"].(string); ok && s != "" {
				embed.Fields = append(embed.Fields, core.EmbedField{Name: "Lodging", Value: s})
			}
			if s, ok := lod["Transportation"].(string); ok && s != "" {
				embed.Fields = append(embed.Fields, core.EmbedField{Name: "Getting There", Value: s})
			}
			if s, ok := lod["TotalEstimatedCost"].(string); ok && s != "" && s != "N/A" {
				embed.Fields = append(embed.Fields, core.EmbedField{Name: "Total Est. Cost", Value: s})
			}
		}
	}

	return embed
}

func getSummary(pick core.Pick) string {
	var rich map[string]any
	if pick.Attributes != nil {
		_ = json.Unmarshal(pick.Attributes, &rich)
	}
	if rich != nil {
		if s, ok := rich["summary"].(string); ok && s != "" {
			return s
		}
	}
	return pick.Reason
}

func tierColorFromString(displayScore string) int {
	switch weather.Tier(displayScore) {
	case weather.TierDropEverything:
		return colorDropEverything
	case weather.TierWorthALook:
		return colorWorthALook
	default:
		return colorOnTheRadar
	}
}

func tierEmoji(displayScore string) string {
	switch weather.Tier(displayScore) {
	case weather.TierDropEverything:
		return "\xf0\x9f\x9a\xa8" // 🚨
	case weather.TierWorthALook:
		return "\xf0\x9f\x91\x80" // 👀
	default:
		return "\xf0\x9f\x93\xa1" // 📡
	}
}

func stringSliceFromAny(v any) []string {
	items, ok := v.([]any)
	if !ok {
		return nil
	}
	var result []string
	for _, item := range items {
		if s, ok := item.(string); ok {
			result = append(result, s)
		}
	}
	return result
}
