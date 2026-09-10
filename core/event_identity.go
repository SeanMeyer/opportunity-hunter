package core

import (
	"strings"
	"time"
	_ "time/tzdata"
)

var denverEventLocation = func() *time.Location {
	l, err := time.LoadLocation("America/Denver")
	if err != nil {
		panic(err)
	}
	return l
}()

// EventLocalDate compares dates at the venue rather than the source's UTC offset.
func EventLocalDate(t time.Time) string {
	if IsDatePlaceholder(t) {
		return t.Format("2006-01-02")
	}
	return t.In(denverEventLocation).Format("2006-01-02")
}

// EventVenueKey recognizes the documented downtown alias only when its street
// address agrees. South and other similarly named venues remain distinct.
func EventVenueKey(name, address string) string {
	name = strings.Join(strings.Fields(strings.ToLower(name)), " ")
	address = strings.Join(strings.Fields(strings.ToLower(strings.ReplaceAll(address, ".", ""))), " ")
	address = strings.ReplaceAll(address, " street", " st")
	if (name == "comedy works" || name == "comedy works downtown") && (address == "1226 15th st" || strings.HasPrefix(address, "1226 15th st,")) {
		return "comedy works downtown|1226 15th st"
	}
	return name + "|" + address
}

// IsDatePlaceholder reports source dates that omit a performance time.
func IsDatePlaceholder(t time.Time) bool {
	return t.Hour() == 0 && t.Minute() == 0 && t.Second() == 0
}
