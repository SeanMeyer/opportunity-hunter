package pipeline_test

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/seanmeyer/opportunity-hunter/core"
	"github.com/seanmeyer/opportunity-hunter/hunts/fake"
	"github.com/seanmeyer/opportunity-hunter/pipeline"
	"github.com/seanmeyer/opportunity-hunter/storage"
	"github.com/seanmeyer/opportunity-hunter/testutil"
)

type threadDetailFormatter struct{}

func (threadDetailFormatter) FormatPicks(core.NotifyContext) []core.NotifyAction {
	return []core.NotifyAction{
		{Type: core.CreateThread, ThreadName: "trip"},
		{Type: core.PostToThread, ThreadRef: "trip", Message: core.NotifyMessage{Content: "first detail"}},
		{Type: core.PostToThread, ThreadRef: "trip", Message: core.NotifyMessage{Content: "second detail"}},
	}
}
func (threadDetailFormatter) FormatReminder(core.Opportunity, core.Pick, core.ReminderType) []core.NotifyAction {
	return nil
}

type progressNotifier struct {
	creates, first, second int
	fail                   bool
	badRef                 bool
}

func (n *progressNotifier) ExecuteActions(_ string, actions []core.NotifyAction) (map[string]string, error) {
	threads := map[string]string{}
	for _, a := range actions {
		switch a.Type {
		case core.CreateThread:
			n.creates++
			threads[a.ThreadName] = "known-thread"
		case core.PostToThread:
			if a.ThreadID != "known-thread" && threads[a.ThreadRef] != "known-thread" {
				n.badRef = true
				return threads, errors.New("unresolved reference")
			}
			if a.Message.Content == "first detail" {
				n.first++
			} else {
				n.second++
				if n.fail {
					return threads, errors.New("temporary detail failure")
				}
			}
		}
	}
	return threads, nil
}
func (*progressNotifier) PostError(string) error { return nil }

func TestDeliveryCheckpointsEachActionAcrossRestart(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "test.db")
	db, err := storage.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { db.Close() }()
	n := &progressNotifier{fail: true}
	ev := &testutil.FakeEvaluator{}
	h := &fake.FakeHunt{HuntName: "test", Eval: ev, Srcs: []core.Source{&fake.FakeSource{Items: makeRawItems(1)}}, NotifyFmtFn: func() core.NotifyFormatter { return threadDetailFormatter{} }}
	p := pipeline.New(db, core.NewCostTracker(0, nil), n, core.ScanRegion{}, "")
	r := p.Run(ctx, h)
	if len(r.Errors) == 0 {
		t.Fatal("expected detail failure")
	}
	db.Close()
	db, err = storage.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	n.fail = false
	p = pipeline.New(db, core.NewCostTracker(0, nil), n, core.ScanRegion{}, "")
	p.Run(ctx, h)
	p.Run(ctx, h)
	if n.creates != 1 || n.first != 1 || n.second != 2 || n.badRef || len(ev.Calls) != 1 {
		t.Fatalf("delivery progress=%+v evaluations=%d", n, len(ev.Calls))
	}
}

func TestDryRunPreservesPendingDelivery(t *testing.T) {
	db := testutil.NewTestDB(t)
	ctx := context.Background()
	n := &testutil.FakeNotifier{Err: errors.New("delivery failed")}
	h := &fake.FakeHunt{HuntName: "test", Eval: &testutil.FakeEvaluator{}, Srcs: []core.Source{&fake.FakeSource{Items: makeRawItems(1)}}, NotifyFmtFn: func() core.NotifyFormatter { return recoveryFormatter{} }}
	p := pipeline.New(db, core.NewCostTracker(0, nil), n, core.ScanRegion{}, "")
	p.Run(ctx, h)
	n.Err = nil
	setter, ok := any(p).(interface{ SetDryRun(bool) })
	if !ok {
		t.Fatal("pipeline has no public dry-run configuration")
	}
	setter.SetDryRun(true)
	p.Run(ctx, h)
	pending, err := db.PendingDeliveries(ctx, "test")
	if err != nil || len(pending) != 1 || len(n.Actions) != 1 {
		t.Fatalf("dry run consumed delivery: pending=%d actions=%d err=%v", len(pending), len(n.Actions), err)
	}
	setter.SetDryRun(false)
	p.Run(ctx, h)
	if len(n.Actions) != 2 {
		t.Fatal("live run did not deliver preserved work")
	}
}

type failingCreateNotifier struct {
	calls             int
	fail, acknowledge bool
}

func (n *failingCreateNotifier) ExecuteActions(_ string, actions []core.NotifyAction) (map[string]string, error) {
	n.calls++
	threads := map[string]string{}
	if actions[0].Type == core.CreateThread && (!n.fail || n.acknowledge) {
		threads[actions[0].ThreadName] = "known-thread"
	}
	if n.fail {
		return threads, errors.New("webhook unavailable")
	}
	return threads, nil
}
func (*failingCreateNotifier) PostError(string) error { return nil }

