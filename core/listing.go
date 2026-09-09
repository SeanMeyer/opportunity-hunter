package core

import (
	"fmt"
	"strings"
	"time"
)

// FormatListingTime keeps date-only feeds from claiming a midnight showtime.
func FormatListingTime(t time.Time) string {
	if t.IsZero() {
		return "Unknown"
	}
	if t.Hour() == 0 && t.Minute() == 0 && t.Second() == 0 {
		return t.Format("Mon Jan 2, 2006") + " (time unconfirmed)"
	}
	return t.Format("Mon Jan 2, 2006, 3:04 PM MST")
}

// ListingFacts preserves price and schedule evidence across event prompts.
func ListingFacts(o Opportunity) string {
	dates := o.ShowDates
	if len(dates) == 0 {
		dates = []time.Time{o.StartTime}
	}
	values := make([]string, 0, len(dates))
	for _, d := range dates {
		values = append(values, FormatListingTime(d))
	}
	price := "Unknown"
	if o.PriceMin != nil {
		price = fmt.Sprintf("from $%.0f", *o.PriceMin)
	}
	if o.PriceMax != nil {
		if o.PriceMin != nil {
			price = fmt.Sprintf("$%.0f - $%.0f", *o.PriceMin, *o.PriceMax)
		} else {
			price = fmt.Sprintf("up to $%.0f", *o.PriceMax)
		}
	}
	return fmt.Sprintf("- Listing date/time: %s\n- Price: %s\n", strings.Join(values, ", "), price)
}

const RecommendationEvidenceRules = `Respect explicit price, travel, and schedule constraints. Unknown values cannot establish that a hard constraint is met; state what must be checked. Do not infer ticket scarcity, sellout risk, booking deadlines, or current availability from fame alone. Ground urgency in supplied or retrieved evidence; otherwise recommend checking details without invented pressure. Feedback changes suitability, not the underlying facts.`
