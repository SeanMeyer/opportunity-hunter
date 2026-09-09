package movies

import (
	"github.com/seanmeyer/opportunity-hunter/core"
	"strings"
	"testing"
	"time"
)

func TestPromptPreservesListingConstraints(t *testing.T) {
	price := 42.0
	p := buildPrompt(core.EvalContext{Opportunities: []core.Opportunity{{Title: "Film A", StartTime: time.Date(2026, 9, 12, 21, 30, 0, 0, time.UTC), PriceMin: &price}, {Title: "Film B"}}}, nil, 0, 0)
	for _, want := range []string{"$42", "2026", "9:30 PM", "Unknown"} {
		if !strings.Contains(p, want) {
			t.Errorf("missing %q in prompt", want)
		}
	}
	if strings.Contains(p, "0001") {
		t.Fatal("fabricated date")
	}
}
