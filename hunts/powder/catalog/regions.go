package catalog

import (
	_ "embed"
	"encoding/json"
	"fmt"

	"github.com/seanmeyer/opportunity-hunter/hunts/powder/weather"
)

//go:embed data/regions.json
var regionsJSON []byte

// RegionWithResorts pairs a region with the ski areas that share its weather.
type RegionWithResorts struct {
	Region  weather.Region
	Resorts []weather.Resort
}

// Region is an alias for weather.Region for backward compatibility.
type Region = weather.Region

// Resort is an alias for weather.Resort for backward compatibility.
type Resort = weather.Resort

// JSON intermediate types matching the seed JSON structure.

type regionsFile struct {
	Regions []regionJSON `json:"regions"`
}

type regionJSON struct {
	ID          string        `json:"id"`
	Name        string        `json:"name"`
	Country     string        `json:"country"`
	Timezone    string        `json:"timezone"`
	MacroRegion string        `json:"macro_region"`
	Coords      coordsJSON    `json:"coords"`
	Logistics   logisticsJSON `json:"logistics"`
	Resorts     []resortJSON  `json:"resorts"`
}

type coordsJSON struct {
	Lat float64 `json:"lat"`
	Lon float64 `json:"lon"`
}

type logisticsJSON struct {
	NearestAirport string `json:"nearest_airport"`
	DriveNotes     string `json:"drive_notes"`
	LodgingNotes   string `json:"lodging_notes"`
}

type resortJSON struct {
	ID       string            `json:"id"`
	Name     string            `json:"name"`
	Coords   coordsJSON        `json:"coords"`
	Stats    resortStatsJSON   `json:"stats"`
	Metadata map[string]string `json:"metadata"`
}

type resortStatsJSON struct {
	SummitElevationFt int      `json:"summit_elevation_ft"`
	BaseElevationFt   int      `json:"base_elevation_ft"`
	VerticalDropFt    int      `json:"vertical_drop_ft"`
	SkiableAcres      int      `json:"skiable_acres"`
	Lifts             int      `json:"lifts"`
	PassAffiliations  []string `json:"pass_affiliations"`
}

// Regions loads the embedded seed data and converts to domain types.
// Panics on parse failure so misconfigured seed data is caught at startup.
func Regions() []RegionWithResorts {
	var f regionsFile
	if err := json.Unmarshal(regionsJSON, &f); err != nil {
		panic(fmt.Sprintf("seed: parse regions.json: %v", err))
	}

	result := make([]RegionWithResorts, 0, len(f.Regions))
	for _, rj := range f.Regions {
		result = append(result, RegionWithResorts{
			Region:  toRegion(rj),
			Resorts: toResorts(rj.ID, rj.Resorts),
		})
	}
	return result
}

// RegionsForUser loads regions and assigns friction tiers based on user's home coordinates.
func RegionsForUser(homeLat, homeLon float64) []RegionWithResorts {
	regions := Regions()
	for i := range regions {
		r := &regions[i].Region
		dist := weather.HaversineDistanceKM(homeLat, homeLon, r.Latitude, r.Longitude)
		r.FrictionTier = weather.FrictionTierFromDistance(dist)
		near, ext := weather.FrictionThresholds(r.FrictionTier)
		r.NearThresholdIn = near
		r.ExtendedThresholdIn = ext
	}
	return regions
}

// DefaultRegions returns a flat list of regions (backward compatible with existing code).
func DefaultRegions() []Region {
	all := Regions()
	regions := make([]Region, 0, len(all))
	for _, rr := range all {
		regions = append(regions, rr.Region)
	}
	return regions
}

// DefaultResorts returns a flat list of all resorts.
func DefaultResorts() []Resort {
	all := Regions()
	var resorts []Resort
	for _, rr := range all {
		resorts = append(resorts, rr.Resorts...)
	}
	return resorts
}

func toRegion(j regionJSON) weather.Region {
	return weather.Region{
		ID:         j.ID,
		Name:       j.Name,
		Country:    j.Country,
		Timezone:   j.Timezone,
		Latitude:   j.Coords.Lat,
		Longitude:  j.Coords.Lon,
		StormGroup: j.MacroRegion,
	}
}

func toResorts(regionID string, js []resortJSON) []weather.Resort {
	resorts := make([]weather.Resort, 0, len(js))
	for _, j := range js {
		resorts = append(resorts, weather.Resort{
			ID:                j.ID,
			RegionID:          regionID,
			Name:              j.Name,
			Latitude:          j.Coords.Lat,
			Longitude:         j.Coords.Lon,
			SummitElevationFt: j.Stats.SummitElevationFt,
			BaseElevationFt:   j.Stats.BaseElevationFt,
			VerticalDropFt:    j.Stats.VerticalDropFt,
			SkiableAcres:      j.Stats.SkiableAcres,
			LiftCount:         j.Stats.Lifts,
			PassAffiliations:  j.Stats.PassAffiliations,
			Metadata:          j.Metadata,
		})
	}
	return resorts
}
