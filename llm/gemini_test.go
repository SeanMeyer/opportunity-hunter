package llm

import (
	"testing"
)

func TestEstimateCost_Flash(t *testing.T) {
	cost := EstimateCost("gemini-2.5-flash", 10000, 1000)
	if cost <= 0 {
		t.Fatal("expected positive cost")
	}
	// Should be cheap for flash model.
	if cost > 0.01 {
		t.Fatalf("flash cost seems too high: %f", cost)
	}
}

func TestEstimateCost_Pro(t *testing.T) {
	cost := EstimateCost("gemini-2.0-pro", 10000, 1000)
	if cost <= 0 {
		t.Fatal("expected positive cost")
	}
	// Pro should be more expensive than flash.
	flashCost := EstimateCost("gemini-2.5-flash", 10000, 1000)
	if cost <= flashCost {
		t.Fatal("pro should be more expensive than flash")
	}
}

func TestExtractSources_Nil(t *testing.T) {
	sources := extractSources(nil)
	if len(sources) != 0 {
		t.Fatal("expected empty sources for nil response")
	}
}
