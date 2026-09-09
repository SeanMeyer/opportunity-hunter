package main

import (
	"context"
	"errors"
	"github.com/seanmeyer/opportunity-hunter/core"
	"github.com/seanmeyer/opportunity-hunter/hunts/fake"
	"github.com/seanmeyer/opportunity-hunter/testutil"
	"testing"
	"time"
)

func TestStartupAdvancesCompletedHuntBeforeNext(t *testing.T) {
	for _, failed := range []bool{false, true} {
		t.Run(map[bool]string{false: "success", true: "error"}[failed], func(t *testing.T) {
			db := testutil.NewTestDB(t)
			ctx := context.Background()
			hunts := []core.Hunt{&fake.FakeHunt{HuntName: "first"}, &fake.FakeHunt{HuntName: "second"}}
			for _, h := range hunts {
				if err := db.SeedScheduleIfNotExists(ctx, h.Name(), 60, time.Now().AddDate(0, -5, 0)); err != nil {
					t.Fatal(err)
				}
			}
			entered := make(chan struct{})
			unblock := make(chan struct{})
			done := make(chan struct{})
			go func() {
				defer close(done)
				runInitialHunts(ctx, db, hunts, func(_ context.Context, h core.Hunt) core.HuntResult {
					if h.Name() == "second" {
						close(entered)
						<-unblock
					}
					r := core.HuntResult{HuntName: h.Name()}
					if failed {
						r.Errors = []core.StepError{{Step: "scan", Err: errors.New("failed")}}
					}
					return r
				})
			}()
			<-entered
			sched, err := db.GetSchedule(ctx, "first")
			close(unblock)
			<-done
			if err != nil || !sched.NextScanAt.After(time.Now()) {
				t.Fatalf("completed hunt still overdue: %+v, %v", sched, err)
			}
		})
	}
}

func TestStartupCancellationAdvancesFinishedHuntOnly(t *testing.T) {
	db := testutil.NewTestDB(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	hunts := []core.Hunt{&fake.FakeHunt{HuntName: "first"}, &fake.FakeHunt{HuntName: "second"}}
	for _, h := range hunts {
		if err := db.SeedScheduleIfNotExists(ctx, h.Name(), 60, time.Now().AddDate(0, -5, 0)); err != nil {
			t.Fatal(err)
		}
	}
	calls := 0
	runInitialHunts(ctx, db, hunts, func(_ context.Context, h core.Hunt) core.HuntResult {
		calls++
		cancel()
		return core.HuntResult{HuntName: h.Name()}
	})
	first, _ := db.GetSchedule(context.Background(), "first")
	second, _ := db.GetSchedule(context.Background(), "second")
	if calls != 1 || !first.NextScanAt.After(time.Now()) || second.NextScanAt.After(time.Now()) {
		t.Fatalf("calls=%d first=%v second=%v", calls, first, second)
	}
}

func TestNotificationConfigurationReachesPipelineAndUI(t *testing.T) {
	hunts := []core.Hunt{&fake.FakeHunt{HuntName: "comedy"}, &fake.FakeHunt{HuntName: "powder"}}
	webhooks := map[string]string{"powder": "https://example.com/webhook"}
	n := newRoutingNotifier(webhooks, "")
	infos := buildHuntInfos(hunts, webhooks)
	if n.NotificationsEnabled("comedy") || !n.NotificationsEnabled("powder") || infos[0].NotificationsEnabled || !infos[1].NotificationsEnabled {
		t.Fatal("notification opt-out not reflected in notifier and UI")
	}
}
