package pipeline_test

import (
	"context"
	"github.com/seanmeyer/opportunity-hunter/core"
	"github.com/seanmeyer/opportunity-hunter/hunts/comedy"
	"github.com/seanmeyer/opportunity-hunter/hunts/fake"
	"github.com/seanmeyer/opportunity-hunter/hunts/movies"
	"github.com/seanmeyer/opportunity-hunter/hunts/performing"
	"github.com/seanmeyer/opportunity-hunter/hunts/powder"
	"github.com/seanmeyer/opportunity-hunter/pipeline"
	"github.com/seanmeyer/opportunity-hunter/testutil"
	"testing"
	"time"
)

// Replace network sources while retaining each production hunt's identity rules.
type sourcedHunt struct {
	core.Hunt
	source core.Source
}

func TestPowderLegacyShowDatesDoNotHideDistinctWindows(t *testing.T) {
	db := testutil.NewTestDB(t)
	ctx := context.Background()
	start := time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC)
	next := start.Add(24 * time.Hour)
	end := start.Add(48 * time.Hour)
	// The old generic merger could attach another window's start to ShowDates
	// while retaining only the first window's end and attributes.
	_, err := db.InsertOpportunity(ctx, core.Opportunity{HuntName: "powder", SourceID: "legacy", Title: "Colorado", State: core.Discovered, StartTime: start, EndTime: &end, ShowDates: []time.Time{start, next}, DiscoveredAt: start})
	if err != nil {
		t.Fatal(err)
	}
	hunt := sourcedHunt{&powder.PowderHunt{}, &fake.FakeSource{SourceName: "test", Items: []core.RawItem{{SourceID: "second", Title: "Colorado", StartTime: next.Format(time.RFC3339), EndTime: end.Format(time.RFC3339)}}}}
	p := pipeline.New(db, core.NewCostTracker(0, nil), &testutil.FakeNotifier{}, core.ScanRegion{}, "")
	for scan, want := range []int{1, 0} {
		result := p.ScanAll(ctx, []core.Hunt{hunt}).HuntResults[0]
		if result.Scanned != want || len(result.Errors) != 0 {
			t.Fatalf("scan %d: %+v, want %d new", scan, result, want)
		}
	}
}

func (h sourcedHunt) Sources() []core.Source { return []core.Source{h.source} }
func (h sourcedHunt) MultiDateKey(raw core.RawItem) string {
	if merger, ok := h.Hunt.(interface{ MultiDateKey(core.RawItem) string }); ok {
		return merger.MultiDateKey(raw)
	}
	return ""
}

func TestScanPreservesHuntIdentityAcrossRescans(t *testing.T) {
	for _, tc := range []struct {
		name   string
		hunt   core.Hunt
		titles [2]string
		end    bool
		want   int
	}{
		{"powder windows", &powder.PowderHunt{}, [2]string{"Colorado", "Colorado"}, true, 2},
		{"distinct movies", &movies.MoviesHunt{}, [2]string{"Movie: Part One", "Movie: Part Two"}, false, 2},
		{"comedy dates", &comedy.ComedyHunt{}, [2]string{"Nikki Glaser", "Nikki Glaser: The Stunning Tour"}, false, 1},
		{"performing dates", &performing.PerformingHunt{}, [2]string{"Hamlet", "Hamlet"}, false, 1},
		{"source identity without merging", &fake.FakeHunt{HuntName: "source-id"}, [2]string{"Same title", "Same title"}, false, 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db := testutil.NewTestDB(t)
			ctx := context.Background()
			items := []core.RawItem{{SourceID: "one", Source: "test", Title: tc.titles[0], StartTime: "2027-01-01T00:00:00Z"}, {SourceID: "two", Source: "test", Title: tc.titles[1], StartTime: "2027-01-08T00:00:00Z"}}
			if tc.end {
				items[0].EndTime = "2027-01-03T00:00:00Z"
				items[1].EndTime = "2027-01-10T00:00:00Z"
			}
			h := sourcedHunt{tc.hunt, &fake.FakeSource{SourceName: "test", Items: items}}
			p := pipeline.New(db, core.NewCostTracker(0, nil), &testutil.FakeNotifier{}, core.ScanRegion{}, "")
			for scan := 0; scan < 3; scan++ {
				r := p.ScanAll(ctx, []core.Hunt{h}).HuntResults[0]
				want := 0
				if scan == 0 {
					want = tc.want
				}
				if r.Scanned != want || len(r.Errors) > 0 {
					t.Errorf("scan %d: stored=%d want=%d errors=%v", scan, r.Scanned, want, r.Errors)
				}
			}
			opps, err := db.GetByState(ctx, h.Name(), core.Discovered)
			if err != nil {
				t.Fatal(err)
			}
			if len(opps) != tc.want {
				t.Fatalf("stored %d opportunities, want %d", len(opps), tc.want)
			}
			if tc.want == 1 && len(opps[0].ShowDates) != 2 {
				t.Errorf("lost multi-date shows: %+v", opps[0].ShowDates)
			}
			if tc.end {
				for _, opp := range opps {
					if opp.EndTime == nil || opp.EndTime.Sub(opp.StartTime).Hours() != 48 {
						t.Errorf("corrupted window: %+v", opp)
					}
				}
			}
		})
	}
}
