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

func TestSavedDownvotesMoveAndCanReturn(t *testing.T) {
	db := testutil.NewTestDB(t)
	ctx := context.Background()
	var ids []int64
	for i, score := range []float64{.9, .7} {
		id, err := db.InsertOpportunity(ctx, core.Opportunity{HuntName: "comedy", SourceID: fmt.Sprint(i), Title: fmt.Sprintf("Show %d", i), State: core.Evaluated, DiscoveredAt: time.Now(), StartTime: time.Now().Add(time.Hour)})
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, id)
		_, err = db.SaveEvaluationWithPicks(ctx, core.Evaluation{HuntName: "comedy", EvaluatedAt: time.Now()}, []core.Pick{{OpportunityID: id, Score: score}})
		if err != nil {
			t.Fatal(err)
		}
	}
	s, _ := New(db, []HuntInfo{{Name: "comedy", CardRenderer: interactionRenderer{}}}, "")
	vote := func(id int64, rating string) *url.URL {
		t.Helper()
		body := url.Values{"hunt": {"comedy"}, "opportunity_id": {fmt.Sprint(id)}, "rating": {rating}, "sort": {"date"}}
		req := httptest.NewRequest("POST", "/feedback", strings.NewReader(body.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w := httptest.NewRecorder()
		s.Handler().ServeHTTP(w, req)
		if w.Code != 303 {
			t.Fatal(w.Code, w.Body.String())
		}
		loc, err := url.Parse(w.Header().Get("Location"))
		if err != nil {
			t.Fatal(err)
		}
		return loc
	}
	read := func(path string) string {
		t.Helper()
		w := httptest.NewRecorder()
		s.Handler().ServeHTTP(w, httptest.NewRequest("GET", path, nil))
		if w.Code != 200 {
			t.Fatal(w.Code, w.Body.String())
		}
		return w.Body.String()
	}
	down := vote(ids[0], "down")
	if down.Fragment != "not-for-me" || down.Query().Get("sort") != "date" {
		t.Fatalf("wrong redirect %s", down)
	}
	body := read(down.String())
	section := strings.Index(body, `<details id="not-for-me" class="downvoted-section">`)
	if section < 0 || strings.Index(body, `id="card-1"`) < section || strings.Index(body, `id="card-2"`) > section {
		t.Fatal("downvote not separated below remaining card")
	}
	if !strings.Contains(body, "Saved in Not for me.") || !strings.Contains(body, "2 results · 1 not for me") {
		t.Fatal("missing saved state/count")
	}
	if strings.Count(body, "<article ") != 2 || !strings.Contains(body, "Not for me (1)") {
		t.Fatal("missing or duplicated cards")
	}
	filtered := read("/?hunt=comedy&filter=8")
	if strings.Contains(filtered, `id="card-2"`) || !strings.Contains(filtered, `id="not-for-me"`) || !strings.Contains(filtered, "All matching recommendations are in Not for me below.") {
		t.Fatal("all-downvoted filtered view is wrong")
	}
	up := vote(ids[0], "up")
	body = read(up.String())
	if up.Fragment != "card-1" || strings.Contains(body, `id="not-for-me"`) || strings.Count(body, "<article ") != 2 {
		t.Fatal("changed vote did not restore recommendation")
	}
	picks, _ := db.GetPicksForOpportunity(ctx, ids[0])
	if picks[0].Score != .9 {
		t.Fatal("modified AI score")
	}
}
