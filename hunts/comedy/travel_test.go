package comedy

import (
	"context"
	"github.com/seanmeyer/opportunity-hunter/core"
	"github.com/seanmeyer/opportunity-hunter/distance"
	"io"
	"net/http"
	"strings"
	"testing"
)

type routeTransport func(*http.Request) (*http.Response, error)

func (f routeTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestLongWalkFetchesDrive(t *testing.T) {
	client := distance.NewClient("test", &http.Client{Transport: routeTransport(func(r *http.Request) (*http.Response, error) {
		b, _ := io.ReadAll(r.Body)
		response := `[{"condition":"ROUTE_EXISTS","duration":"22380s","distanceMeters":25000}]`
		if strings.Contains(string(b), `"DRIVE"`) {
			response = `[{"condition":"ROUTE_EXISTS","duration":"1800s","distanceMeters":32000}]`
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(response)), Header: make(http.Header)}, nil
	})})
	h := &ComedyHunt{distClient: client, homeAddress: "origin"}
	venues := map[int64]core.Venue{1: {ID: 1, Address: "venue"}}
	h.EnrichVenues(context.Background(), venues)
	if venues[1].DrivingMinutes != 30 {
		t.Fatalf("drive missing: %+v", venues[1])
	}
	card := h.CardRenderer().RenderCard(core.Opportunity{}, core.Pick{}, venues[1])
	for _, f := range card.Fields {
		if f.Label == "Distance" && !strings.Contains(f.Value, "30 min drive") {
			t.Fatalf("wrong travel: %s", f.Value)
		}
	}
}
func TestUnavailableDriveDoesNotRecommendLongWalk(t *testing.T) {
	card := (&ComedyHunt{}).CardRenderer().RenderCard(core.Opportunity{}, core.Pick{}, core.Venue{WalkingMinutes: 373, DistanceMi: 15.7})
	for _, f := range card.Fields {
		if f.Label == "Distance" && strings.Contains(f.Value, "373 min walk") {
			t.Fatal("unreasonable walk presented as travel recommendation")
		}
	}
}
func TestPromptUsesDrivingForLongTrip(t *testing.T) {
	id := int64(1)
	p := buildPrompt(core.EvalContext{Opportunities: []core.Opportunity{{Title: "Test", VenueID: &id}}, Venues: map[int64]core.Venue{1: {WalkingMinutes: 373, DrivingMinutes: 30}}})
	if strings.Contains(p, "373 min walk") || !strings.Contains(p, "30 min drive") {
		t.Fatal("prompt does not use appropriate driving route")
	}
}
