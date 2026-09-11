package movies

import (
	"github.com/seanmeyer/opportunity-hunter/core"
	"strings"
	"testing"
)

func TestMovieTravelUsesDriving(t *testing.T) {
	id := int64(1)
	v := core.Venue{WalkingMinutes: 180, DrivingMinutes: 20, DistanceMi: 7, DrivingDistanceMi: 12}
	card := (&MoviesHunt{}).CardRenderer().RenderCard(core.Opportunity{}, core.Pick{}, v)
	found := false
	for _, f := range card.Fields {
		if f.Label == "Distance" {
			found = true
			if f.Value != "20 min drive · 12.0 mi" {
				t.Errorf("wrong route %s", f.Value)
			}
		}
	}
	if !found {
		t.Fatal("missing route")
	}
	p := buildPrompt(core.EvalContext{Opportunities: []core.Opportunity{{VenueID: &id}}, Venues: map[int64]core.Venue{1: v}}, nil, 0, 0)
	if strings.Contains(p, "180 min walk") || !strings.Contains(p, "20 min drive") {
		t.Fatal("prompt has wrong route")
	}
}
