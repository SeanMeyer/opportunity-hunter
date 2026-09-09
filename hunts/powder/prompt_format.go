package powder

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/seanmeyer/opportunity-hunter/core"
	"github.com/seanmeyer/opportunity-hunter/hunts/powder/weather"
)

// FormatConsolidatedWeatherForPrompt builds per-resort daily weather tables.
func FormatConsolidatedWeatherForPrompt(forecasts []weather.Forecast, resorts []weather.Resort) string {
	if len(forecasts) == 0 {
		return "No detailed forecast data available."
	}

	// Group forecasts by resort.
	byResort := make(map[string][]weather.Forecast)
	for _, f := range forecasts {
		byResort[f.ResortID] = append(byResort[f.ResortID], f)
	}

	resortNames := make(map[string]string)
	for _, r := range resorts {
		resortNames[r.ID] = r.Name
	}

	var b strings.Builder
	b.WriteString("Opening snow = previous day 16:00 through this day 09:00; During skiing = 09:00–16:00, assumed resort-local hours, not verified operating times. Unknown means hourly coverage is incomplete or the saved forecast predates ski-session totals. These are snowfall amounts, not guaranteed untouched depth. Wind bearings are FROM true north; circular means of available hourly directions. Variable means directions disagree substantially; Unknown means unavailable. Day = local 06:00–18:00; Night = 00:00–06:00 and 18:00–24:00 on the row's date (not the following morning). Gridpoint winds do not establish lift access or terrain shelter.\n")
	for resortID, rForecasts := range byResort {
		name := resortNames[resortID]
		if name == "" {
			name = resortID
		}
		b.WriteString(fmt.Sprintf("### %s\n", name))

		// Use preferred model forecast (first one with data).
		best := pickBestForecast(rForecasts)
		if best == nil {
			b.WriteString("No data.\n\n")
			continue
		}

		b.WriteString(fmt.Sprintf("Forecast: %s / %s; fetched %s.\n", best.Source, best.Model, best.FetchedAt.Format(time.RFC3339)))
		b.WriteString("| Date | Opening snow | During skiing | Temp (F) | Day gust (mph) | Day direction | Night gust (mph) | Night direction | SLR | Freeze Lvl (ft) | Confidence |\n")
		b.WriteString("|------|----------|------------|----------|------------|---------------|------------------|-----------------|-----|-----------------|------------|\n")

		for _, d := range best.DailyData {
			openingSnow, skiingSnow := "Unknown", "Unknown"
			if d.SkiSession != nil {
				openingSnow = formatSessionSnow(d.SkiSession.OpeningSnowCM)
				skiingSnow = formatSessionSnow(d.SkiSession.DuringSkiingSnowCM)
			}
			tempLow := weather.CToF(d.TemperatureMinC)
			tempHigh := weather.CToF(d.TemperatureMaxC)
			windMax := d.Day.WindGustKmh * 0.621371
			slr := d.SLRatio
			freezeLvl := d.FreezingLevelM * 3.28084

			conf := "—"
			hasSessionSnow := d.SkiSession != nil && ((d.SkiSession.OpeningSnowCM != nil && *d.SkiSession.OpeningSnowCM > 0) ||
				(d.SkiSession.DuringSkiingSnowCM != nil && *d.SkiSession.DuringSkiingSnowCM > 0))
			if d.Day.SnowfallCM > 0 || d.Night.SnowfallCM > 0 || hasSessionSnow {
				conf = "see consensus"
			}

			b.WriteString(fmt.Sprintf("| %s | %s | %s | %.0f/%.0f | %.0f | %s | %.0f | %s | %.0f:1 | %.0f | %s |\n",
				d.Date.Format("Mon Jan 2"),
				openingSnow, skiingSnow,
				tempLow, tempHigh,
				windMax, formatWindDirection(d.Day), d.Night.WindGustKmh*0.621371, formatWindDirection(d.Night), slr, freezeLvl, conf))
		}
		b.WriteString("\n")
	}

	return b.String()
}

func pickBestForecast(forecasts []weather.Forecast) *weather.Forecast {
	if len(forecasts) == 0 {
		return nil
	}
	// Prefer open_meteo forecasts with most daily data.
	var best *weather.Forecast
	for i := range forecasts {
		f := &forecasts[i]
		if best == nil || len(f.DailyData) > len(best.DailyData) {
			best = f
		}
	}
	return best
}

