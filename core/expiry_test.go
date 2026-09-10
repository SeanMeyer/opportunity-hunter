package core

import (
	"testing"
	"time"
)

type neverExpires struct{}

func (neverExpires) ShouldExpire(Opportunity) bool { return false }

func TestOpportunityExpiryWindowsAndPolicies(t *testing.T) {
	now := time.Date(2026, 9, 9, 18, 0, 0, 0, time.FixedZone("MDT", -6*3600))
	past, future := now.Add(-24*time.Hour), now.Add(24*time.Hour)
	cases := []struct {
		name   string
		opp    Opportunity
		custom Expirer
		want   bool
	}{
		{"past", Opportunity{StartTime: past}, nil, true},
		{"future", Opportunity{StartTime: future}, nil, false},
		{"ongoing", Opportunity{StartTime: past, EndTime: &future}, nil, false},
		{"merged remaining", Opportunity{StartTime: past, ShowDates: []time.Time{future, past}}, nil, false},
		{"streaming", Opportunity{StartTime: past}, neverExpires{}, false},
		{"persisted expired", Opportunity{StartTime: future, State: Expired}, neverExpires{}, true},
		{"date only today", Opportunity{StartTime: time.Date(2026, 9, 9, 0, 0, 0, 0, time.UTC)}, nil, false},
		{"date only yesterday", Opportunity{StartTime: time.Date(2026, 9, 8, 0, 0, 0, 0, time.UTC)}, nil, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := OpportunityExpired(tc.opp, tc.custom, now); got != tc.want {
				t.Fatalf("expired=%v want%v", got, tc.want)
			}
		})
	}
}

func TestUpcomingListingSortsFiltersAndConvertsTimedDates(t *testing.T) {
	loc := time.FixedZone("MDT", -6*3600)
	now := time.Date(2026, 9, 9, 18, 0, 0, 0, loc)
	early := time.Date(2026, 9, 27, 2, 0, 0, 0, time.UTC)
	late := early.Add(24 * time.Hour)
	past := now.Add(-24 * time.Hour)
	original := Opportunity{StartTime: past, ShowDates: []time.Time{late, past, early}}
	got := UpcomingListing(original, now)
	if len(got.ShowDates) != 2 || !got.StartTime.Equal(early) || got.StartTime.Hour() != 20 || got.StartTime.Day() != 26 || !got.ShowDates[1].Equal(late) {
		t.Fatalf("bad upcoming dates: %+v", got)
	}
	if len(original.ShowDates) != 3 || !original.ShowDates[0].Equal(late) {
		t.Fatal("mutated stored dates")
	}
}

func TestUpcomingListingDeduplicatesPlaceholdersAndInstants(t *testing.T) {
	loc := time.FixedZone("MST", -7*3600)
	now := time.Date(2026, 11, 1, 12, 0, 0, 0, loc)
	actual := time.Date(2026, 11, 8, 2, 0, 0, 0, time.UTC)
	late := actual.Add(3 * time.Hour)
	placeholder := time.Date(2026, 11, 7, 0, 0, 0, 0, time.UTC)
	original := Opportunity{StartTime: placeholder, ShowDates: []time.Time{placeholder, actual, actual.In(loc), late}}
	got := UpcomingListing(original, now)
	if len(got.ShowDates) != 2 || !got.ShowDates[0].Equal(actual) || !got.ShowDates[1].Equal(late) {
		t.Fatalf("duplicate or missing times: %+v", got.ShowDates)
	}
	if len(original.ShowDates) != 4 {
		t.Fatal("mutated stored history")
	}
}
