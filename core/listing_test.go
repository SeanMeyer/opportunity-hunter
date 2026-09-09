package core

import (
	"strings"
	"testing"
	"time"
)

func TestListingScheduleDoesNotInventTimes(t *testing.T) {
	for _, tc := range []struct {
		at   time.Time
		want string
	}{{time.Time{}, "Unknown"}, {time.Date(2026, 9, 12, 0, 0, 0, 0, time.UTC), "time unconfirmed"}, {time.Date(2026, 9, 12, 19, 30, 0, 0, time.UTC), "7:30 PM"}} {
		if got := FormatListingTime(tc.at); !strings.Contains(got, tc.want) {
			t.Errorf("%s: %q", tc.want, got)
		}
	}
}
