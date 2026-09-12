package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/seanmeyer/opportunity-hunter/core"
	"github.com/seanmeyer/opportunity-hunter/llm"
	"github.com/seanmeyer/opportunity-hunter/testutil"
	"testing"
	"time"
)

func TestFailedResearchRetainsCost(t *testing.T) {
	for _, e := range []error{fmt.Errorf("extraction failed"), nil} {
		r, err := finishResearch(record{}, llm.TwoStepResult{CostUSD: .12}, e)
		if err == nil || r.Research.CostUSD != .12 {
			t.Fatalf("lost failed cost %+v %v", r, err)
		}
	}
}
func TestIdentityRejectsChangedProduction(t *testing.T) {
	a := core.Opportunity{ID: 1, Title: "Hamilton", Subtitle: "Original Broadway", Attributes: []byte(`{"year":2016}`)}
	b := a
	b.Subtitle = "Local cast"
	if sameIdentity(a, b) {
		t.Fatal("accepted cast change")
	}
	b = a
	b.Attributes = []byte(`{"year":2026}`)
	if sameIdentity(a, b) {
		t.Fatal("accepted year change")
	}
}
func TestCandidatesSkipExpiredAndEnriched(t *testing.T) {
	db := testutil.NewTestDB(t)
	ctx := context.Background()
	for i := 0; i < 3; i++ {
		at := time.Now().Add(time.Hour)
		if i == 0 {
			at = time.Now().Add(-time.Hour)
		}
		id, e := db.InsertOpportunity(ctx, core.Opportunity{HuntName: "comedy", Title: fmt.Sprint(i), SourceID: fmt.Sprint(i), State: core.Evaluated, DiscoveredAt: time.Now(), StartTime: at})
		if e != nil {
			t.Fatal(e)
		}
		var attrs core.Attributes
		if i == 1 {
			attrs = core.WithReviewEvidence(nil, []core.ReviewEvidence{{Kind: "review", Title: "source", Subject: "work", Summary: "text", URL: "https://example.com/review"}})
		}
		if _, e = db.SaveEvaluationWithPicks(ctx, core.Evaluation{HuntName: "comedy", EvaluatedAt: time.Now()}, []core.Pick{{OpportunityID: id, Attributes: attrs}}); e != nil {
			t.Fatal(e)
		}
	}
	rows, e := candidates(ctx, db)
	if e != nil || len(rows) != 1 || rows[0].Opportunity.Title != "2" {
		t.Fatalf("candidates %+v %v", rows, e)
	}
}

func TestCheckpointFormattingDoesNotChangeIdentity(t *testing.T) {
	r := record{Opportunity: core.Opportunity{ID: 1, Attributes: []byte(`{"year":2016,"cast":"A & B <C>"}`)}, Pick: core.Pick{Attributes: []byte(`{"genre":"play"}`)}}
	b, e := json.MarshalIndent(r, "", "  ")
	if e != nil {
		t.Fatal(e)
	}
	var saved record
	if e = json.Unmarshal(b, &saved); e != nil {
		t.Fatal(e)
	}
	if !sameIdentity(r.Opportunity, saved.Opportunity) || !sameJSON(r.Pick.Attributes, saved.Pick.Attributes) {
		t.Fatal("checkpoint formatting invalidated identity")
	}
}
