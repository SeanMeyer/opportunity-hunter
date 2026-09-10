package powder

import (
	"github.com/seanmeyer/opportunity-hunter/core"
	"testing"
)

func TestUpdatesAlwaysRouteToForumThread(t *testing.T) {
	for _, thread := range []string{"", "existing"} {
		for _, change := range []string{"material", "minor", "downgrade"} {
			ctx := core.NotifyContext{ExistingThreadID: thread, Opportunities: []core.Opportunity{{ID: 1, Title: "Storm"}}, Picks: []core.Pick{{OpportunityID: 1, DisplayScore: "RECOMMENDED", Reason: "changed", Attributes: core.Attributes(`{"change_class":"` + change + `"}`)}}}
			actions := (&powderNotifyFormatter{}).FormatPicks(ctx)
			if len(actions) == 0 {
				t.Fatal("lost update")
			}
			if err := core.ValidateActions(actions); err != nil {
				t.Fatal(err)
			}
			for _, a := range actions {
				if a.Type == core.PostMessage {
					t.Fatalf("unrouted %s update", change)
				}
				if thread != "" && (a.Type != core.PostToThread || a.ThreadID != thread) {
					t.Fatal("update missed existing thread")
				}
			}
		}
	}
}
