package sources

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/seanmeyer/opportunity-hunter/core"
	"github.com/seanmeyer/opportunity-hunter/hunts/powder/catalog"
)

// OpenMeteo fetches weather forecasts from the Open-Meteo API.
type OpenMeteo struct {
	regions []catalog.Region
}

func NewOpenMeteo(regions []catalog.Region) *OpenMeteo {
	return &OpenMeteo{regions: regions}
}

func (s *OpenMeteo) Name() string { return "open-meteo" }

func (s *OpenMeteo) Scan(ctx context.Context, _ core.ScanRegion) ([]core.RawItem, error) {
	var items []core.RawItem

	for _, region := range s.regions {
		// In production, this fetches from api.open-meteo.com with the region's coordinates.
		// For now, returns an empty item per region so the framework can process it.
		// Real weather data will populate snowfall windows.

		windowStart := time.Now().Add(2 * 24 * time.Hour)
		windowEnd := windowStart.Add(3 * 24 * time.Hour)

		attrs, _ := json.Marshal(map[string]any{
			"snowfall_in":    0, // placeholder — real data comes from API
			"friction_tier":  region.FrictionTier,
			"storm_group":    region.StormGroup,
			"weather_window": "near",
		})

		items = append(items, core.RawItem{
			SourceID:       fmt.Sprintf("om-%s-%s", region.ID, windowStart.Format("2006-01-02")),
			Source:         "open-meteo",
			Title:          region.Name,
			Subtitle:       fmt.Sprintf("%s — %s", windowStart.Format("Jan 2"), windowEnd.Format("Jan 2")),
			VenueName:      region.Name,
			VenueLatitude:  region.Latitude,
			VenueLongitude: region.Longitude,
			StartTime:      windowStart.Format(time.RFC3339),
			EndTime:        windowEnd.Format(time.RFC3339),
			Attributes:     attrs,
			RawJSON:        "{}",
		})
	}

	return items, nil
}
