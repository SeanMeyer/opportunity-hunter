package pipeline_test

import (
	"context"
	"errors"
	"github.com/seanmeyer/opportunity-hunter/core"
	"github.com/seanmeyer/opportunity-hunter/hunts/fake"
	"github.com/seanmeyer/opportunity-hunter/pipeline"
	"github.com/seanmeyer/opportunity-hunter/testutil"
	"testing"
)

type disabledNotifier struct{ testutil.FakeNotifier }

func (*disabledNotifier) NotificationsEnabled(string) bool { return false }

func TestDisabledNotificationsCompleteWithoutSending(t *testing.T) {
	for _, dry := range []bool{false, true} {
		t.Run(map[bool]string{false: "live", true: "dry"}[dry], func(t *testing.T) {
			db := testutil.NewTestDB(t)
			ctx := context.Background()
			n := &disabledNotifier{}
			h := &fake.FakeHunt{HuntName: "test", Eval: &testutil.FakeEvaluator{}, Srcs: []core.Source{&fake.FakeSource{Items: makeRawItems(1)}}, NotifyFmtFn: func() core.NotifyFormatter { return threadDetailFormatter{} }}
			p := pipeline.New(db, core.NewCostTracker(0, nil), n, core.ScanRegion{}, "")
			p.SetDryRun(dry)
			r := p.Run(ctx, h)
			pending, err := db.PendingDeliveries(ctx, "test")
			want := 0
			if dry {
				want = 1
			}
			if len(n.Actions) != 0 || r.Notified != 0 || len(r.Errors) != 0 || err != nil || len(pending) != want {
				t.Fatalf("actions=%d result=%+v pending=%d err=%v", len(n.Actions), r, len(pending), err)
			}
		})
	}
}

func TestOptOutConsumesPreviouslyPreparedThreadDelivery(t *testing.T) {
	db := testutil.NewTestDB(t)
	ctx := context.Background()
	h := &fake.FakeHunt{HuntName: "test", Eval: &testutil.FakeEvaluator{}, Srcs: []core.Source{&fake.FakeSource{Items: makeRawItems(1)}}, NotifyFmtFn: func() core.NotifyFormatter { return threadDetailFormatter{} }}
	failing := &testutil.FakeNotifier{Err: errors.New("offline")}
	pipeline.New(db, core.NewCostTracker(0, nil), failing, core.ScanRegion{}, "").Run(ctx, h)
	pending, err := db.PendingDeliveries(ctx, "test")
	if err != nil || len(pending) != 1 || !pending[0].Prepared || len(pending[0].Actions) != 3 {
		t.Fatalf("missing prepared thread backlog: %+v, %v", pending, err)
	}
	n := &disabledNotifier{}
	p := pipeline.New(db, core.NewCostTracker(0, nil), n, core.ScanRegion{}, "")
	p.SetDryRun(true)
	p.Run(ctx, h)
	pending, err = db.PendingDeliveries(ctx, "test")
	if err != nil || len(pending) != 1 {
		t.Fatalf("dry run consumed queue: %v %v", pending, err)
	}
	p.SetDryRun(false)
	r := p.Run(ctx, h)
	pending, err = db.PendingDeliveries(ctx, "test")
	if len(n.Actions) != 0 || r.Notified != 0 || len(r.Errors) != 0 || err != nil || len(pending) != 0 {
		t.Fatalf("actions=%d result=%+v pending=%d err=%v", len(n.Actions), r, len(pending), err)
	}
	live := &testutil.FakeNotifier{}
	pipeline.New(db, core.NewCostTracker(0, nil), live, core.ScanRegion{}, "").Run(ctx, h)
	if len(live.Actions) != 0 {
		t.Fatal("opted-out backlog replayed after re-enabling")
	}
}