func TestDeliveryFailureStopsHuntAndPreservesQueue(t *testing.T) {
	for _, acknowledge := range []bool{false, true} {
		t.Run(map[bool]string{false: "unacknowledged", true: "acknowledged-create"}[acknowledge], func(t *testing.T) {
			db := testutil.NewTestDB(t)
			ctx := context.Background()
			n := &failingCreateNotifier{fail: true, acknowledge: acknowledge}
			src := &fake.FakeSource{Items: makeRawItems(2)}
			h := &fake.FakeHunt{HuntName: "test", Eval: &testutil.FakeEvaluator{}, Srcs: []core.Source{src}, NotifyFmtFn: func() core.NotifyFormatter { return threadDetailFormatter{} }}
			p := pipeline.New(db, core.NewCostTracker(0, nil), n, core.ScanRegion{}, "")
			r := p.Run(ctx, h)
			if n.calls != 1 || len(r.Errors) != 1 {
				t.Fatalf("failure did not stop delivery: calls=%d errors=%v", n.calls, r.Errors)
			}
			pending, err := db.PendingDeliveries(ctx, "test")
			if err != nil || len(pending) != 2 {
				t.Fatalf("queue lost: pending=%v err=%v", pending, err)
			}
			wantActions := 3
			if acknowledge {
				wantActions = 2
			}
			if len(pending[0].Actions) != wantActions {
				t.Fatalf("acknowledgment not preserved: %+v", pending[0])
			}
			if acknowledge && (pending[0].Actions[0].ThreadID != "known-thread" || pending[0].Actions[0].ThreadRef != "") {
				t.Fatalf("thread reference unresolved: %+v", pending[0].Actions[0])
			}
			// New work forces the second delivery pass after the initial retry fails.
			src.Items = makeRawItems(3)
			r = p.Run(ctx, h)
			if n.calls != 2 || r.Evaluated != 1 {
				t.Fatalf("initial failure did not suppress second pass: calls=%d result=%+v", n.calls, r)
			}
			n.fail = false
			p.Run(ctx, h)
			pending, err = db.PendingDeliveries(ctx, "test")
			if err != nil || len(pending) != 0 {
				t.Fatalf("queue did not recover: pending=%v err=%v", pending, err)
			}
		})
	}
}

type synthesisFormatter struct{}

func (synthesisFormatter) FormatPicks(c core.NotifyContext) []core.NotifyAction {
	return []core.NotifyAction{{Type: core.PostMessage, Message: core.NotifyMessage{Content: c.Synthesis}}}
}
func (synthesisFormatter) FormatReminder(core.Opportunity, core.Pick, core.ReminderType) []core.NotifyAction {
	return nil
}

func TestDeliveryFailurePreservesUnpreparedSynthesis(t *testing.T) {
	db := testutil.NewTestDB(t)
	ctx := context.Background()
	n := &testutil.FakeNotifier{Err: errors.New("webhook unavailable")}
	src := &fake.FakeSource{Items: makeRawItems(2)}
	h := &fake.FakeHunt{HuntName: "test", Eval: &testutil.FakeEvaluator{}, Srcs: []core.Source{src}, NotifyFmtFn: func() core.NotifyFormatter { return synthesisFormatter{} },
		BrieferGroupFn: func(evals []core.Evaluation) []core.NotifyGroup {
			var groups []core.NotifyGroup
			for _, e := range evals {
				groups = append(groups, core.NotifyGroup{Key: e.GroupKey})
			}
			return groups
		},
		SynthesizeFn: func(_ context.Context, g core.NotifyGroup, _ *core.CostTracker) (string, error) {
			return "briefing for " + g.Key, nil
		},
	}
	p := pipeline.New(db, core.NewCostTracker(0, nil), n, core.ScanRegion{}, "")
	p.Run(ctx, h) // Final pass fails before the second group is prepared.
	src.Items = makeRawItems(3)
	p.Run(ctx, h) // Initial pass fails; third group's synthesis must still persist.
	pending, err := db.PendingDeliveries(ctx, "test")
	if err != nil || len(pending) != 3 {
		t.Fatalf("pending=%v err=%v", pending, err)
	}
	for _, d := range pending {
		if d.Context.Synthesis != "briefing for "+d.GroupKey {
			t.Errorf("lost synthesis for %s: %q", d.GroupKey, d.Context.Synthesis)
		}
	}
	n.Err = nil
	n.Actions = nil
	p = pipeline.New(db, core.NewCostTracker(0, nil), n, core.ScanRegion{}, "")
	p.Run(ctx, h)
	if len(n.Actions) != 3 {
		t.Fatalf("delivered %d actions", len(n.Actions))
	}
	for i, a := range n.Actions {
		if a.Message.Content != "briefing for "+pending[i].GroupKey {
			t.Errorf("retry lost briefing: %+v", a)
		}
	}
}
