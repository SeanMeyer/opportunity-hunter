package core

import "context"

// Source fetches raw items from an external provider.
type Source interface {
	Name() string
	Scan(ctx context.Context, region ScanRegion) ([]RawItem, error)
}

// ScanRegion defines the geographic area to scan.
type ScanRegion struct {
	Latitude  float64
	Longitude float64
	RadiusMi  int
}

// RawItem is the normalized output from any Source, before deduplication.
type RawItem struct {
	SourceID       string
	Source         string
	Title          string
	Subtitle       string
	VenueName      string
	VenueAddress   string
	VenueLatitude  float64
	VenueLongitude float64
	StartTime      string // RFC3339
	EndTime        string // RFC3339, empty for single events
	PriceMin       *float64
	PriceMax       *float64
	TicketURL      string
	RawJSON        string
	Attributes     Attributes
}
