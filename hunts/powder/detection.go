package powder

import (
	"encoding/json"
	"log/slog"
	"time"

	"github.com/seanmeyer/opportunity-hunter/core"
	"github.com/seanmeyer/opportunity-hunter/hunts/powder/weather"
)

// ShouldReEvaluate checks if an opportunity should be re-evaluated based on
// weather changes, cooldown timers, and budget constraints.
// Gating order: first eval → budget → cooldown → weather change.
func (h *PowderHunt) ShouldReEvaluate(opp core.Opportunity, lastEval *core.Evaluation) bool {
	if lastEval == nil {
		return false
	}

	attrs, err := DecodePowderAttrs(opp.Attributes)
	if err != nil {
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

	// Check cooldown by tier.
	tier := tierFromAttrs(attrs, lastEval)
	cooldown := weather.CooldownFor(tier)
	if cooldown > 0 && time.Since(lastEval.EvaluatedAt) < cooldown {
		return false
	}

	// Check if weather has actually changed since last evaluation.
	changed := h.forecastsChangedSinceEval(opp)
	if !changed {
		slog.Debug("weather unchanged, skipping re-eval", "region", opp.Title)
		return false
	}

	return true
}

// forecastsChangedSinceEval compares the previous forecast snapshot (from RawData)
// with the current cached forecasts for the opportunity's region.
func (h *PowderHunt) forecastsChangedSinceEval(opp core.Opportunity) bool {
	if opp.RawData == "" || opp.RawData == "{}" {
		return true // No previous data, allow re-eval.
	}

	var prevSnapshot weather.ScanSnapshot
	if err := json.Unmarshal([]byte(opp.RawData), &prevSnapshot); err != nil || len(prevSnapshot.Forecasts) == 0 {
		return true
	}

	// Get current forecasts from cache or return true (assume changed).
	currentForecasts := h.getCachedForecasts(opp.Title)
	if len(currentForecasts) == 0 {
		return true
	}

	summary := weather.ForecastsChanged(prevSnapshot.Forecasts, currentForecasts)
	if summary.Changed {
		slog.Info("weather changed for re-eval",
			"region", opp.Title, "reason", summary.Reason,
			"snow_delta_in", summary.TotalSnowfallDeltaIn)
	}
	return summary.Changed
}

// getCachedForecasts returns cached forecasts for a region name.
// Populated during scan phase; empty if no scan happened this run.
func (h *PowderHunt) getCachedForecasts(regionName string) []weather.Forecast {
	h.cacheMu.RLock()
	defer h.cacheMu.RUnlock()
	if h.forecastCache == nil {
		return nil
	}
	return h.forecastCache[regionName]
}

// CacheForecasts stores fetched forecasts for re-eval comparison.
func (h *PowderHunt) CacheForecasts(regionName string, forecasts []weather.Forecast) {
	h.cacheMu.Lock()
	defer h.cacheMu.Unlock()
	if h.forecastCache == nil {
		h.forecastCache = make(map[string][]weather.Forecast)
	}
	h.forecastCache[regionName] = forecasts
}

// tierFromAttrs extracts the tier for cooldown calculation.
func tierFromAttrs(attrs PowderAttrs, lastEval *core.Evaluation) weather.Tier {
	return extractPriorTier(lastEval)
}
