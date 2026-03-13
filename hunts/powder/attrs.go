package powder

import (
	"encoding/json"

	"github.com/seanmeyer/opportunity-hunter/core"
	"github.com/seanmeyer/opportunity-hunter/hunts/powder/weather"
)

// PowderAttrs holds powder-specific attributes stored on each opportunity.
type PowderAttrs struct {
	SnowfallIn    float64 `json:"snowfall_in"`
	Consensus     float64 `json:"consensus"`
	FrictionTier  string  `json:"friction_tier"`
	ChangeClass   string  `json:"change_class"`
	WeatherWindow string  `json:"weather_window"`
	StormGroup    string  `json:"storm_group"`
}

func (a PowderAttrs) Encode() core.Attributes {
	b, _ := json.Marshal(a)
	return b
}

func DecodePowderAttrs(raw core.Attributes) (PowderAttrs, error) {
	var a PowderAttrs
	return a, json.Unmarshal(raw, &a)
}

// Re-export tier constants from weather package for backward compatibility with tests.
type Tier = weather.Tier

const (
	TierDropEverything = weather.TierDropEverything
	TierWorthALook     = weather.TierWorthALook
	TierOnTheRadar     = weather.TierOnTheRadar
)

// FrictionTier represents geographic distance from home.
type FrictionTier string

const (
	FrictionLocal    FrictionTier = "local_drive"
	FrictionRegional FrictionTier = "regional_drive"
	FrictionHigh     FrictionTier = "high_friction_drive"
	FrictionFlight   FrictionTier = "flight"
)

// Thresholds returns near and extended snowfall thresholds in inches for a friction tier.
func (ft FrictionTier) Thresholds() (nearIn, extendedIn float64) {
	return weather.FrictionThresholds(string(ft))
}
