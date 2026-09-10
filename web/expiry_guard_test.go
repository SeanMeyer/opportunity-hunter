package web_test

import (
	"context"
	"github.com/seanmeyer/opportunity-hunter/core"
	"github.com/seanmeyer/opportunity-hunter/hunts/movies"
	"github.com/seanmeyer/opportunity-hunter/testutil"
	"github.com/seanmeyer/opportunity-hunter/web"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestCardsHideExpiredBetweenScans(t *testing.T) {
	db := testutil.NewTestDB(t)
	ctx := context.Background()
	for _, v := range []struct {
		title  string
		offset time.Duration
	}{{"Past event sentinel", -48 * time.Hour}, {"Future event sentinel", 48 * time.Hour}} {
		o := core.Opportunity{HuntName: "comedy", Title: v.title, SourceID: v.title, State: core.Evaluated, StartTime: time.Now().Add(v.offset)}
		id, err := db.InsertOpportunity(ctx, o)
		if err != nil {
			t.Fatal(err)
		}
		o.ID = id
		_, err = db.SaveEvaluatedGroup(ctx, core.Evaluation{HuntName: "comedy", GroupKey: v.title}, []core.Pick{{OpportunityID: id, Score: 1}}, []core.Opportunity{o})
		if err != nil {
			t.Fatal(err)
		}
	}
	s, err := web.New(db, []web.HuntInfo{{Name: "comedy"}}, "")
	if err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRecorder()
	s.Handler().ServeHTTP(r, httptest.NewRequest("GET", "/?hunt=comedy", nil))
	if strings.Contains(r.Body.String(), "Past event sentinel") || !strings.Contains(r.Body.String(), "Future event sentinel") {
		t.Fatal("expired event visible or future event missing")
	}
}

func TestCardsHonorStreamingAvailabilityPolicy(t *testing.T) {
	db := testutil.NewTestDB(t)
	ctx := context.Background()
	o := core.Opportunity{HuntName: "movies", Title: "Available streaming sentinel", State: core.Evaluated, StartTime: time.Now().AddDate(-1, 0, 0), Attributes: core.Attributes(`{"release_type":"streaming"}`)}
	id, err := db.InsertOpportunity(ctx, o)
	if err != nil {
		t.Fatal(err)
	}
	o.ID = id
	_, err = db.SaveEvaluatedGroup(ctx, core.Evaluation{HuntName: "movies"}, []core.Pick{{OpportunityID: id, Score: 1}}, []core.Opportunity{o})
	if err != nil {
		t.Fatal(err)
	}
	s, err := web.New(db, []web.HuntInfo{{Name: "movies", Expirer: &movies.MoviesHunt{}}}, "")
	if err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRecorder()
	s.Handler().ServeHTTP(r, httptest.NewRequest("GET", "/?hunt=movies", nil))
	if !strings.Contains(r.Body.String(), o.Title) {
		t.Fatal("streaming movie incorrectly hidden")
	}
}
