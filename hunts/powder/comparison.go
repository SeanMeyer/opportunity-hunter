package powder

import (
	"github.com/seanmeyer/opportunity-hunter/hunts/powder/weather"
)

// Compare classifies the change between a prior evaluation and a new one.
// Returns a ChangeClass: new, material, minor, or downgrade.
func Compare(priorTier, newTier weather.Tier, priorSnowfall, newSnowfall float64) weather.ChangeClass {
	priorRank := weather.TierRank(priorTier)
	newRank := weather.TierRank(newTier)

	// Tier downgrade.
	if newRank < priorRank {
		return weather.ChangeDowngrade
	}

	// Tier upgrade is always material.
	if newRank > priorRank {
		return weather.ChangeMaterial
	}

	// Same tier — check snowfall delta.
	delta := newSnowfall - priorSnowfall
	if delta > 4 || delta < -4 {
		return weather.ChangeMaterial
	}

	return weather.ChangeMinor
}
