package core

import (
	"testing"
	"time"
)

func TestEventLocalDatePreservesDateOnlyAndConvertsActualTimes(t *testing.T) {
	for _, tc := range []struct {
		raw, date   string
		placeholder bool
	}{{"2026-10-16T02:00:00Z", "2026-10-15", false}, {"2026-10-15T00:00:00Z", "2026-10-15", true}, {"2026-10-15T00:00:00-06:00", "2026-10-15", true}} {
		at, _ := time.Parse(time.RFC3339, tc.raw)
		if EventLocalDate(at) != tc.date || IsDatePlaceholder(at) != tc.placeholder {
			t.Fatalf("%s date=%s placeholder=%v", tc.raw, EventLocalDate(at), IsDatePlaceholder(at))
		}
	}
}