// FormatResortsForPrompt renders resort details for the LLM prompt.
func FormatResortsForPrompt(resorts []weather.Resort) string {
	if len(resorts) == 0 {
		return "No resort details available."
	}

	var b strings.Builder
	for _, r := range resorts {
		b.WriteString(fmt.Sprintf("**%s**\n", r.Name))
		b.WriteString(fmt.Sprintf("- Summit: %d ft | Base: %d ft | Vertical: %d ft\n",
			r.SummitElevationFt, r.BaseElevationFt, r.VerticalDropFt))
		b.WriteString(fmt.Sprintf("- Skiable acres: %d | Lifts: %d\n", r.SkiableAcres, r.LiftCount))
		if len(r.PassAffiliations) > 0 {
			b.WriteString(fmt.Sprintf("- Passes: %s\n", strings.Join(r.PassAffiliations, ", ")))
		}
		if notes, ok := r.Metadata["notes"]; ok && notes != "" {
			b.WriteString(fmt.Sprintf("- Notes: %s\n", notes))
		}
		b.WriteString("\n")
	}
	return b.String()
}

// FormatRideQualityForPrompt renders snow quality analysis for the prompt.
func FormatRideQualityForPrompt(qualities []weather.SnowQuality, days []weather.DailyForecast) string {
	if len(qualities) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString("### Ride Quality Analysis\n\n")

	for i, q := range qualities {
		if i >= len(days) {
			break
		}
		d := days[i]
		snowIn := weather.CMToInches(d.SnowfallCM)
		if snowIn < 0.5 {
			continue
		}

		b.WriteString(fmt.Sprintf("**%s** (%.1f\")\n", d.Date.Format("Mon Jan 2"), snowIn))
		if q.DensityCategory != "" {
			b.WriteString(fmt.Sprintf("- Density: %s (%.0f kg/m³)\n", q.DensityCategory, q.AvgDensityKgM3))
		}
		if q.CrystalQuality != "" {
			b.WriteString(fmt.Sprintf("- Crystal quality: %s (avg wind %.0f mph during snow)\n", q.CrystalQuality, q.WindDuringSnowMph))
		}
		if q.BaseRisk != "low" && q.BaseRiskReason != "" {
			b.WriteString(fmt.Sprintf("- Base risk: %s — %s\n", q.BaseRisk, q.BaseRiskReason))
		}
		if q.Bluebird {
			b.WriteString("- Bluebird conditions expected\n")
		}
		for _, note := range q.RideQualityNotes {
			b.WriteString(fmt.Sprintf("- %s\n", note))
		}
		b.WriteString("\n")
	}

	return b.String()
}

// FormatRainLineRisk checks freezing level vs base elevation.
func FormatRainLineRisk(forecasts []weather.Forecast, resorts []weather.Resort) string {
	if len(forecasts) == 0 || len(resorts) == 0 {
		return ""
	}

	// Find the lowest base elevation.
	minBase := resorts[0].BaseElevationFt
	for _, r := range resorts[1:] {
		if r.BaseElevationFt < minBase {
			minBase = r.BaseElevationFt
		}
	}

	var warnings []string
	for _, f := range forecasts {
		for _, d := range f.DailyData {
			freezeLvlFt := d.FreezingLevelM * 3.28084
			if freezeLvlFt > float64(minBase) && d.PrecipitationMM > 2 {
				warnings = append(warnings, fmt.Sprintf("- %s: Freezing level at %.0f ft (base at %d ft) — rain at base possible",
					d.Date.Format("Mon Jan 2"), freezeLvlFt, minBase))
			}
		}
		break // Only check first forecast to avoid duplication.
	}

	if len(warnings) == 0 {
		return ""
	}
	return "### Rain Line Risk\n" + strings.Join(warnings, "\n") + "\n"
}

// FormatResortConsensusForPrompt renders multi-model spread per day.
func FormatResortConsensusForPrompt(consensus weather.ModelConsensus) string {
	if len(consensus.DailyConsensus) == 0 {
		return "No multi-model consensus data available."
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("Models: %s\n\n", strings.Join(consensus.Models, ", ")))
	b.WriteString("| Date | Min | Mean | Max | Spread | Confidence |\n")
	b.WriteString("|------|-----|------|-----|--------|------------|\n")

	for _, dc := range consensus.DailyConsensus {
		b.WriteString(fmt.Sprintf("| %s | %.1f\" | %.1f\" | %.1f\" | %.0f%% | %s |\n",
			dc.Date.Format("Mon Jan 2"),
			weather.CMToInches(dc.SnowfallMinCM),
			weather.CMToInches(dc.SnowfallMeanCM),
			weather.CMToInches(dc.SnowfallMaxCM),
			dc.SpreadToMean*100,
			dc.Confidence))
	}

	return b.String()
}

