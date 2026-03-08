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
	HuntName     string
	SourceID     string
	Source       string
	Title        string
	Subtitle     string
	VenueID      *int64
	StartTime    time.Time
	EndTime      *time.Time
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
	ID        int64
	Name      string
	Address   string
	Latitude  float64
	Longitude float64
	Notes     string
}
