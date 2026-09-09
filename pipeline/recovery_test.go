package pipeline_test

import (
	"context"
	"errors"
	"github.com/seanmeyer/opportunity-hunter/core"
	"github.com/seanmeyer/opportunity-hunter/hunts/fake"
	"github.com/seanmeyer/opportunity-hunter/pipeline"
	"github.com/seanmeyer/opportunity-hunter/storage"
	"github.com/seanmeyer/opportunity-hunter/testutil"
	"path/filepath"
	"testing"
)

type recoveryFormatter struct{}

type groupedRecoveryFormatter struct {
	contexts []core.NotifyContext
	empty    bool
}

func (f *groupedRecoveryFormatter) FormatPicks(c core.NotifyContext) []core.NotifyAction {
	f.contexts = append(f.contexts, c)
	if f.empty {
		return nil
	}
	return []core.NotifyAction{{Type: core.CreateThread, ThreadName: c.Evaluations[0].GroupKey}}
}
func (*groupedRecoveryFormatter) FormatReminder(core.Opportunity, core.Pick, core.ReminderType) []core.NotifyAction {
	return nil
}

type selectiveNotifier struct{ calls map[string]int }

func (n *selectiveNotifier) ExecuteActions(_ string, a []core.NotifyAction) (map[string]string, error) {
	key := a[0].ThreadName
	n.calls[key]++
	if key == "Show 1" && n.calls[key] == 1 {
		return nil, errors.New("temporary failure")
	}
	return map[string]string{key: "thread-" + key}, nil
}
func (*selectiveNotifier) PostError(string) error { return nil }

func TestDeliveryGroupsAndEmptyDecisions(t *testing.T) {
	for _, empty := range []bool{false, true} {
		t.Run(map[bool]string{false: "groups", true: "empty"}[empty], func(t *testing.T) {
			db := testutil.NewTestDB(t)
			ctx := context.Background()
			formatter := &groupedRecoveryFormatter{empty: empty}
			n := &selectiveNotifier{calls: map[string]int{}}
			h := &fake.FakeHunt{HuntName: "test", Eval: &testutil.FakeEvaluator{}, Srcs: []core.Source{&fake.FakeSource{Items: makeRawItems(2)}}, NotifyFmtFn: func() core.NotifyFormatter { return formatter }}
			p := pipeline.New(db, core.NewCostTracker(0, nil), n, core.ScanRegion{}, "")
			p.Run(ctx, h)
			p.Run(ctx, h)
			p.Run(ctx, h)
			pending, err := db.PendingDeliveries(ctx, "test")
			if err != nil || len(pending) != 0 {
				t.Fatalf("pending=%+v err=%v", pending, err)
			}
			if len(formatter.contexts) != 2 {
				t.Fatalf("formatted %d times", len(formatter.contexts))
			}
			if empty {
				if len(n.calls) != 0 {
					t.Fatal("empty decision sent")
				}
				return
			}
			if n.calls["Show 0"] != 1 || n.calls["Show 1"] != 2 {
				t.Fatalf("calls=%v", n.calls)
			}
			for _, key := range []string{"Show 0", "Show 1"} {
				id, err := db.GetThread(ctx, "test", key)
				if err != nil || id != "thread-"+key {
					t.Fatalf("thread %s=%s err=%v", key, id, err)
				}
			}
		})
	}
}

func (recoveryFormatter) FormatPicks(core.NotifyContext) []core.NotifyAction {
	return []core.NotifyAction{{Type: core.PostMessage}}
}
func (recoveryFormatter) FormatReminder(core.Opportunity, core.Pick, core.ReminderType) []core.NotifyAction {
	return nil
}

type canceledEvaluator struct{ cancel context.CancelFunc }

func (e canceledEvaluator) Evaluate(ctx context.Context, _ core.EvalContext) (*core.EvalResult, error) {
	e.cancel()
	return nil, ctx.Err()
}

func TestRecoveryFailures(t *testing.T) {
	for _, mode := range []string{"notify", "save", "state", "scan", "cancel"} {
		t.Run(mode, func(t *testing.T) {
			ctx := context.Background()
			path := filepath.Join(t.TempDir(), "test.db")
			db, err := storage.Open(path)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { db.Close() }()
			ct := core.NewCostTracker(0, nil)
			n := &testutil.FakeNotifier{}
			ev := &testutil.FakeEvaluator{CostUSD: .1, Picks: []core.Pick{{Score: .9}}}
			h := &fake.FakeHunt{HuntName: "test", Eval: ev, Srcs: []core.Source{&fake.FakeSource{Items: makeRawItems(1)}}, NotifyFmtFn: func() core.NotifyFormatter { return recoveryFormatter{} }}
			switch mode {
			case "notify":
				n.Err = errors.New("delivery failed")
			case "save":
				_, err = db.RawDB().Exec("CREATE TRIGGER fail BEFORE INSERT ON evaluations BEGIN SELECT RAISE(FAIL,'save failed'); END")
			case "scan":
				_, err = db.RawDB().Exec("CREATE TRIGGER fail BEFORE INSERT ON opportunities BEGIN SELECT RAISE(FAIL,'scan failed'); END")
			case "state":
				_, err = db.RawDB().Exec("CREATE TRIGGER fail BEFORE UPDATE ON opportunities BEGIN SELECT RAISE(FAIL,'state failed'); END")
			case "cancel":
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				h.Eval = canceledEvaluator{cancel}
			}
			if err != nil {
				t.Fatal(err)
			}
			p := pipeline.New(db, ct, n, core.ScanRegion{}, "")
			r := p.Run(ctx, h)
			if len(r.Errors) == 0 {
				t.Fatal("failure missing from result")
			}
			switch mode {
			case "notify":
				db.Close()
				budget := 0.0
				h.Sched.MaxMonthlySpendUSD = &budget
				db, err = storage.Open(path)
				if err != nil {
					t.Fatal(err)
				}
				n.Err = nil
				p = pipeline.New(db, ct, n, core.ScanRegion{}, "")
				r = p.Run(ctx, h)
				if r.Notified != 1 || len(ev.Calls) != 1 {
					t.Fatalf("retry=%+v eval calls=%d", r, len(ev.Calls))
				}
				p.Run(ctx, h)
				if len(n.Actions) != 2 {
					t.Fatalf("successful delivery repeated: %d", len(n.Actions))
				}
			case "save", "state":
				if len(n.Actions) != 0 || ct.Total() != .1 {
					t.Fatalf("actions=%d cost=%f", len(n.Actions), ct.Total())
				}
			case "cancel":
				run, err := db.LatestRun(context.Background(), "test")
				if err != nil || run.FinishedAt == nil || run.Status != "error" {
					t.Fatalf("run=%+v err=%v", run, err)
				}
			}
		})
	}
}
