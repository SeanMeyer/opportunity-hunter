package core_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/seanmeyer/opportunity-hunter/core"
)

func TestOpportunity_MarkEvaluated(t *testing.T) {
	opp := core.Opportunity{HuntName: "test", State: core.Discovered}
	now := time.Now()
	opp.MarkEvaluated(now)

	if opp.State != core.Evaluated {
		t.Fatalf("want Evaluated, got %s", opp.State)
	}
	if opp.EvaluatedAt == nil || !opp.EvaluatedAt.Equal(now) {
		t.Fatal("EvaluatedAt not set correctly")
	}
}

func TestOpportunity_MarkNotified(t *testing.T) {
	opp := core.Opportunity{HuntName: "test", State: core.Evaluated}
	now := time.Now()
	opp.MarkNotified(now)

	if opp.State != core.Notified {
		t.Fatalf("want Notified, got %s", opp.State)
	}
	if opp.NotifiedAt == nil || !opp.NotifiedAt.Equal(now) {
		t.Fatal("NotifiedAt not set correctly")
	}
}

func TestOpportunity_MarkReminded(t *testing.T) {
	opp := core.Opportunity{HuntName: "test", State: core.Notified}
	now := time.Now()
	opp.MarkReminded(now)

	if opp.State != core.Reminded {
		t.Fatalf("want Reminded, got %s", opp.State)
	}
	if opp.RemindedAt == nil || !opp.RemindedAt.Equal(now) {
		t.Fatal("RemindedAt not set correctly")
	}
}

func TestOpportunity_MarkExpired(t *testing.T) {
	opp := core.Opportunity{HuntName: "test", State: core.Notified}
	opp.MarkExpired()

	if opp.State != core.Expired {
		t.Fatalf("want Expired, got %s", opp.State)
	}
}

func TestOpportunity_Validate_Valid(t *testing.T) {
	opp := core.Opportunity{
		HuntName:     "comedy",
		Title:        "Test Show",
		State:        core.Discovered,
		StartTime:    time.Now().Add(24 * time.Hour),
		DiscoveredAt: time.Now(),
	}
	if err := opp.Validate(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestOpportunity_Validate_EmptyHuntName(t *testing.T) {
	opp := core.Opportunity{
		Title:        "Test Show",
		State:        core.Discovered,
		StartTime:    time.Now(),
		DiscoveredAt: time.Now(),
	}
	if err := opp.Validate(); err == nil {
		t.Fatal("expected error for empty HuntName")
	}
}

func TestOpportunity_Validate_EndTimeBeforeStartTime(t *testing.T) {
	start := time.Now().Add(24 * time.Hour)
	end := time.Now()
	opp := core.Opportunity{
		HuntName:     "test",
		Title:        "Test Show",
		State:        core.Discovered,
		StartTime:    start,
		EndTime:      &end,
		DiscoveredAt: time.Now(),
	}
	if err := opp.Validate(); err == nil {
		t.Fatal("expected error for EndTime before StartTime")
	}
}

func TestAttributes_RoundTrip(t *testing.T) {
	type TestAttrs struct {
		Score float64 `json:"score"`
		Name  string  `json:"name"`
	}
	attrs := TestAttrs{Score: 8.5, Name: "test"}
	raw, err := json.Marshal(attrs)
	if err != nil {
		t.Fatal(err)
	}

	var decoded TestAttrs
	if err := json.Unmarshal(core.Attributes(raw), &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Score != 8.5 || decoded.Name != "test" {
		t.Fatalf("round trip failed: got %+v", decoded)
	}
}

func TestState_Constants(t *testing.T) {
	states := []core.State{
		core.Discovered,
		core.Evaluated,
		core.Notified,
		core.Reminded,
		core.Expired,
	}
	for _, s := range states {
		if s == "" {
			t.Fatal("state constant is empty")
		}
	}
}
