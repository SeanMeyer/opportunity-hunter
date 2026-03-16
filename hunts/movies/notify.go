package movies

import (
	"fmt"
	"strings"

	"github.com/seanmeyer/opportunity-hunter/core"
)

type moviesNotifyFormatter struct{}

func (f *moviesNotifyFormatter) FormatPicks(ctx core.NotifyContext) []core.NotifyAction {
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
						Title:       "Movie Picks",
						Description: desc.String(),
						Color:       0xE74C3C, // red
					},
				},
			},
		},
	}
}

func (f *moviesNotifyFormatter) FormatReminder(opp core.Opportunity, pick core.Pick, _ core.ReminderType) []core.NotifyAction {
	attrs, _ := DecodeMovieAttrs(opp.Attributes)
	if attrs.ReleaseType == "streaming" {
		return nil // no reminders for streaming
	}
	return []core.NotifyAction{
		{
			Type: core.PostMessage,
			Message: core.NotifyMessage{
				Content: fmt.Sprintf("Reminder: **%s** — %s opens tomorrow",
					opp.Title, pick.DisplayScore),
			},
		},
	}
}

func (h *MoviesHunt) NotifyFormatter() core.NotifyFormatter {
	return &moviesNotifyFormatter{}
}
