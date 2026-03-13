package weather

import (
	"fmt"
	"math"
	"time"
)

// Thresholds for determining whether a forecast change is material.
const (
	DailySnowfallThresholdIn = 2.0
	TotalSnowfallThresholdIn = 3.0
	TempThresholdC           = 4.4
)

// Detect checks forecasts against the region's friction-tier thresholds.
// Three window types are evaluated:
//   - Near-range (days 1-7): uses near threshold
//   - Extended-range (days 8-16): uses extended threshold
//   - Bridge windows (days 4-10, 7-13): catches storms straddling the boundary
func Detect(region Region, forecasts []Forecast, now time.Time) DetectionResult {
	// Safety: if thresholds are unset (0), nothing should detect.
	if region.NearThresholdIn <= 0 && region.ExtendedThresholdIn <= 0 {
		return DetectionResult{RegionID: region.ID}
	}

	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)

	nearStart := today.AddDate(0, 0, 1)
	nearEnd := today.AddDate(0, 0, 7)
	extStart := today.AddDate(0, 0, 8)
	extEnd := today.AddDate(0, 0, 16)

	daily := preferredDailySnowfall(today, forecasts)

	nearIn := CMToInches(sumDailyCM(daily, nearStart, nearEnd))
	extIn := CMToInches(sumDailyCM(daily, extStart, extEnd))

	var windows []SnowfallWindow
	if nearIn >= region.NearThresholdIn {
		windows = append(windows, SnowfallWindow{
			RegionID:    region.ID,
			StartDate:   nearStart,
			EndDate:     nearEnd,
			TotalIn:     nearIn,
			IsNearRange: true,
		})
	}
	if extIn >= region.ExtendedThresholdIn {
		windows = append(windows, SnowfallWindow{
			RegionID:    region.ID,
			StartDate:   extStart,
			EndDate:     extEnd,
			TotalIn:     extIn,
			IsNearRange: false,
		})
	}

	if len(windows) == 0 {
		hasNearSnow := sumDailyCM(daily, nearStart, nearEnd) > 0
		hasExtSnow := sumDailyCM(daily, extStart, extEnd) > 0

		if hasNearSnow && hasExtSnow {
			type bridge struct{ startDay, endDay int }
			bridges := []bridge{{4, 10}, {7, 13}}
			for _, b := range bridges {
				bStart := today.AddDate(0, 0, b.startDay)
				bEnd := today.AddDate(0, 0, b.endDay)
				bIn := CMToInches(sumDailyCM(daily, bStart, bEnd))
				if bIn >= region.NearThresholdIn {
					windows = append(windows, SnowfallWindow{
						RegionID:    region.ID,
						StartDate:   bStart,
						EndDate:     bEnd,
						TotalIn:     bIn,
						IsNearRange: true,
					})
					break
				}
			}
		}
	}

	return DetectionResult{
		RegionID: region.ID,
		Detected: len(windows) > 0,
		Windows:  windows,
	}
}

func preferredDailySnowfall(today time.Time, forecasts []Forecast) map[string]float64 {
	preferred := make(map[string]float64)
	for _, f := range forecasts {
		for _, d := range f.DailyData {
			key := d.Date.UTC().Format("2006-01-02")
			if d.SnowfallCM > preferred[key] {
				preferred[key] = d.SnowfallCM
			}
		}
	}
	return preferred
}

func sumDailyCM(daily map[string]float64, start, end time.Time) float64 {
	var total float64
	for d := start; !d.After(end); d = d.AddDate(0, 0, 1) {
		total += daily[d.Format("2006-01-02")]
	}
	return total
}

