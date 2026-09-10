package pipeline_test

import (
	"context"
	"errors"
	"fmt"
	"github.com/seanmeyer/opportunity-hunter/core"
	"github.com/seanmeyer/opportunity-hunter/hunts/fake"
	"github.com/seanmeyer/opportunity-hunter/pipeline"
	"github.com/seanmeyer/opportunity-hunter/testutil"
	"testing"
	"time"
)

type expiryPickFormatter struct{}

func (expiryPickFormatter) FormatReminder(core.Opportunity, core.Pick, core.ReminderType) []core.NotifyAction {
	return nil
}
func (expiryPickFormatter) FormatPicks(c core.NotifyContext) []core.NotifyAction {
	var actions []core.NotifyAction
	for _, o := range c.Opportunities {
		actions = append(actions, core.NotifyAction{Type: core.PostMessage, Message: core.NotifyMessage{Content: o.Title}})
	}
	return actions
}

func TestMixedExpiryDeliveryPreservesSafeRetry(t *testing.T) {
	for _, prepared := range []bool{false, true} {
		t.Run(map[bool]string{false: "filter before prepare", true: "retain checkpoint"}[prepared], func(t *testing.T) {
			db := testutil.NewTestDB(t)
			ctx := context.Background()
			var opps []core.Opportunity
			var picks []core.Pick
			for i, offset := range []time.Duration{-48 * time.Hour, 48 * time.Hour} {
				o := core.Opportunity{HuntName: "test", Title: []string{"obsolete", "current"}[i], SourceID: []string{"old", "new"}[i], State: core.Evaluated, StartTime: time.Now().Add(offset)}
				id, err := db.InsertOpportunity(ctx, o)
				if err != nil {
					t.Fatal(err)
				}
				o.ID = id
				opps = append(opps, o)
				picks = append(picks, core.Pick{OpportunityID: id, Score: 1})
			}
			eid, err := db.SaveEvaluatedGroup(ctx, core.Evaluation{HuntName: "test", GroupKey: "mixed"}, picks, opps)
			if err != nil {
				t.Fatal(err)
			}
			if prepared {
				if err := db.PrepareDelivery(ctx, eid, []core.NotifyAction{{Type: core.PostMessage, Message: core.NotifyMessage{Content: "remaining acknowledgement-sensitive message"}}}); err != nil {
					t.Fatal(err)
				}
			}
			h := &fake.FakeHunt{HuntName: "test", Eval: &testutil.FakeEvaluator{}, ExpirerFn: func(o core.Opportunity) bool { return o.StartTime.Before(time.Now()) }, NotifyFmtFn: func() core.NotifyFormatter { return expiryPickFormatter{} }}
			n := &testutil.FakeNotifier{Err: errors.New("offline")}
			r := pipeline.New(db, core.NewCostTracker(0, nil), n, core.ScanRegion{}, "").Run(ctx, h)
			pending, err := db.PendingDeliveries(ctx, "test")
			if err != nil || len(pending) != 1 {
				t.Fatalf("lost queue: %+v %v", pending, err)
			}
			if prepared {
				if len(n.Actions) != 0 || len(r.Errors) == 0 || pending[0].Actions[0].Message.Content != "remaining acknowledgement-sensitive message" {
					t.Fatal("unsafe prepared replay or lost pending action")
				}
				return
			}
			if len(n.Actions) != 1 || n.Actions[0].Message.Content != "current" || len(pending[0].Context.Opportunities) != 1 {
				t.Fatalf("filter failed actions=%+v context=%+v", n.Actions, pending[0].Context)
			}
			live := &testutil.FakeNotifier{}
			r = pipeline.New(db, core.NewCostTracker(0, nil), live, core.ScanRegion{}, "").Run(ctx, h)
			if len(r.Errors) != 0 || len(live.Actions) != 1 || live.Actions[0].Message.Content != "current" {
				t.Fatalf("filtered retry failed: %+v %+v", r, live.Actions)
			}
		})
	}
}

