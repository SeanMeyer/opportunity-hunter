package performing

import (
	"fmt"
	"strings"

	"github.com/seanmeyer/opportunity-hunter/core"
)

type performingNotifyFormatter struct{}

func (f *performingNotifyFormatter) FormatPicks(ctx core.NotifyContext) []core.NotifyAction {
	if len(ctx.Picks) == 0 {
		return nil
	}

	var desc strings.Builder
	for _, pick := range ctx.Picks {
		// Find the matching opportunity.
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
						Title:       "Performing Arts Picks",
						Description: desc.String(),
						Color:       0x9B59B6, // purple
					},
				},
			},
		},
	}
}

func (f *performingNotifyFormatter) FormatReminder(opp core.Opportunity, pick core.Pick, _ core.ReminderType) []core.NotifyAction {
	nextDate := opp.NextShowDate()
	return []core.NotifyAction{
		{
			Type: core.PostMessage,
			Message: core.NotifyMessage{
				Content: fmt.Sprintf("Reminder: **%s** — %s on %s",
					opp.Title, pick.DisplayScore,
					nextDate.Format("Mon Jan 2 at 3:04 PM")),
			},
		},
	}
}

// NotifyFormatter returns the performing arts notification formatter.
func (h *PerformingHunt) NotifyFormatter() core.NotifyFormatter {
	return &performingNotifyFormatter{}
}
