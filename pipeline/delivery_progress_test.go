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
