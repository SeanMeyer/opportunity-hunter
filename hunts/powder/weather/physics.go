package weather

import (
	"fmt"
	"math"
	"time"
)

// WindShelterFactor reduces wind speed for sheltered terrain (trees, gullies).
const WindShelterFactor = 0.5

// Rain/snow temperature thresholds in Celsius.
const slrThresholdRainC = 1.6667 // 35°F — above this is rain

const (
	densityFloor   = 40.0  // kg/m3 — coldest realistic snow (25:1 SLR)
	densityCeiling = 250.0 // kg/m3 — heaviest realistic wet snow (4:1 SLR)
)

// CalculateDensity returns fresh snow density in kg/m3 using the Vionnet et al. (2012)
// formula: density = 109 + 6*T + 26*sqrt(u), clamped to [40, 250] kg/m3.
// Returns 0 for rain (temp above 1.67°C / 35°F).
func CalculateDensity(tempC float64, windSpeedMs float64) float64 {
	if tempC > slrThresholdRainC {
		return 0
	}
	density := 109.0 + 6.0*tempC + 26.0*math.Sqrt(windSpeedMs)
	if density < densityFloor {
		return densityFloor
	}
	if density > densityCeiling {
		return densityCeiling
	}
	return density
}

// SLRFromDensity converts snow density (kg/m3) to snow-to-liquid ratio.
func SLRFromDensity(density float64) float64 {
	if density <= 0 {
		return 0
	}
	return 1000.0 / density
}

// SnowfallFromPrecip returns snowfall in cm for a given hour's precipitation (mm),
// temperature (°C), and wind speed (m/s).
func SnowfallFromPrecip(precipMM float64, tempC float64, windSpeedMs float64) float64 {
	if precipMM <= 0 {
		return 0
	}
	density := CalculateDensity(tempC, windSpeedMs)
	if density <= 0 {
		return 0
	}
	slr := SLRFromDensity(density)
	return precipMM / 10.0 * slr
}

// IsRain returns true if the temperature is above the rain threshold.
func IsRain(tempC float64) bool {
	return tempC > slrThresholdRainC
}

// IsMixedPrecip returns true if the temperature is in the mixed precipitation zone (32-35°F).
func IsMixedPrecip(tempC float64) bool {
	return tempC >= 0 && tempC <= slrThresholdRainC
}

// DensityCategoryName returns a human-readable density classification from kg/m3.
func DensityCategoryName(densityKgM3 float64) string {
	switch {
	case densityKgM3 <= 0:
		return ""
	case densityKgM3 < 60:
		return "cold_smoke"
	case densityKgM3 < 90:
		return "dry_powder"
	case densityKgM3 < 130:
		return "standard"
	case densityKgM3 < 180:
		return "heavy"
	default:
		return "wet_cement"
	}
}

// CrystalQualityName classifies crystal integrity based on average wind during snowfall (mph).
func CrystalQualityName(avgWindMph float64) string {
	switch {
	case avgWindMph < 15:
		return "intact"
	case avgWindMph < 25:
		return "partially_broken"
	default:
		return "wind_broken"
	}
}

// SolarElevationAtNoon returns the sun's elevation angle in degrees at solar noon.
func SolarElevationAtNoon(latitudeDeg float64, date time.Time) float64 {
	doy := float64(date.YearDay())
	declination := 23.45 * math.Sin(2*math.Pi*(284+doy)/365.0)
	return 90.0 - math.Abs(latitudeDeg-declination)
}

// AssessBaseRisk evaluates the risk of a hard layer under new snow.
func AssessBaseRisk(preStormDays []DailyForecast, latitude float64, stormStartDate time.Time) (risk string, reason string) {
	if len(preStormDays) == 0 {
		return "low", ""
	}

	var warmHours int
	var solarRisk bool
	solarElevation := SolarElevationAtNoon(latitude, stormStartDate)

	for _, d := range preStormDays {
		if d.TemperatureMaxC > 0 {
			if d.TemperatureMaxC > 3 {
				warmHours += 6
			} else {
				warmHours += 2
			}
		}
		tempThresholdC := -3.0
		if solarElevation > 55 {
			tempThresholdC = -8.0
		} else if solarElevation > 45 {
			tempThresholdC = -5.0
		} else if solarElevation < 35 {
			continue
		}
		if d.Day.CloudCoverPct < 30 && d.Day.TemperatureC > tempThresholdC {
			solarRisk = true
		}
	}

	switch {
	case warmHours >= 6:
		return "high", fmt.Sprintf("above freezing for ~%d hours before storm — melt-freeze crust likely", warmHours)
	case warmHours >= 1 && solarRisk:
		return "high", "melt-freeze and sun crust both likely"
	case solarRisk:
		return "moderate", fmt.Sprintf("clear skies with solar elevation %.0f° — sun crust possible on south-facing terrain", solarElevation)
	case warmHours >= 1:
		return "moderate", "brief above-freezing period — possible melt-freeze layer"
	default:
		return "low", ""
	}
}

