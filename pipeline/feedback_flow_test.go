package pipeline_test

import (
	"context"
	"fmt"
	"github.com/seanmeyer/opportunity-hunter/core"
	"github.com/seanmeyer/opportunity-hunter/hunts/fake"
	"github.com/seanmeyer/opportunity-hunter/pipeline"
	"github.com/seanmeyer/opportunity-hunter/testutil"
	"github.com/seanmeyer/opportunity-hunter/web"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestFeedbackFormToNextEvaluationUsesLatestChoice(t *testing.T) {
	db := testutil.NewTestDB(t)
	ctx := context.Background()
	now := time.Now()
	id, err := db.InsertOpportunity(ctx, core.Opportunity{HuntName: "comedy", SourceID: "past", Title: "Past show", State: core.Evaluated, DiscoveredAt: now})
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.SaveEvaluationWithPicks(ctx, core.Evaluation{HuntName: "comedy", EvaluatedAt: now}, []core.Pick{{OpportunityID: id, DisplayScore: "9/10", Reason: "Original assessment"}})
	if err != nil {
		t.Fatal(err)
	}
	s, err := web.New(db, []web.HuntInfo{{Name: "comedy"}}, "")
	if err != nil {
		t.Fatal(err)
	}
	for _, rating := range []string{"up", "down"} {
		form := url.Values{"hunt": {"comedy"}, "opportunity_id": {fmt.Sprint(id)}, "rating": {rating}, "note": {"Too far away"}}
		r := httptest.NewRequest("POST", "/feedback", strings.NewReader(form.Encode()))
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w := httptest.NewRecorder()
		s.Handler().ServeHTTP(w, r)
		if w.Code != 303 {
			t.Fatalf("save: %d %s", w.Code, w.Body.String())
		}
	}
	evaluator := &testutil.FakeEvaluator{}
	hunt := &fake.FakeHunt{HuntName: "comedy", Eval: evaluator, Srcs: []core.Source{&fake.FakeSource{SourceName: "test", Items: []core.RawItem{{SourceID: "future", Title: "Future show", StartTime: now.Add(48 * time.Hour).Format(time.RFC3339)}}}}}
	p := pipeline.New(db, core.NewCostTracker(0, nil), &testutil.FakeNotifier{}, core.ScanRegion{}, "")
	result := p.Run(ctx, hunt)
	if len(result.Errors) > 0 || len(evaluator.Calls) != 1 {
		t.Fatalf("pipeline: %+v, calls=%d", result, len(evaluator.Calls))
	}
	feedback := evaluator.Calls[0].Feedback
	if len(feedback) != 1 {
		t.Fatalf("received %d judgments, want 1: %+v", len(feedback), feedback)
	}
	fb := feedback[0]
	if fb.Rating != "down" || fb.Note != "Too far away" || fb.OpportunityTitle != "Past show" || fb.EvalScore != "9/10" || fb.EvalSummary != "Original assessment" {
		t.Fatalf("lost feedback context: %+v", fb)
	}
	prompt := core.FormatFeedback(feedback)
	if !strings.Contains(prompt, "Bad recommendation") || strings.Contains(prompt, "Good recommendation") {
		t.Fatalf("wrong prompt polarity: %s", prompt)
	}
}
