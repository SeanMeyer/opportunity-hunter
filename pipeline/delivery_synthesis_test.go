package pipeline

import (
	"context"
	"testing"

	"github.com/seanmeyer/opportunity-hunter/core"
	"github.com/seanmeyer/opportunity-hunter/hunts/fake"
	"github.com/seanmeyer/opportunity-hunter/testutil"
)

type savedSynthesisFormatter struct{}

func (savedSynthesisFormatter) FormatPicks(c core.NotifyContext) []core.NotifyAction {
	return []core.NotifyAction{{Type: core.PostMessage, Message: core.NotifyMessage{Content: c.Synthesis}}}
}
func (savedSynthesisFormatter) FormatReminder(core.Opportunity, core.Pick, core.ReminderType) []core.NotifyAction {
	return nil
}

func TestDeliveryPreservesOlderSynthesisForSameGroup(t *testing.T) {
	ctx := context.Background()
	db := testutil.NewTestDB(t)
	for _, text := range []string{"old briefing", "new briefing"} {
		if _, err := db.SaveEvaluatedGroup(ctx, core.Evaluation{HuntName: "test", GroupKey: "same"}, nil, []core.Opportunity{}); err != nil {
			t.Fatal(err)
		}
		if err := db.SaveDeliverySynthesis(ctx, "test", map[string]string{"same": text}); err != nil {
			t.Fatal(err)
		}
	}
	pending, err := db.PendingDeliveries(ctx, "test")
	if err != nil || len(pending) != 2 {
		t.Fatalf("pending=%v err=%v", pending, err)
	}
	if pending[0].Context.Synthesis != "old briefing" || pending[1].Context.Synthesis != "new briefing" {
		t.Errorf("saved briefings changed: %q, %q", pending[0].Context.Synthesis, pending[1].Context.Synthesis)
	}
	n := &testutil.FakeNotifier{}
	p := New(db, core.NewCostTracker(0, nil), n, core.ScanRegion{}, "")
	h := &fake.FakeHunt{HuntName: "test", NotifyFmtFn: func() core.NotifyFormatter { return savedSynthesisFormatter{} }}
	result := core.HuntResult{}
	p.deliverPending(ctx, h, core.HuntCapabilities{HasNotifyHunt: true}, map[string]string{"same": "latest briefing"}, map[int64]bool{}, &result)
	if len(n.Actions) != 2 {
		t.Fatalf("actions=%v errors=%v", n.Actions, result.Errors)
	}
	if n.Actions[0].Message.Content != "old briefing" || n.Actions[1].Message.Content != "new briefing" {
		t.Fatalf("delivery replaced saved briefings: %+v", n.Actions)
	}
}
