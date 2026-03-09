package powder

import (
	"time"

	"github.com/seanmeyer/opportunity-hunter/core"
)

// Weather change thresholds for re-evaluation.
const (
	DailySnowfallThresholdIn = 2.0
	TotalSnowfallThresholdIn = 3.0
)

// Tier cooldowns — how long to wait before re-evaluating.
var tierCooldowns = map[Tier]time.Duration{
	TierDropEverything: 0,
	TierWorthALook:     12 * time.Hour,
	TierOnTheRadar:     24 * time.Hour,
}

// ShouldReEvaluate checks if an opportunity should be re-evaluated based on
// weather changes and cooldown timers.
func (h *PowderHunt) ShouldReEvaluate(opp core.Opportunity, lastEval *core.Evaluation) bool {
	if lastEval == nil {
		return false
	}

	attrs, err := DecodePowderAttrs(opp.Attributes)
	if err != nil {
		return false
	}

	// Check cooldown by tier.
	tier := Tier(attrs.FrictionTier)
	cooldown, ok := tierCooldowns[tier]
	if !ok {
		cooldown = 24 * time.Hour
	}
	if time.Since(lastEval.EvaluatedAt) < cooldown {
		return false
	}

	// Check budget.
	if h.costTracker != nil {
		schedule := h.DefaultSchedule()
		if schedule.MaxMonthlySpendUSD != nil {
			spent := h.costTracker.ForHunt(h.Name())
			if spent >= *schedule.MaxMonthlySpendUSD {
				return false
			}
		}
	}

	// In production, this would compare current weather data against the
	// last evaluation's weather snapshot. For now, always allow re-eval
	// after cooldown.
	return true
}
