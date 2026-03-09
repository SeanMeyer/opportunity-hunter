package comedy

import (
	"fmt"
	"strings"

	"github.com/seanmeyer/opportunity-hunter/core"
)

type comedyNotifyFormatter struct{}

func (f *comedyNotifyFormatter) FormatPicks(ctx core.NotifyContext) []core.NotifyAction {
	if len(ctx.Picks) == 0 {
		return nil
	}

	var desc strings.Builder
	for _, pick := range ctx.Picks {
		var opp core.Opportunity
		for _, o := range ctx.Opportunities {
			if o.ID == pick.OpportunityID {
				opp = o
				break
			}
		}
		fmt.Fprintf(&desc, "**%s** — %s\n", opp.Title, pick.DisplayScore)
		if pick.Reason != "" {
			fmt.Fprintf(&desc, "%s\n", pick.Reason)
		}
		if pick.Urgency != "" {
			fmt.Fprintf(&desc, "_%s_\n", pick.Urgency)
		}
		desc.WriteString("\n")
	}

	return []core.NotifyAction{
		{
			Type: core.PostMessage,
			Message: core.NotifyMessage{
				Embeds: []core.Embed{
					{
						Title:       "Comedy Picks This Week",
						Description: desc.String(),
						Color:       0xF1C40F, // gold
					},
				},
			},
		},
	}
}

func (f *comedyNotifyFormatter) FormatReminder(opp core.Opportunity, pick core.Pick, _ core.ReminderType) []core.NotifyAction {
	return []core.NotifyAction{
		{
			Type: core.PostMessage,
			Message: core.NotifyMessage{
				Content: fmt.Sprintf("Reminder: **%s** — %s tomorrow at %s",
					opp.Title, pick.DisplayScore,
					opp.StartTime.Format("3:04 PM")),
			},
		},
	}
}

func (h *ComedyHunt) NotifyFormatter() core.NotifyFormatter {
	return &comedyNotifyFormatter{}
}
