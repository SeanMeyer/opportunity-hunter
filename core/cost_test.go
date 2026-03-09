package core_test

import (
	"testing"

	"github.com/seanmeyer/opportunity-hunter/core"
)

func TestCostTracker_NewEmpty(t *testing.T) {
	ct := core.NewCostTracker(0, nil)
	if ct.Total() != 0 {
		t.Fatalf("expected 0 total, got %f", ct.Total())
	}
}

func TestCostTracker_NewSeeded(t *testing.T) {
	ct := core.NewCostTracker(5.0, map[string]float64{"comedy": 3.0, "powder": 2.0})
	if ct.Total() != 5.0 {
		t.Fatalf("expected 5.0 total, got %f", ct.Total())
	}
	if ct.ForHunt("comedy") != 3.0 {
		t.Fatalf("expected 3.0 for comedy, got %f", ct.ForHunt("comedy"))
	}
}

func TestCostTracker_Add(t *testing.T) {
	ct := core.NewCostTracker(0, nil)
	ct.Add("comedy", 1.5)
	ct.Add("comedy", 0.5)
	ct.Add("powder", 2.0)

	if ct.Total() != 4.0 {
		t.Fatalf("expected 4.0 total, got %f", ct.Total())
	}
	if ct.ForHunt("comedy") != 2.0 {
		t.Fatalf("expected 2.0 for comedy, got %f", ct.ForHunt("comedy"))
	}
	if ct.ForHunt("powder") != 2.0 {
		t.Fatalf("expected 2.0 for powder, got %f", ct.ForHunt("powder"))
	}
	if ct.ForHunt("nonexistent") != 0 {
		t.Fatal("expected 0 for nonexistent hunt")
	}
}