// AssessRideQuality computes snow quality signals for each day in the forecast.
func AssessRideQuality(days []DailyForecast, preStormDays []DailyForecast, latitude float64, stormStartDate time.Time) []SnowQuality {
	if len(days) == 0 {
		return nil
	}

	baseRisk, baseRiskReason := AssessBaseRisk(preStormDays, latitude, stormStartDate)

	densityForDay := func(d DailyForecast) float64 {
		if d.SLRatio <= 0 {
			return 0
		}
		return 1000.0 / d.SLRatio
	}

	avgWindDuringSnow := func(d DailyForecast) float64 {
		var totalWind, totalPrecip float64
		if d.Day.PrecipitationMM > 0 {
			totalWind += d.Day.WindSpeedKmh * d.Day.PrecipitationMM
			totalPrecip += d.Day.PrecipitationMM
		}
		if d.Night.PrecipitationMM > 0 {
			totalWind += d.Night.WindSpeedKmh * d.Night.PrecipitationMM
			totalPrecip += d.Night.PrecipitationMM
		}
		if totalPrecip <= 0 {
			return 0
		}
		return totalWind / totalPrecip * 0.621371
	}

	qualities := make([]SnowQuality, len(days))

	for i, d := range days {
		snowIn := CMToInches(d.SnowfallCM)
		density := densityForDay(d)
		densCat := DensityCategoryName(density)
		windMph := avgWindDuringSnow(d)
		crystalQ := CrystalQualityName(windMph)

		q := SnowQuality{
			DensityCategory:   densCat,
			AvgDensityKgM3:    density,
			CrystalQuality:    crystalQ,
			WindDuringSnowMph: windMph,
			CloudCoverPct:     d.Day.CloudCoverPct,
			BaseRisk:          baseRisk,
			BaseRiskReason:    baseRiskReason,
		}

		isSnowDay := snowIn >= 0.5
		var notes []string

		if isSnowDay {
			switch crystalQ {
			case "intact":
				notes = append(notes, "Fresh dendrites likely — expect true powder feel")
			case "partially_broken":
				notes = append(notes, "Moderate wind during snowfall — crystals partially broken, still good but not the lightest")
			case "wind_broken":
				notes = append(notes, "Heavy wind during snowfall — snow will feel chalky on exposed terrain, best quality in protected trees")
			}
		}

		if isSnowDay && i > 0 {
			prevDensity := densityForDay(days[i-1])
			prevCat := DensityCategoryName(prevDensity)
			prevSnowIn := CMToInches(days[i-1].SnowfallCM)

			if prevSnowIn >= 0.5 && prevCat != "" && densCat != "" && prevCat != densCat {
				prevIsLight := prevCat == "cold_smoke" || prevCat == "dry_powder"
				curIsLight := densCat == "cold_smoke" || densCat == "dry_powder"

				if !prevIsLight && curIsLight {
					notes = append(notes, fmt.Sprintf("Favorable layering — light snow over supportive dense base from %s",
						days[i-1].Date.Format("Jan 02")))
				} else if prevIsLight && !curIsLight {
					notes = append(notes, "New heavy snow over lighter layer — may feel punchy and inconsistent")
				}
			}
		}

		if isSnowDay && baseRisk != "low" {
			isFirstSnowDay := true
			for j := 0; j < i; j++ {
				if CMToInches(days[j].SnowfallCM) >= 0.5 {
					isFirstSnowDay = false
					break
				}
			}

			if isFirstSnowDay {
				notes = append(notes, baseRiskReason)

				if baseRisk == "high" {
					switch {
					case densCat == "cold_smoke" && snowIn < 12:
						notes = append(notes, "Likely punching through to hard layer underneath")
					case densCat == "cold_smoke" && snowIn >= 12:
						notes = append(notes, "Deep enough to float above the crust, but may hit it in thin spots")
					case densCat == "dry_powder" && snowIn < 8:
						notes = append(notes, "May punch through to crust in spots")
					case densCat == "dry_powder" && snowIn >= 8:
						notes = append(notes, "Should have enough depth over the crust")
					case snowIn >= 4:
						notes = append(notes, "Dense enough to ride without hitting the crust")
					}
				} else if baseRisk == "moderate" && densCat == "cold_smoke" && snowIn < 8 {
					notes = append(notes, "Thin — could feel inconsistent over variable base")
				}
			}
		}

		if i > 0 {
			prevSnowIn := CMToInches(days[i-1].SnowfallCM)
			prevNightSnowIn := CMToInches(days[i-1].Night.SnowfallCM)
			if d.Day.CloudCoverPct < 20 && (prevNightSnowIn >= 4 || prevSnowIn >= 4) {
				q.Bluebird = true
				notes = append(notes, "Bluebird powder day — clear skies with fresh snow")
			}
		}

		q.RideQualityNotes = notes
		qualities[i] = q
	}

	return qualities
}
