package comedy

import (
	"github.com/seanmeyer/opportunity-hunter/core"
	"strings"
	"testing"
	"time"
)

func TestDateDisplayDistinguishesSameDayShowtimes(t *testing.T) {
	at := time.Date(2026, 11, 7, 19, 0, 0, 0, time.FixedZone("MST", -7*3600))
	got := formatDateDisplay(core.Opportunity{StartTime: at, ShowDates: []time.Time{at, at.Add(3 * time.Hour)}})
	if !strings.Contains(got, "7:00 PM") || !strings.Contains(got, "10:00 PM") {
		t.Fatalf("showtimes indistinguishable: %s", got)
	}
	got = formatDateDisplay(core.Opportunity{StartTime: at, ShowDates: []time.Time{at, at.Add(24 * time.Hour)}})
	if got != "Sat Nov 7, Sun Nov 8" {
		t.Fatalf("ordinary dates no longer compact: %s", got)
	}
}
