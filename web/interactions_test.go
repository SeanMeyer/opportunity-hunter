package web

import (
	"context"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/seanmeyer/opportunity-hunter/core"
	"github.com/seanmeyer/opportunity-hunter/storage"
	"github.com/seanmeyer/opportunity-hunter/testutil"
)

func TestFormsRejectInvalidInput(t *testing.T) {
	db := testutil.NewTestDB(t)
	s, err := New(db, []HuntInfo{{Name: "comedy"}, {Name: "powder"}}, "")
	if err != nil {
		t.Fatal(err)
	}
	id, err := db.InsertOpportunity(context.Background(), core.Opportunity{HuntName: "comedy", SourceID: "test", Title: "Real title", State: core.Evaluated, DiscoveredAt: time.Now()})
	if err != nil {
		t.Fatal(err)
	}
	_ = id
	for _, tc := range []struct{ path, body string }{
		{"/preferences", "hunt=unknown&preferences=test"},
		{"/schedule", "hunt=unknown&interval=60"},
		{"/schedule", "hunt=comedy&interval=60&start_time=06:00junk"},
		{"/schedule", "hunt=comedy&interval=10080&start_day=1junk"},
		{"/feedback", "hunt=comedy&rating=garbage"},
		{"/feedback", "hunt=unknown&rating=up"},
		{"/feedback", "hunt=comedy&rating=up&opportunity_id=broken"},
		{"/feedback", "hunt=powder&rating=up&opportunity_id=1"},
	} {
		t.Run(tc.body, func(t *testing.T) {
			req := httptest.NewRequest("POST", tc.path, strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			w := httptest.NewRecorder()
			s.Handler().ServeHTTP(w, req)
			if w.Code != 400 {
				t.Fatalf("got %d, want 400", w.Code)
			}
		})
	}
}

type interactionRenderer struct{}

func (interactionRenderer) RenderCard(o core.Opportunity, p core.Pick, v core.Venue) core.CardData {
	return core.CardData{Title: o.Title, Reason: p.Reason, SortScore: p.Score, DateDisplay: "Saturday", Fields: []core.CardField{{Label: "Strategy", Value: "Long narrative belongs only in details"}, {Label: "Price", Value: "$35"}}}
}

func TestCardsRestoreLatestFeedbackAndSeparateDetails(t *testing.T) {
	db := testutil.NewTestDB(t)
	ctx := context.Background()
	now := time.Now()
	id, err := db.InsertOpportunity(ctx, core.Opportunity{HuntName: "comedy", SourceID: "card", Title: "Test card", State: core.Evaluated, DiscoveredAt: now, StartTime: now.Add(24 * time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.SaveEvaluationWithPicks(ctx, core.Evaluation{HuntName: "comedy", EvaluatedAt: now}, []core.Pick{{OpportunityID: id, Score: .9, Reason: "Good fit"}})
	if err != nil {
		t.Fatal(err)
	}
	for _, rating := range []string{"up", "down"} {
		_, err = db.SaveFeedback(ctx, storage.FeedbackRow{HuntName: "comedy", OpportunityID: &id, Rating: rating, Note: "Saved note", CreatedAt: now})
		if err != nil {
			t.Fatal(err)
		}
	}
	s, err := New(db, []HuntInfo{{Name: "comedy", CardRenderer: interactionRenderer{}}}, "")
	if err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, httptest.NewRequest("GET", "/?hunt=comedy", nil))
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	body := w.Body.String()
	for _, want := range []string{`data-saved-rating="down"`, `data-rating="down" aria-pressed="true"`, `value="Saved note"`, `<span class="card-meta-item">Saturday</span>`} {
		if !strings.Contains(body, want) {
			t.Errorf("missing %s", want)
		}
	}
	if strings.Count(body, "Long narrative belongs only in details") != 1 {
		t.Error("narrative repeated in summary")
	}
}

func TestFeedbackUsesStoredTitleAndPreservesView(t *testing.T) {
	db := testutil.NewTestDB(t)
	s, _ := New(db, []HuntInfo{{Name: "comedy"}}, "")
	_, err := db.InsertOpportunity(context.Background(), core.Opportunity{HuntName: "comedy", SourceID: "test", Title: "Real title", State: core.Evaluated, DiscoveredAt: time.Now()})
	if err != nil {
		t.Fatal(err)
	}
	body := url.Values{"hunt": {"comedy"}, "rating": {"up"}, "opportunity_id": {"1"}, "title": {"Wrong title"}, "sort": {"date"}, "filter": {"8"}}
	req := httptest.NewRequest("POST", "/feedback", strings.NewReader(body.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)
	if w.Code != 303 {
		t.Fatal(w.Code, w.Body.String())
	}
	loc, _ := url.Parse(w.Header().Get("Location"))
	if loc.Query().Get("sort") != "date" || loc.Query().Get("filter") != "8" || loc.Fragment != "card-1" {
		t.Errorf("lost view: %s", loc)
	}
	rows, err := db.GetRecentFeedback(context.Background(), "comedy", 10)
	if err != nil || len(rows) != 1 {
		t.Fatalf("feedback: %v %v", rows, err)
	}
	if rows[0].Title != "Real title" {
		t.Errorf("untrusted title saved: %s", rows[0].Title)
	}
}

func TestUnknownRoutesAndHunts(t *testing.T) {
	s, _ := New(testutil.NewTestDB(t), []HuntInfo{{Name: "comedy"}}, "")
	for _, path := range []string{"/missing", "/?hunt=missing"} {
		w := httptest.NewRecorder()
		s.Handler().ServeHTTP(w, httptest.NewRequest("GET", path, nil))
		if w.Code != 404 {
			t.Errorf("%s: got %d", path, w.Code)
		}
	}
}

func TestUndatedCardsSortLast(t *testing.T) {
	cards := []core.CardData{{Title: "Undated"}, {Title: "Tomorrow", DateSort: 200}, {Title: "Today", DateSort: 100}}
	sortCards(cards, core.SortByDate)
	if cards[0].Title != "Today" || cards[2].Title != "Undated" {
		t.Fatalf("wrong date order: %+v", cards)
	}
}

func TestManualRunPreservesView(t *testing.T) {
	called := make(chan string, 1)
	s, err := New(testutil.NewTestDB(t), []HuntInfo{{Name: "comedy"}}, "", func(_ context.Context, hunt string) { called <- hunt })
	if err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest("POST", "/run", strings.NewReader("hunt=comedy&sort=date&filter=8"))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, r)
	location, err := url.Parse(w.Header().Get("Location"))
	if err != nil || w.Code != 303 {
		t.Fatalf("run response: %d %v", w.Code, err)
	}
	if location.Query().Get("sort") != "date" || location.Query().Get("filter") != "8" {
		t.Fatal(location)
	}
	select {
	case hunt := <-called:
		if hunt != "comedy" {
			t.Fatal(hunt)
		}
	case <-time.After(time.Second):
		t.Fatal("run not triggered")
	}
}