// ForecastsChanged compares previous and current forecast slices on
// detection-critical fields (snowfall, temperature, precipitation).
func ForecastsChanged(previous, current []Forecast) WeatherChangeSummary {
	if len(previous) == 0 {
		return WeatherChangeSummary{Changed: true, Reason: "first evaluation (no previous forecast)"}
	}
	if len(current) == 0 {
		return WeatherChangeSummary{Changed: true, Reason: "current forecast is empty"}
	}

	prevByDate := aggregateDailyByDate(previous)
	currByDate := aggregateDailyByDate(current)

	mismatched := 0
	for dateKey := range prevByDate {
		if _, ok := currByDate[dateKey]; !ok {
			mismatched++
		}
	}
	if mismatched > 0 {
		return WeatherChangeSummary{
			Changed:        true,
			DaysMismatched: mismatched,
			Reason:         fmt.Sprintf("current forecast missing %d day(s) present in previous", mismatched),
		}
	}

	var totalSnowPrev, totalSnowCurr float64
	var maxDailyDelta float64
	var maxTempDelta float64

	for dateKey, prev := range prevByDate {
		curr, ok := currByDate[dateKey]
		if !ok {
			continue
		}

		prevSnowIn := CMToInches(prev.SnowfallCM)
		currSnowIn := CMToInches(curr.SnowfallCM)
		totalSnowPrev += prevSnowIn
		totalSnowCurr += currSnowIn

		dailyDelta := math.Abs(currSnowIn - prevSnowIn)
		if dailyDelta > maxDailyDelta {
			maxDailyDelta = dailyDelta
		}

		minDelta := math.Abs(curr.TemperatureMinC - prev.TemperatureMinC)
		maxDelta := math.Abs(curr.TemperatureMaxC - prev.TemperatureMaxC)
		tempDelta := max(minDelta, maxDelta)
		if tempDelta > maxTempDelta {
			maxTempDelta = tempDelta
		}
	}

	totalDelta := math.Abs(totalSnowCurr - totalSnowPrev)

	summary := WeatherChangeSummary{
		TotalSnowfallDeltaIn:    totalDelta,
		MaxDailySnowfallDeltaIn: maxDailyDelta,
		MaxTempDeltaC:           maxTempDelta,
	}

	if maxDailyDelta >= DailySnowfallThresholdIn {
		summary.Changed = true
		summary.Reason = fmt.Sprintf("daily snowfall delta %.1f\" exceeds %.1f\" threshold", maxDailyDelta, DailySnowfallThresholdIn)
		return summary
	}
	if totalDelta >= TotalSnowfallThresholdIn {
		summary.Changed = true
		summary.Reason = fmt.Sprintf("total snowfall delta %.1f\" exceeds %.1f\" threshold", totalDelta, TotalSnowfallThresholdIn)
		return summary
	}
	if maxTempDelta >= TempThresholdC {
		summary.Changed = true
		summary.Reason = fmt.Sprintf("temperature delta %.1f°C exceeds %.1f°C threshold", maxTempDelta, TempThresholdC)
		return summary
	}

	return summary
}

type dailySummary struct {
	SnowfallCM      float64
	TemperatureMinC float64
	TemperatureMaxC float64
}

func aggregateDailyByDate(forecasts []Forecast) map[string]dailySummary {
	type accumulator struct {
		snowSum    float64
		tempMinSum float64
		tempMaxSum float64
		count      int
	}

	accum := make(map[string]*accumulator)
	for _, f := range forecasts {
		for _, d := range f.DailyData {
			key := d.Date.Format("2006-01-02")
			a, ok := accum[key]
			if !ok {
				a = &accumulator{}
				accum[key] = a
			}
			a.snowSum += d.SnowfallCM
			a.tempMinSum += d.TemperatureMinC
			a.tempMaxSum += d.TemperatureMaxC
			a.count++
		}
	}

	result := make(map[string]dailySummary, len(accum))
	for key, a := range accum {
		result[key] = dailySummary{
			SnowfallCM:      a.snowSum / float64(a.count),
			TemperatureMinC: a.tempMinSum / float64(a.count),
			TemperatureMaxC: a.tempMaxSum / float64(a.count),
		}
	}
	return result
}
