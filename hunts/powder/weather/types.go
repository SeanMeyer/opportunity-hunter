package weather

import (
	"fmt"
	"math"
	"strings"
	"time"
)

// Forecast holds parsed daily weather data from a single source fetch.
// Each forecast is tied to a specific resort's coordinates and elevation.
type Forecast struct {
	RegionID  string
	ResortID  string
	FetchedAt time.Time
	Source    string // "open_meteo", "nws"
	Model     string // weather model name (e.g., "gfs_seamless", "ecmwf_ifs025")
	DailyData []DailyForecast
}

// DailyForecast holds the weather metrics for a single calendar day.
// Day = 6am–6pm local; Night combines midnight–6am and 6pm–midnight
// on the same calendar date. It is not the following morning.
type DailyForecast struct {
	SkiSession          *SkiSessionSnow `json:",omitempty"`
	Date                time.Time
	SnowfallCM          float64
	ShelteredSnowfallCM float64
	TemperatureMinC     float64
	TemperatureMaxC     float64
	PrecipitationMM     float64
	FreezingLevelM      float64
	SLRatio             float64
	RainHours           int
	MixedHours          int
	Day                 HalfDay
	Night               HalfDay
}

// HalfDay holds weather metrics for a 12-hour period (day or night).
type HalfDay struct {
	SnowfallCM            float64
	ShelteredSnowfallCM   float64
	TemperatureC          float64
	PrecipitationMM       float64
	WindSpeedKmh          float64
	WindGustKmh           float64
	WindDirectionDeg      *float64 `json:",omitempty"` // Circular mean FROM true north; nil means unknown or variable.
	WindDirectionVariable bool     `json:",omitempty"`
	FreezingLevelMinM     float64
	FreezingLevelMaxM     float64
	CloudCoverPct         float64
}

// SnowfallWindow summarizes accumulated snowfall over a date range.
type SnowfallWindow struct {
	RegionID    string
	StartDate   time.Time
	EndDate     time.Time
	TotalIn     float64
	IsNearRange bool
}

// ModelConsensus aggregates multi-model forecasts for a region.
type ModelConsensus struct {
	RegionID       string
	Models         []string
	DailyConsensus []DayConsensus
}

// DayConsensus holds per-day consensus data across multiple weather models.
type DayConsensus struct {
	Date           time.Time
	SnowfallMinCM  float64
	SnowfallMaxCM  float64
	SnowfallMeanCM float64
	SpreadToMean   float64
	Confidence     string // "high", "moderate", "low"
}

// ForecastDiscussion holds NWS Area Forecast Discussion text.
type ForecastDiscussion struct {
	WFO       string
	IssuedAt  time.Time
	Text      string
	FetchedAt time.Time
}

// DetectionResult holds the outcome of checking a region's forecasts against thresholds.
type DetectionResult struct {
	RegionID string
	Detected bool
	Windows  []SnowfallWindow
}

// FetchResult holds all weather data fetched for a region in a single pass.
type FetchResult struct {
	Forecasts  []Forecast
	Discussion *ForecastDiscussion
}

// ScanSnapshot captures the full weather context at scan time.
// Serialized into RawData on each opportunity so the evaluator can
// reconstruct rich prompt context without re-fetching weather.
type ScanSnapshot struct {
	Forecasts  []Forecast          `json:"forecasts"`
	Discussion *ForecastDiscussion `json:"discussion,omitempty"`
	Consensus  ModelConsensus      `json:"consensus"`
	Resorts    []Resort            `json:"resorts"`
	Detection  DetectionResult     `json:"detection"`
	ScannedAt  time.Time           `json:"scanned_at"`
}

// Region mirrors the catalog region with fields needed by weather clients.
type Region struct {
	ID                  string
	Name                string
	Latitude            float64
	Longitude           float64
	FrictionTier        string
	NearThresholdIn     float64
	ExtendedThresholdIn float64
	Country             string
	Timezone            string
	StormGroup          string
}

// Resort mirrors the catalog resort with fields needed by weather clients.
type Resort struct {
	ID                string
	RegionID          string
	Name              string
	Latitude          float64
	Longitude         float64
	SummitElevationFt int
	BaseElevationFt   int
	VerticalDropFt    int
	SkiableAcres      int
	LiftCount         int
	PassAffiliations  []string
	Metadata          map[string]string
}

