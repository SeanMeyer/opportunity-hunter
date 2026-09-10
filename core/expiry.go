package core

import (
	"sort"
	"time"
)

// ListingDatePassed keeps date-only listings available through their calendar
// day. Midnight is the existing source convention for an unconfirmed showtime.
func ListingDatePassed(at, now time.Time) bool {
	if at.IsZero() {
		return true
	}
	if at.Hour() == 0 && at.Minute() == 0 && at.Second() == 0 {
		end := time.Date(at.Year(), at.Month(), at.Day()+1, 0, 0, 0, 0, now.Location())
		return !end.After(now)
	}
	return !at.After(now)
}

// OpportunityExpired is shared by evaluation, delivery and read-time UI filters.
// Hunt policies retain availability semantics such as streaming movies. A
// persisted expired state always suppresses an opportunity.
func OpportunityExpired(opp Opportunity, expirer Expirer, now time.Time) bool {
	if opp.State == Expired {
		return true
	}
	if expirer != nil {
		return expirer.ShouldExpire(opp)
	}
	last := opp.LastShowDate()
	if opp.EndTime != nil && opp.EndTime.After(last) {
		last = *opp.EndTime
	}
	return ListingDatePassed(last, now)
}

// UpcomingListing returns a presentation copy with remaining shows in date
// order. Timed listings display in the configured local timezone; date-only
// listings retain their calendar date and unconfirmed-time marker.
func UpcomingListing(opp Opportunity, now time.Time) Opportunity {
	local := func(at time.Time) time.Time {
		if at.Hour() == 0 && at.Minute() == 0 && at.Second() == 0 {
			return at
		}
		return at.In(now.Location())
	}
	dates := make([]time.Time, 0, len(opp.ShowDates))
	actualDays := make(map[string]bool)
	for _, d := range opp.ShowDates {
		if !IsDatePlaceholder(d) {
			actualDays[local(d).Format("2006-01-02")] = true
		}
	}
	seen := make(map[string]bool)
	for _, d := range opp.ShowDates {
		if ListingDatePassed(d, now) {
			continue
		}
		key := d.UTC().Format(time.RFC3339Nano)
		if IsDatePlaceholder(d) {
			day := d.Format("2006-01-02")
			if actualDays[day] {
				continue
			}
			key = "date:" + day
		}
		if seen[key] {
			continue
		}
		seen[key] = true
		dates = append(dates, local(d))
	}
	sort.Slice(dates, func(i, j int) bool { return dates[i].Before(dates[j]) })
	opp.ShowDates = dates
	opp.StartTime = local(opp.StartTime)
	if len(dates) > 0 {
		opp.StartTime = dates[0]
	}
	if opp.EndTime != nil {
		end := local(*opp.EndTime)
		opp.EndTime = &end
	}
	return opp
}
