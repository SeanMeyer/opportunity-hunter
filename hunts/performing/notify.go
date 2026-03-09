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
	return []core.NotifyAction{
		{
			Type: core.PostMessage,
			Message: core.NotifyMessage{
				Content: fmt.Sprintf("Reminder: **%s** — %s on %s",
					opp.Title, pick.DisplayScore,
					opp.StartTime.Format("Mon Jan 2 at 3:04 PM")),
			},
		},
	}
}

// NotifyFormatter returns the performing arts notification formatter.
func (h *PerformingHunt) NotifyFormatter() core.NotifyFormatter {
	return &performingNotifyFormatter{}
}

// GroupForEval groups opportunities by week for batch evaluation.
func (h *PerformingHunt) GroupForEval(items []core.Opportunity) []core.Group {
	weeks := make(map[string][]core.Opportunity)
	for _, opp := range items {
		year, week := opp.StartTime.ISOWeek()
		key := fmt.Sprintf("%d-W%02d", year, week)
		weeks[key] = append(weeks[key], opp)
	}

	var groups []core.Group
	for key, opps := range weeks {
		groups = append(groups, core.Group{
			Key:           key,
			Opportunities: opps,
		})
	}
	return groups
}