// MidMountainElevationM returns the mid-mountain elevation in meters.
func (r Resort) MidMountainElevationM() int {
	midFt := (r.BaseElevationFt + r.SummitElevationFt) / 2
	return int(float64(midFt) * 0.3048)
}

// WeatherChangeSummary is the result of comparing two forecast snapshots.
type WeatherChangeSummary struct {
	Changed                 bool
	TotalSnowfallDeltaIn    float64
	MaxDailySnowfallDeltaIn float64
	MaxTempDeltaC           float64
	DaysMismatched          int
	Reason                  string
}

// Evaluation result types used by the evaluator.

// Tier represents the LLM's quality assessment.
type Tier string

const (
	TierDropEverything Tier = "DROP_EVERYTHING"
	TierRecommended    Tier = "RECOMMENDED"
	TierWatch          Tier = "WATCH"
	TierSkip           Tier = "SKIP"
	TierWorthALook     Tier = "WORTH_A_LOOK"
	TierOnTheRadar     Tier = "ON_THE_RADAR"
)

// ChangeClass categorizes evaluation differences.
type ChangeClass string

const (
	ChangeNew       ChangeClass = "new"
	ChangeMaterial  ChangeClass = "material"
	ChangeMinor     ChangeClass = "minor"
	ChangeDowngrade ChangeClass = "downgrade"
)

// DayEvaluation is the LLM's assessment for a single day.
type DayEvaluation struct {
	Date           time.Time
	Snowfall       string
	Conditions     string
	Recommendation string
}

// KeyFactors captures the LLM's pro/con breakdown.
type KeyFactors struct {
	Pros []string
	Cons []string
}

// ResortInsight captures a notable finding about a resort.
type ResortInsight struct {
	Resort  string `json:"resort"`
	Insight string `json:"insight"`
}

// LogisticsSummary holds the LLM's narrative on trip logistics.
type LogisticsSummary struct {
	Lodging            string
	Transportation     string
	RoadConditions     string
	FlightCost         string
	CarRental          string
	LodgingCost        string
	TotalEstimatedCost string
}

// SnowQuality holds per-day ride quality signals.
type SnowQuality struct {
	CrystalQuality    string
	WindDuringSnowMph float64
	DensityCategory   string
	AvgDensityKgM3    float64
	Bluebird          bool
	CloudCoverPct     float64
	BaseRisk          string
	BaseRiskReason    string
	RideQualityNotes  []string
}

// CMToInches converts centimeters to inches.
func CMToInches(cm float64) float64 { return cm / 2.54 }

// CToF converts Celsius to Fahrenheit.
func CToF(c float64) float64 { return c*9.0/5.0 + 32.0 }

// AFDCoversSnowDays checks whether any day with significant snowfall (>=2")
// falls within the AFD's ~7-day coverage from issuance.
func AFDCoversSnowDays(d *ForecastDiscussion, forecasts []Forecast) bool {
	const afdHorizonDays = 7
	afdCoverage := d.IssuedAt.AddDate(0, 0, afdHorizonDays)
	for _, f := range forecasts {
		for _, day := range f.DailyData {
			if CMToInches(day.SnowfallCM) >= 2.0 && !day.Date.After(afdCoverage) {
				return true
			}
		}
	}
	return false
}

// NormalizeTier keeps saved evaluations from earlier versions usable.
func NormalizeTier(t Tier) Tier {
	switch t {
	case TierWorthALook:
		return TierRecommended
	case TierOnTheRadar:
		return TierWatch
	}
	return t
}

func TierScore(t Tier) float64 {
	switch NormalizeTier(t) {
	case TierDropEverything:
		return .95
	case TierRecommended:
		return .75
	case TierWatch:
		return .5
	default:
		return 0
	}
}

// TierRank maps tiers to an ordinal for comparison.
func TierRank(t Tier) int {
	switch NormalizeTier(t) {
	case TierDropEverything:
		return 3
	case TierRecommended:
		return 2
	case TierWatch:
		return 1
	default:
		return 0
	}
}

// CooldownFor returns the minimum time between evaluations for a given tier.
func CooldownFor(tier Tier) time.Duration {
	switch NormalizeTier(tier) {
	case TierDropEverything:
		return 0
	case TierRecommended:
		return 12 * time.Hour
	case TierWatch:
		return 24 * time.Hour
	default:
		return 24 * time.Hour
	}
}

