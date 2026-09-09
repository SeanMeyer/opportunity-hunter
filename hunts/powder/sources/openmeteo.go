package sources

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/seanmeyer/opportunity-hunter/core"
	"github.com/seanmeyer/opportunity-hunter/hunts/powder/catalog"
	"github.com/seanmeyer/opportunity-hunter/hunts/powder/weather"
)

// WeatherSource fetches real weather data from Open-Meteo and NWS,
// runs detection against friction-tier thresholds, and produces
// RawItems for each detected snowfall window.
type WeatherSource struct {
	service *weather.Service
	catalog []catalog.RegionWithResorts
}

func NewWeatherSource(service *weather.Service, cat []catalog.RegionWithResorts) *WeatherSource {
	return &WeatherSource{service: service, catalog: cat}
}

func (s *WeatherSource) Name() string { return "weather" }

func (s *WeatherSource) Scan(ctx context.Context, _ core.ScanRegion) ([]core.RawItem, error) {
	var items []core.RawItem
	var failures []error
	now := time.Now().UTC()

	for _, rr := range s.catalog {
		region := rr.Region
		resorts := rr.Resorts

		// Fetch weather from all sources (Open-Meteo + NWS for US).
		result, err := s.service.FetchAll(ctx, region, resorts)
		if err != nil {
			slog.Warn("weather fetch failed for region", "region_id", region.ID, "error", err)
			failures = append(failures, fmt.Errorf("region %s: %w", region.ID, err))
			continue
		}
		if len(result.Forecasts) == 0 {
			continue
		}

		// Run detection against thresholds.
		detection := weather.Detect(region, result.Forecasts, now)
		if !detection.Detected {
			continue
		}

		// Compute consensus for the region.
		var omForecasts []weather.Forecast
		for _, f := range result.Forecasts {
			if f.Source == "open_meteo" {
				omForecasts = append(omForecasts, f)
			}
		}
		consensus := weather.ComputeConsensus(omForecasts)

		// Build snapshot with full weather context.
		snapshot := weather.ScanSnapshot{
			Forecasts:  result.Forecasts,
			Discussion: result.Discussion,
			Consensus:  consensus,
			Resorts:    resorts,
			Detection:  detection,
			ScannedAt:  now,
		}
		snapshotJSON, _ := json.Marshal(snapshot)

		// Create one opportunity per detected window.
		for _, window := range detection.Windows {
			windowType := "near"
			if !window.IsNearRange {
				windowType = "extended"
			}

			attrs, _ := json.Marshal(map[string]any{
				"snowfall_in":    window.TotalIn,
				"friction_tier":  region.FrictionTier,
				"storm_group":    region.StormGroup,
				"weather_window": windowType,
				"consensus":      consensusAgreement(consensus),
			})

			startStr := window.StartDate.Format(time.RFC3339)
			endStr := window.EndDate.Format(time.RFC3339)

			items = append(items, core.RawItem{
				SourceID:       fmt.Sprintf("wx-%s-%s-%s", region.ID, windowType, window.StartDate.Format("2006-01-02")),
				Source:         "weather",
				Title:          region.Name,
				Subtitle:       fmt.Sprintf("%.0f\" %s — %s", window.TotalIn, window.StartDate.Format("Jan 2"), window.EndDate.Format("Jan 2")),
				VenueName:      region.Name,
				VenueLatitude:  region.Latitude,
				VenueLongitude: region.Longitude,
				StartTime:      startStr,
				EndTime:        endStr,
				Attributes:     attrs,
				RawJSON:        string(snapshotJSON),
			})
		}
	}

	return items, errors.Join(failures...)
}

// consensusAgreement returns a 0-1 value representing overall model agreement.
func consensusAgreement(consensus weather.ModelConsensus) float64 {
	if len(consensus.DailyConsensus) == 0 {
		return 0
	}
	var highCount int
	for _, dc := range consensus.DailyConsensus {
		if dc.Confidence == "high" {
			highCount++
		}
	}
	return float64(highCount) / float64(len(consensus.DailyConsensus))
}
