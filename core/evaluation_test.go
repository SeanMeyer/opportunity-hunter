package core_test

import (
	"testing"
	"time"

	"github.com/seanmeyer/opportunity-hunter/core"
)

func TestEvalContext_Validate_Valid(t *testing.T) {
	venueID := int64(1)
	ec := core.EvalContext{
		Opportunities: []core.Opportunity{
			{ID: 1, HuntName: "test", VenueID: &venueID},
		},
		Venues:      map[int64]core.Venue{1: {ID: 1, Name: "Test Venue"}},
		CostTracker: core.NewCostTracker(0, nil),
	}
	if err := ec.Validate(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestEvalContext_Validate_EmptyOpportunities(t *testing.T) {
	ec := core.EvalContext{
		Opportunities: nil,
		CostTracker:   core.NewCostTracker(0, nil),
	}
	if err := ec.Validate(); err == nil {
		t.Fatal("expected error for empty opportunities")
	}
}

func TestEvalContext_Validate_MissingVenue(t *testing.T) {
	venueID := int64(99)
	ec := core.EvalContext{
		Opportunities: []core.Opportunity{
			{ID: 1, HuntName: "test", VenueID: &venueID},
		},
		Venues:      map[int64]core.Venue{1: {ID: 1}},
		CostTracker: core.NewCostTracker(0, nil),
	}
	if err := ec.Validate(); err == nil {
		t.Fatal("expected error for missing venue ID 99")
	}
}

func TestEvalContext_Validate_NilVenueIDOK(t *testing.T) {
	ec := core.EvalContext{
		Opportunities: []core.Opportunity{
			{ID: 1, HuntName: "movies", VenueID: nil},
		},
		Venues:      map[int64]core.Venue{},
		CostTracker: core.NewCostTracker(0, nil),
	}
	if err := ec.Validate(); err != nil {
		t.Fatalf("nil VenueID should be allowed: %v", err)
	}
}

func TestEvalContext_Validate_NilCostTracker(t *testing.T) {
	ec := core.EvalContext{
		Opportunities: []core.Opportunity{
			{ID: 1, HuntName: "test"},
		},
	}
	if err := ec.Validate(); err == nil {
		t.Fatal("expected error for nil CostTracker")
	}
}

func TestEvaluation_Fields(t *testing.T) {
	eval := core.Evaluation{
		ID:             1,
		HuntName:       "comedy",
		GroupKey:        "2026-W10",
		EvaluatedAt:    time.Now(),
		RawLLMResponse: `{"picks": []}`,
		CostUSD:        0.003,
	}
	if eval.HuntName != "comedy" {
		t.Fatal("HuntName mismatch")
	}
	if eval.CostUSD != 0.003 {
		t.Fatal("CostUSD mismatch")
	}
}

func TestPick_Fields(t *testing.T) {
	pick := core.Pick{
		ID:            1,
		EvaluationID:  10,
		OpportunityID: 20,
		Score:         0.85,
		DisplayScore:  "8/10",
		Reason:        "Great comedian",
		Urgency:       "May sell out",
	}
	if pick.Score != 0.85 {
		t.Fatal("Score mismatch")
	}
	if pick.DisplayScore != "8/10" {
		t.Fatal("DisplayScore mismatch")
	}
}

func TestGroup_Structure(t *testing.T) {
	g := core.Group{
		Key: "2026-W10",
		Opportunities: []core.Opportunity{
			{ID: 1, Title: "Show A"},
			{ID: 2, Title: "Show B"},
		},
		Venues: map[int64]core.Venue{},
	}
	if len(g.Opportunities) != 2 {
		t.Fatal("expected 2 opportunities in group")
	}
}

func TestFeedbackEntry_Fields(t *testing.T) {
	fb := core.FeedbackEntry{
		OpportunityTitle: "Nate Bargatze",
		Rating:           "loved",
		Note:             "Hilarious show",
	}
	if fb.Rating != "loved" {
		t.Fatal("Rating mismatch")
	}
}