// ComputeConsensus takes multiple Forecast values (same region, different models),
// aligns by date, and computes spread/mean/confidence per day.
func ComputeConsensus(forecasts []Forecast) ModelConsensus {
	if len(forecasts) == 0 {
		return ModelConsensus{}
	}

	regionID := forecasts[0].RegionID
	models := deduplicateGFSModels(forecasts)

	type dateSnow struct {
		values []float64
		models []string
	}
	byDate := make(map[string]*dateSnow)
	var dateOrder []string

	for _, f := range forecasts {
		for _, d := range f.DailyData {
			key := d.Date.Format("2006-01-02")
			ds, ok := byDate[key]
			if !ok {
				ds = &dateSnow{}
				byDate[key] = ds
				dateOrder = append(dateOrder, key)
			}
			ds.values = append(ds.values, d.SnowfallCM)
			ds.models = append(ds.models, f.Model)
		}
	}

	daily := make([]DayConsensus, 0, len(dateOrder))
	for _, key := range dateOrder {
		ds := byDate[key]
		t, _ := time.Parse("2006-01-02", key)

		if len(ds.values) == 0 {
			continue
		}

		dedupedValues := deduplicateGFS(ds.values, ds.models)

		minVal := dedupedValues[0]
		maxVal := dedupedValues[0]
		sum := 0.0
		for _, v := range dedupedValues {
			sum += v
			if v < minVal {
				minVal = v
			}
			if v > maxVal {
				maxVal = v
			}
		}
		mean := sum / float64(len(dedupedValues))

		var spreadToMean float64
		var confidence string
		if mean == 0 {
			spreadToMean = 0
			confidence = "high"
		} else {
			spreadToMean = (maxVal - minVal) / mean
			switch {
			case spreadToMean < 0.5:
				confidence = "high"
			case spreadToMean <= 1.0:
				confidence = "moderate"
			default:
				confidence = "low"
			}
		}

		daily = append(daily, DayConsensus{
			Date:           t,
			SnowfallMinCM:  minVal,
			SnowfallMaxCM:  maxVal,
			SnowfallMeanCM: mean,
			SpreadToMean:   spreadToMean,
			Confidence:     confidence,
		})
	}

	return ModelConsensus{
		RegionID:       regionID,
		Models:         models,
		DailyConsensus: daily,
	}
}

func deduplicateGFS(values []float64, models []string) []float64 {
	if len(values) != len(models) {
		return values
	}
	seen := make(map[string]bool)
	var result []float64
	for i, v := range values {
		model := models[i]
		if strings.HasPrefix(model, "gfs_") {
			key := fmt.Sprintf("gfs:%.1f", v)
			if seen[key] {
				continue
			}
			seen[key] = true
		}
		result = append(result, v)
	}
	return result
}

func deduplicateGFSModels(forecasts []Forecast) []string {
	seen := make(map[string]bool)
	var gfsSeen bool
	var models []string
	for _, f := range forecasts {
		if f.Model == "" || seen[f.Model] {
			continue
		}
		seen[f.Model] = true
		if strings.HasPrefix(f.Model, "gfs_") {
			if gfsSeen {
				continue
			}
			gfsSeen = true
		}
		models = append(models, f.Model)
	}
	return models
}

// HaversineDistanceKM returns the great-circle distance in km between two lat/lon points.
func HaversineDistanceKM(lat1, lon1, lat2, lon2 float64) float64 {
	const earthRadiusKM = 6371.0
	dLat := (lat2 - lat1) * math.Pi / 180
	dLon := (lon2 - lon1) * math.Pi / 180
	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(lat1*math.Pi/180)*math.Cos(lat2*math.Pi/180)*
			math.Sin(dLon/2)*math.Sin(dLon/2)
	return earthRadiusKM * 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
}

// FrictionTierFromDistance assigns a friction tier based on straight-line distance.
func FrictionTierFromDistance(distKM float64) string {
	estimatedDriveHours := (distKM * 1.8) / 100.0
	switch {
	case estimatedDriveHours <= 3:
		return "local_drive"
	case estimatedDriveHours <= 8:
		return "regional_drive"
	case estimatedDriveHours <= 14:
		return "high_friction_drive"
	default:
		return "flight"
	}
}

// FrictionThresholds returns near and extended snowfall thresholds for a friction tier.
func FrictionThresholds(tier string) (nearIn, extendedIn float64) {
	switch tier {
	case "local_drive":
		return 6, 12
	case "regional_drive":
		return 14, 20
	case "high_friction_drive":
		return 18, 24
	case "flight":
		return 24, 36
	default:
		return 14, 20
	}
}