func TestExpiredItemsNeverEvaluatedOrDelivered(t *testing.T) {
	for _, prepared := range []bool{false, true} {
		t.Run(map[bool]string{false: "unprepared", true: "prepared"}[prepared], func(t *testing.T) {
			db := testutil.NewTestDB(t)
			ctx := context.Background()
			opp := core.Opportunity{HuntName: "test", Title: "obsolete", SourceID: "old", State: core.Discovered, StartTime: time.Now().Add(-48 * time.Hour)}
			id, err := db.InsertOpportunity(ctx, opp)
			if err != nil {
				t.Fatal(err)
			}
			opp.ID = id
			eid, err := db.SaveEvaluatedGroup(ctx, core.Evaluation{HuntName: "test", GroupKey: "old", EvaluatedAt: time.Now()}, []core.Pick{{OpportunityID: id, Score: 1}}, []core.Opportunity{opp})
			if err != nil {
				t.Fatal(err)
			}
			if prepared {
				if err := db.PrepareDelivery(ctx, eid, []core.NotifyAction{{Type: core.PostMessage, Message: core.NotifyMessage{Content: "obsolete"}}}); err != nil {
					t.Fatal(err)
				}
			}
			items := makeRawItems(1)
			items[0].SourceID = "new-old"
			items[0].StartTime = time.Now().Add(-24 * time.Hour).Format(time.RFC3339)
			ev := &testutil.FakeEvaluator{}
			n := &testutil.FakeNotifier{}
			h := &fake.FakeHunt{HuntName: "test", Eval: ev, Srcs: []core.Source{&fake.FakeSource{Items: items}}, ExpirerFn: func(o core.Opportunity) bool { return o.StartTime.Before(time.Now()) }, ReEvalFn: func(core.Opportunity, *core.Evaluation) bool { return true }, NotifyFmtFn: func() core.NotifyFormatter { return threadDetailFormatter{} }}
			r := pipeline.New(db, core.NewCostTracker(0, nil), n, core.ScanRegion{}, "").Run(ctx, h)
			pending, err := db.PendingDeliveries(ctx, "test")
			if len(ev.Calls) != 0 || len(n.Actions) != 0 || len(pending) != 0 || err != nil || len(r.Errors) != 0 {
				t.Fatalf("evals=%d sends=%d pending=%d err=%v result=%+v", len(ev.Calls), len(n.Actions), len(pending), err, r)
			}
			expired, err := db.GetByState(ctx, "test", core.Expired)
			if err != nil || len(expired) != 2 {
				t.Fatalf("expired=%d err=%v", len(expired), err)
			}
		})
	}
}

func TestEvaluationReceivesOnlyRemainingShowDates(t *testing.T) {
	db := testutil.NewTestDB(t)
	ctx := context.Background()
	past := time.Now().Add(-48 * time.Hour)
	future := time.Now().Add(48 * time.Hour)
	o := core.Opportunity{HuntName: "test", Title: "merged", State: core.Discovered, StartTime: past, ShowDates: []time.Time{future, past}}
	if _, err := db.InsertOpportunity(ctx, o); err != nil {
		t.Fatal(err)
	}
	ev := &testutil.FakeEvaluator{}
	h := &fake.FakeHunt{HuntName: "test", Eval: ev}
	r := pipeline.New(db, core.NewCostTracker(0, nil), &disabledNotifier{}, core.ScanRegion{}, "").Run(ctx, h)
	if len(r.Errors) != 0 || len(ev.Calls) != 1 {
		t.Fatalf("result=%+v calls=%d", r, len(ev.Calls))
	}
	got := ev.Calls[0].Opportunities[0]
	if len(got.ShowDates) != 1 || got.StartTime.Before(time.Now()) || got.ShowDates[0].Before(time.Now()) {
		t.Fatalf("past dates reached evaluator: %+v", got)
	}
}

func TestSavedDeliveryScheduleDriftNeverReplays(t *testing.T) {
	for _, prepared := range []bool{false, true} {
		for _, merged := range []bool{false, true} {
			t.Run(fmt.Sprintf("prepared=%v/merged=%v", prepared, merged), func(t *testing.T) {
				db := testutil.NewTestDB(t)
				ctx := context.Background()
				past := time.Now().Add(-48 * time.Hour)
				future := time.Now().Add(48 * time.Hour)
				current := core.Opportunity{HuntName: "test", Title: "remaining event", State: core.Evaluated, StartTime: future}
				if merged {
					current.StartTime = past
					current.ShowDates = []time.Time{past, future}
				}
				id, err := db.InsertOpportunity(ctx, current)
				if err != nil {
					t.Fatal(err)
				}
				current.ID = id
				saved := current
				if !merged {
					saved.StartTime = past
				}
				eid, err := db.SaveEvaluatedGroup(ctx, core.Evaluation{HuntName: "test", GroupKey: "drift"}, []core.Pick{{OpportunityID: id, Reason: "Buy tickets for old date"}}, []core.Opportunity{saved})
				if err != nil {
					t.Fatal(err)
				}
				remaining := []core.NotifyAction{{Type: core.PostToThread, ThreadID: "already-created", Message: core.NotifyMessage{Content: "old dates"}}}
				if prepared {
					if err := db.PrepareDelivery(ctx, eid, remaining); err != nil {
						t.Fatal(err)
					}
				}
				h := &fake.FakeHunt{HuntName: "test", Eval: &testutil.FakeEvaluator{}, NotifyFmtFn: func() core.NotifyFormatter { return expiryPickFormatter{} }}
				n := &testutil.FakeNotifier{}
				r := pipeline.New(db, core.NewCostTracker(0, nil), n, core.ScanRegion{}, "").Run(ctx, h)
				pending, err := db.PendingDeliveries(ctx, "test")
				if err != nil || len(pending) != 1 || len(n.Actions) != 0 || len(r.Errors) == 0 {
					t.Fatalf("unsafe replay result=%+v sends=%d pending=%+v err=%v", r, len(n.Actions), pending, err)
				}
				if prepared && (len(pending[0].Actions) != 1 || pending[0].Actions[0].ThreadID != "already-created") {
					t.Fatal("checkpoint altered")
				}
			})
		}
	}
}