// FormatDiscussionForPrompt conditionally includes NWS AFD text.
func FormatDiscussionForPrompt(discussion *weather.ForecastDiscussion, forecasts []weather.Forecast) string {
	if discussion == nil || discussion.Text == "" {
		return "No NWS forecast discussion available."
	}

	if !weather.AFDCoversSnowDays(discussion, forecasts) {
		return "NWS forecast discussion available but does not cover significant snow days."
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("**NWS %s** (issued %s)\n\n",
		discussion.WFO, discussion.IssuedAt.Format("Mon Jan 2 15:04 MST")))
	b.WriteString(discussion.Text)
	return b.String()
}

// FormatProfileForPrompt renders the user profile for the LLM.
func FormatProfileForPrompt(profile *UserProfile) string {
	if profile == nil {
		return "No specific profile provided."
	}

	var b strings.Builder

	if profile.HomeBase != "" {
		b.WriteString(fmt.Sprintf("- Home: %s\n", profile.HomeBase))
	}
	if len(profile.Passes) > 0 {
		b.WriteString(fmt.Sprintf("- Passes: %s\n", strings.Join(profile.Passes, ", ")))
	}
	if profile.SkillLevel != "" {
		b.WriteString(fmt.Sprintf("- Skill: %s\n", profile.SkillLevel))
	}
	if profile.RemoteWork {
		b.WriteString("- Remote work: yes (flexible weekdays)\n")
	}
	if profile.PTODays > 0 {
		b.WriteString(fmt.Sprintf("- PTO remaining: %d days\n", profile.PTODays))
	}
	if len(profile.BlackoutDates) > 0 {
		dates := make([]string, len(profile.BlackoutDates))
		for i, d := range profile.BlackoutDates {
			dates[i] = d.Format("Jan 2")
		}
		b.WriteString(fmt.Sprintf("- Blackout dates: %s\n", strings.Join(dates, ", ")))
	}
	if profile.Preferences != "" {
		b.WriteString(fmt.Sprintf("- Preferences: %s\n", profile.Preferences))
	}

	if b.Len() == 0 {
		return "No specific profile provided."
	}
	return b.String()
}

// UserProfile holds structured subscriber profile data.
// This is defined here (in powder package) for use by prompt formatting,
// and mirrors the core.UserProfile but with powder-specific rendering.
type UserProfile struct {
	HomeBase      string
	HomeLat       float64
	HomeLon       float64
	Passes        []string
	SkillLevel    string
	Preferences   string
	RemoteWork    bool
	PTODays       int
	BlackoutDates []time.Time
}

// UserProfileFromCore converts a core.UserProfile to the powder-specific format.
func UserProfileFromCore(cp *core.UserProfile) *UserProfile {
	if cp == nil {
		return nil
	}
	return &UserProfile{
		HomeBase:      cp.HomeBase,
		HomeLat:       cp.HomeLat,
		HomeLon:       cp.HomeLon,
		Passes:        cp.Passes,
		SkillLevel:    cp.SkillLevel,
		Preferences:   cp.Preferences,
		RemoteWork:    cp.RemoteWork,
		PTODays:       cp.PTODays,
		BlackoutDates: cp.BlackoutDates,
	}
}

func formatWindDirection(h weather.HalfDay) string {
	if h.WindDirectionVariable {
		return "Variable"
	}
	if h.WindDirectionDeg == nil || math.IsNaN(*h.WindDirectionDeg) || math.IsInf(*h.WindDirectionDeg, 0) || *h.WindDirectionDeg < 0 || *h.WindDirectionDeg > 360 {
		return "Unknown"
	}
	degrees := math.Mod(*h.WindDirectionDeg, 360)
	names := []string{"N", "NNE", "NE", "ENE", "E", "ESE", "SE", "SSE", "S", "SSW", "SW", "WSW", "W", "WNW", "NW", "NNW"}
	return fmt.Sprintf("%s (%.0f°)", names[int(math.Round(degrees/22.5))%16], math.Mod(math.Round(degrees), 360))
}

func formatSessionSnow(cm *float64) string {
	if cm == nil {
		return "Unknown"
	}
	return fmt.Sprintf("%.1f\"", weather.CMToInches(*cm))
}
