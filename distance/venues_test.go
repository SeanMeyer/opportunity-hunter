package distance

import (
	"context"
	"github.com/seanmeyer/opportunity-hunter/core"
	"io"
	"net/http"
	"strings"
	"testing"
)

type venueTransport func(*http.Request) (*http.Response, error)

func (f venueTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestCachedRoutesAndDrivingFailure(t *testing.T) {
	calls := 0
	c := NewClient("test", &http.Client{Transport: venueTransport(func(r *http.Request) (*http.Response, error) {
		calls++
		return &http.Response{StatusCode: 503, Body: io.NopCloser(strings.NewReader("")), Header: make(http.Header)}, nil
	})})
	venues := map[int64]core.Venue{1: {Address: "far", WalkingMinutes: 373, DistanceMi: 15.7, DrivingMinutes: 21, DrivingDistanceMi: 16}, 2: {Address: "near", WalkingMinutes: 30, DistanceMi: 1}}
	c.EnrichVenues(context.Background(), "origin", venues)
	if calls != 0 {
		t.Fatal("cached routes caused HTTP request")
	}
	venues[3] = core.Venue{Address: "failed", WalkingMinutes: 31, DistanceMi: 2}
	c.EnrichVenues(context.Background(), "origin", venues)
	if calls != 1 || venues[3].DrivingMinutes != 0 || core.VenueTravel(venues[3]).Value != "Drive · time unavailable" {
		t.Fatal("driving failure misrepresented")
	}
}
