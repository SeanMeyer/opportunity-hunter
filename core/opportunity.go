package core

import (
	"encoding/json"
	"fmt"
	"time"
)

// Attributes is the type alias for hunt-specific JSON data.
// Core stores and passes it through without parsing.
// Each hunt defines typed structs with Encode/Decode helpers.
type Attributes = json.RawMessage

// State represents the lifecycle state of an opportunity.
type State string

const (
	Discovered State = "discovered"
	Evaluated  State = "evaluated"
	Notified   State = "notified"
	Reminded   State = "reminded"
	Expired    State = "expired"
)

// Opportunity is the universal entity that all hunts produce.
type Opportunity struct {
	ID           int64
	SupersededBy *int64 // Historical duplicate; active queries exclude aliases.
	HuntName     string
	SourceID     string
	Source       string
	Title        string
	Subtitle     string
	VenueID      *int64
	StartTime    time.Time // earliest date (backward compat for sorting)
	EndTime      *time.Time
	ShowDates    []time.Time // all performance dates; empty for non-merged events
	PriceMin     *float64
	PriceMax     *float64
	TicketURL    string
	State        State
	Attributes   Attributes
	RawData      string
	DiscoveredAt time.Time
	EvaluatedAt  *time.Time
	NotifiedAt   *time.Time
	RemindedAt   *time.Time
}

// LastShowDate returns the latest date from ShowDates, falling back to StartTime.
func (o *Opportunity) LastShowDate() time.Time {
	last := o.StartTime
	for _, d := range o.ShowDates {
		if d.After(last) {
			last = d
		}
	}
	return last
}

// NextShowDate returns the next future date from ShowDates, or StartTime if none.
func (o *Opportunity) NextShowDate() time.Time {
	now := time.Now()
	var next time.Time
	for _, d := range o.ShowDates {
		if d.After(now) && (next.IsZero() || d.Before(next)) {
			next = d
		}
	}
	if next.IsZero() {
		return o.StartTime
	}
	return next
}

// MarkEvaluated transitions to Evaluated state with timestamp.
func (o *Opportunity) MarkEvaluated(at time.Time) {
	o.State = Evaluated
	o.EvaluatedAt = &at
}

// MarkNotified transitions to Notified state with timestamp.
func (o *Opportunity) MarkNotified(at time.Time) {
	o.State = Notified
	o.NotifiedAt = &at
}

// MarkReminded transitions to Reminded state with timestamp.
func (o *Opportunity) MarkReminded(at time.Time) {
	o.State = Reminded
	o.RemindedAt = &at
}

// MarkExpired transitions to Expired state.
func (o *Opportunity) MarkExpired() {
	o.State = Expired
}

// Validate checks that the opportunity has required fields and consistent data.
func (o *Opportunity) Validate() error {
	if o.HuntName == "" {
		return fmt.Errorf("opportunity: HuntName is required")
	}
	if o.Title == "" {
		return fmt.Errorf("opportunity: Title is required")
	}
	if o.EndTime != nil && o.EndTime.Before(o.StartTime) {
		return fmt.Errorf("opportunity: EndTime %v is before StartTime %v", o.EndTime, o.StartTime)
	}
	return nil
}

// Venue is a physical location shared across hunts.
type Venue struct {
	ID             int64
	Name           string
	Address        string
	Latitude       float64
	Longitude      float64
	Notes          string
	WalkingMinutes int     // 0 = unknown; enriched from distance_cache
	DrivingMinutes int     // enriched from distance_cache (mode=driving)
	DistanceMi     float64 // 0 = unknown; enriched from distance_cache
}
