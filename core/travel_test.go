package core

import (
	"strings"
	"testing"
)

func TestVenueTravelModeAndMileage(t *testing.T) {
	for _, tc := range []struct {
		name string
		v    Venue
		want string
	}{
		{"walk boundary", Venue{WalkingMinutes: 30, DistanceMi: 1.2, DrivingMinutes: 5, DrivingDistanceMi: 2.3}, "30 min walk · 1.2 mi"},
		{"drive boundary", Venue{WalkingMinutes: 31, DistanceMi: 1.2, DrivingMinutes: 5, DrivingDistanceMi: 2.3}, "5 min drive · 2.3 mi"},
		{"missing drive", Venue{WalkingMinutes: 373, DistanceMi: 15.7}, "Drive · time unavailable"},
		{"no route", Venue{}, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := VenueTravel(tc.v)
			if f.Value != tc.want {
				t.Fatalf("got %q want %q", f.Value, tc.want)
			}
			if strings.HasPrefix(f.Value, "0 min") {
				t.Fatal("unknown time displayed")
			}
		})
	}
}
