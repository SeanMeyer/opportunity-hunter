package movies

import (
	"github.com/seanmeyer/opportunity-hunter/core"
	"strings"
	"testing"
)

func TestThumbFeedbackRetainsPolarity(t *testing.T) {
	p := buildPrompt(core.EvalContext{Feedback: []core.FeedbackEntry{{Rating: "up", OpportunityTitle: "Loved movie"}, {Rating: "down", OpportunityTitle: "Bad match"}}}, nil, 0, 0)
	if !strings.Contains(p, "Good recommendation: Loved movie") || !strings.Contains(p, "Bad recommendation: Bad match") {
		t.Fatalf("feedback polarity lost: %s", p)
	}
}
