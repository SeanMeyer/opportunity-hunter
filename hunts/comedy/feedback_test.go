package comedy

import (
	"github.com/seanmeyer/opportunity-hunter/core"
	"strings"
	"testing"
)

func TestThumbFeedbackRetainsPolarity(t *testing.T) {
	p := buildPrompt(core.EvalContext{Feedback: []core.FeedbackEntry{{Rating: "up", OpportunityTitle: "Loved act"}, {Rating: "down", OpportunityTitle: "Bad match"}}})
	if !strings.Contains(p, "Good recommendation: Loved act") || !strings.Contains(p, "Bad recommendation: Bad match") {
		t.Fatalf("feedback polarity lost: %s", p)
	}
}
