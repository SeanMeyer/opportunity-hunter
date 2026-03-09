package powder

import (
	"encoding/json"

	"github.com/seanmeyer/opportunity-hunter/core"
)

// PowderAttrs holds powder-specific attributes.
type PowderAttrs struct {
	SnowfallIn    float64 `json:"snowfall_in"`
	Consensus     float64 `json:"consensus"` // 0-1 model agreement
	FrictionTier  string  `json:"friction_tier"`
	ChangeClass   string  `json:"change_class"` // new, material, minor, downgrade
	WeatherWindow string  `json:"weather_window"` // "near" or "extended"
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

// Tier represents the excitement/priority level.
type Tier string

const (
	TierDropEverything Tier = "DROP EVERYTHING"
	TierWorthALook     Tier = "WORTH A LOOK"
	TierOnTheRadar     Tier = "ON THE RADAR"
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
	switch ft {
	case FrictionLocal:
		return 6, 12
	case FrictionRegional:
		return 14, 20
	case FrictionHigh:
		return 18, 24
	case FrictionFlight:
		return 24, 36
	default:
		return 14, 20
	}
}
