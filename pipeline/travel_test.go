package pipeline

import (
	"context"
	"github.com/seanmeyer/opportunity-hunter/core"
	"github.com/seanmeyer/opportunity-hunter/hunts/fake"
	"github.com/seanmeyer/opportunity-hunter/storage"
	"github.com/seanmeyer/opportunity-hunter/testutil"
	"testing"
	"time"
)

type travelHunt struct {
	fake.FakeHunt
	sawCached bool
}

func (h *travelHunt) EnrichVenues(_ context.Context, vs map[int64]core.Venue) {
	for id, v := range vs {
		h.sawCached = v.WalkingMinutes == 373 && v.DistanceMi == 15.7
		v.DrivingMinutes = 21
		v.DrivingDistanceMi = 16.8
		vs[id] = v
	}
}
func TestTravelCachePersistsSeparateModes(t *testing.T) {
	ctx := context.Background()
	db := testutil.NewTestDB(t)
	id, err := db.UpsertVenue(ctx, core.Venue{Name: "Hall", Address: "venue"})
	if err != nil {
		t.Fatal(err)
	}
	if err = db.SaveDistance(ctx, storage.DistanceRow{VenueID: id, HomeAddress: "origin", Mode: "walking", Minutes: 373, DistanceMi: 15.7, CreatedAt: time.Now()}); err != nil {
		t.Fatal(err)
	}
	ev := &testutil.FakeEvaluator{}
	h := &travelHunt{FakeHunt: fake.FakeHunt{HuntName: "test", Eval: ev}}
	p := New(db, core.NewCostTracker(0, nil), &testutil.FakeNotifier{}, core.ScanRegion{}, "origin")
	opp := core.Opportunity{HuntName: "test", VenueID: &id, State: core.Discovered, Title: "Show", StartTime: time.Now().Add(time.Hour)}
	opp.ID, err = db.InsertOpportunity(ctx, opp)
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = p.evaluateGroup(ctx, h, core.Group{Opportunities: []core.Opportunity{opp}})
	if err != nil {
		t.Fatal(err)
	}
	if !h.sawCached {
		t.Fatal("existing walking cache not loaded before enrichment")
	}
	walk, err := db.GetDistance(ctx, id, "origin", "walking")
	if err != nil {
		t.Fatal(err)
	}
	drive, err := db.GetDistance(ctx, id, "origin", "driving")
	if err != nil {
		t.Fatal(err)
	}
	if walk.DistanceMi != 15.7 || drive.DistanceMi != 16.8 || drive.Minutes != 21 {
		t.Fatalf("wrong cached modes: walking=%+v driving=%+v", walk, drive)
	}
	if len(ev.Calls) != 1 || ev.Calls[0].Venues[id].DrivingMinutes != 21 {
		t.Fatal("evaluation missed driving route")
	}
}
