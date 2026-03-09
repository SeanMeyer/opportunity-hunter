package powder

import (
	"fmt"
	"strings"

	"github.com/seanmeyer/opportunity-hunter/core"
)

type powderNotifyFormatter struct{}

func (f *powderNotifyFormatter) FormatPicks(ctx core.NotifyContext) []core.NotifyAction {
	if len(ctx.Picks) == 0 {
		return nil
	}

	// Create thread with briefing, then post details per region.
	threadName := buildThreadName(ctx)

	var actions []core.NotifyAction

	// Thread creation with briefing.
	briefing := ctx.Synthesis
	if briefing == "" {
		briefing = "Storm activity detected"
	}
	actions = append(actions, core.NotifyAction{
		Type:       core.CreateThread,
		ThreadName: threadName,
		Message: core.NotifyMessage{
			Embeds: []core.Embed{
				{
					Title:       threadName,
					Description: briefing,
					Color:       0x3498DB, // blue
				},
			},
		},
	})

	// Detail posts per region.
	for _, pick := range ctx.Picks {
		var opp core.Opportunity
		for _, o := range ctx.Opportunities {
			if o.ID == pick.OpportunityID {
				opp = o
				break
			}
		}

		var desc strings.Builder
		fmt.Fprintf(&desc, "**%s** — %s\n", opp.Title, pick.DisplayScore)
		if pick.Reason != "" {
			fmt.Fprintf(&desc, "%s\n", pick.Reason)
		}

		actions = append(actions, core.NotifyAction{
			Type:      core.PostToThread,
			ThreadRef: threadName,
			Message: core.NotifyMessage{
				Embeds: []core.Embed{
					{
						Title:       opp.Title,
						Description: desc.String(),
						Color:       tierColor(pick.DisplayScore),
					},
				},
			},
		})
	}

	return actions
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

func tierColor(displayScore string) int {
	switch displayScore {
	case string(TierDropEverything):
		return 0xE74C3C // red
	case string(TierWorthALook):
		return 0xF1C40F // gold
	default:
		return 0x95A5A6 // gray
	}
}
