package web

import (
	"context"
	"fmt"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/seanmeyer/opportunity-hunter/core"
	"github.com/seanmeyer/opportunity-hunter/testutil"
)

func TestVenueFilterCombinesScoreAndAliases(t *testing.T) {
	db := testutil.NewTestDB(t)
	ctx := context.Background()
	venues := []core.Venue{
		{Name: "Comedy Works Downtown", Address: "1226 15th St"},
		{Name: "Comedy Works", Address: "1226 15th Street"},
		{Name: "Comedy Works South", Address: "5345 Landmark Pl"},
	}
	for i, v := range venues {
		vid, err := db.UpsertVenue(ctx, v)
		if err != nil {
			t.Fatal(err)
		}
		for j, score := range []float64{.9, .7} {
			id, err := db.InsertOpportunity(ctx, core.Opportunity{HuntName: "comedy", SourceID: fmt.Sprintf("%d-%d", i, j), Title: fmt.Sprintf("Show %d-%d", i, j), VenueID: &vid, State: core.Evaluated, DiscoveredAt: time.Now(), StartTime: time.Now().Add(time.Hour)})
			if err != nil {
				t.Fatal(err)
			}
			_, err = db.SaveEvaluationWithPicks(ctx, core.Evaluation{HuntName: "comedy", EvaluatedAt: time.Now()}, []core.Pick{{OpportunityID: id, Score: score}})
			if err != nil {
				t.Fatal(err)
			}
		}
	}
	s, err := New(db, []HuntInfo{{Name: "comedy", CardRenderer: interactionRenderer{}}}, "")
	if err != nil {
		t.Fatal(err)
	}
	key := core.EventVenueKey(venues[0].Name, venues[0].Address)
	for _, tc := range []struct {
		venue, score string
		count        int
	}{
		{key, "", 4}, {key, "8", 2}, {"", "8", 3}, {"missing", "", 0},
	} {
		w := httptest.NewRecorder()
		q := url.Values{"hunt": {"comedy"}, "venue": {tc.venue}, "filter": {tc.score}}
		s.Handler().ServeHTTP(w, httptest.NewRequest("GET", "/?"+q.Encode(), nil))
		body := w.Body.String()
		if w.Code != 200 || strings.Count(body, "<article ") != tc.count {
			t.Fatalf("%+v: status %d cards %d", tc, w.Code, strings.Count(body, "<article "))
		}
		if !strings.Contains(body, `id="card-venue"`) {
			t.Fatal("missing venue dropdown")
		}
		if tc.venue == key && strings.Contains(body, ">Show 2-") {
			t.Fatal("South leaked into Downtown")
		}
		if tc.venue == "missing" && !strings.Contains(body, "Clear filters") {
			t.Fatal("missing reset for unavailable venue")
		}
	}
}

func TestVenuePreservedByFormRedirects(t *testing.T) {
	s, _ := New(testutil.NewTestDB(t), []HuntInfo{{Name: "comedy"}}, "")
	r := httptest.NewRequest("POST", "/preferences", strings.NewReader("hunt=comedy&sort=date&filter=8&venue=downtown"))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, r)
	loc, _ := url.Parse(w.Header().Get("Location"))
	if w.Code != 303 || loc.Query().Get("venue") != "downtown" {
		t.Fatalf("lost venue: %d %s", w.Code, loc)
	}
}

func TestVenueUnavailableWithNoCardsStillOffersReset(t *testing.T) {
	s, _ := New(testutil.NewTestDB(t), []HuntInfo{{Name: "comedy"}}, "")
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, httptest.NewRequest("GET", "/?hunt=comedy&venue=old", nil))
	for _, want := range []string{`id="card-venue"`, "Selected venue (no current events)", "Clear filters"} {
		if !strings.Contains(w.Body.String(), want) {
			t.Fatalf("missing %s", want)
		}
	}
}
