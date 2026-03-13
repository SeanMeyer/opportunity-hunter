package powder

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/seanmeyer/opportunity-hunter/core"
	"github.com/seanmeyer/opportunity-hunter/hunts/powder/weather"
)

const stormEvalPromptVersion = "v3.5.0"

const stormEvalPromptTemplate = `You are an expert powder skiing advisor evaluating a storm opportunity for a specific subscriber.
Your job is to classify the storm into one of three tiers and provide actionable guidance.

## Tier Definitions

**DROP_EVERYTHING** — A rare, high-conviction alert. Multiple factors align exceptionally well.
This tier should feel *rare* — a handful of times per season across all monitored regions.

**WORTH_A_LOOK** — A genuinely interesting storm that stands out. Most storms, even good ones,
are ON_THE_RADAR. WORTH_A_LOOK should feel selective enough that when it appears, the subscriber
thinks "oh, interesting" and actually reads the details.

**ON_THE_RADAR** — The default tier for most detected storms. A storm being detected means it
crossed a snowfall threshold — that alone doesn't make it interesting.

## Travel Friction Calibration

Every alert asks the subscriber to consider spending money, using PTO, and disrupting their life.
Calibrate your tier based on travel cost:
- **Local drive (< 3 hours):** A solid storm is enough for a day trip, but not DROP_EVERYTHING.
- **Regional drive (3-8 hours):** Needs to be clearly above average to be WORTH_A_LOOK.
- **Flight destination:** Very high bar. Most storms at big mountains are routine — ON_THE_RADAR.

## Detected Storm Signal

{{.StormWindow}}

## Region and Resort Context

**Region:** {{.RegionName}}

**Weather Forecast Data:**
{{.WeatherData}}

**Multi-Model Consensus:**
{{.ModelConsensus}}

**NWS Forecast Discussion:**
{{.ForecastDiscussion}}

**Resort Details:**
{{.Resorts}}

**Subscriber Profile:**
{{.UserProfile}}

## Evaluation History

{{.EvaluationHistory}}

## Subscriber Feedback

{{.SubscriberFeedback}}

## Instructions

For EACH resort listed above, search for it by name to find:
- Its current operating schedule from the resort's own website
- Recent snow reports, current base depth, and conditions updates
- Recent news articles about the resort
- Road conditions and access alerts

Return a JSON object matching the required schema. All fields are required.

Prompt version: {{.PromptVersion}}`

func buildPrompt(ec core.EvalContext) string {
	var weatherData, regionName, resortsStr, userProfile string
	var stormWindow, evalHistory, modelConsensus, forecastDiscussion string
	var rideQuality, rainLineRisk string

	regionName = "Storm Region"
	if len(ec.Opportunities) > 0 {
		regionName = ec.Opportunities[0].Title
	}

	// Try to deserialize rich weather snapshot from RawData.
	var snapshot *weather.ScanSnapshot
	if len(ec.Opportunities) > 0 && ec.Opportunities[0].RawData != "" && ec.Opportunities[0].RawData != "{}" {
		var snap weather.ScanSnapshot
		if err := json.Unmarshal([]byte(ec.Opportunities[0].RawData), &snap); err == nil && len(snap.Forecasts) > 0 {
			snapshot = &snap
		} else if err != nil {
			slog.Debug("failed to unmarshal scan snapshot", "err", err)
		}
	}

	if snapshot != nil {
		// Rich prompt from full weather snapshot.
		weatherData = FormatConsolidatedWeatherForPrompt(snapshot.Forecasts, snapshot.Resorts)
		resortsStr = FormatResortsForPrompt(snapshot.Resorts)
		modelConsensus = FormatResortConsensusForPrompt(snapshot.Consensus)
		forecastDiscussion = FormatDiscussionForPrompt(snapshot.Discussion, snapshot.Forecasts)
		rainLineRisk = FormatRainLineRisk(snapshot.Forecasts, snapshot.Resorts)

		// Compute ride quality from best forecast.
		if best := pickBestForecast(snapshot.Forecasts); best != nil && len(best.DailyData) > 0 {
			qualities := weather.AssessRideQuality(best.DailyData, nil, 0, snapshot.ScannedAt)
			rideQuality = FormatRideQualityForPrompt(qualities, best.DailyData)
		}

		stormWindow = FormatDetectionForPrompt(snapshot.Detection, snapshot.ScannedAt)
	} else {
		// Fallback: minimal prompt from opportunity attributes.
		stormWindow = buildStormWindowFallback(ec.Opportunities)
		weatherData = buildWeatherDataFallback(ec.Opportunities)
		resortsStr = "See weather data above for resort details."
		modelConsensus = "See weather data above."
		forecastDiscussion = "No NWS forecast discussion available."
	}

	// Append ride quality and rain line risk to weather data if present.
	if rideQuality != "" {
		weatherData += "\n" + rideQuality
	}
	if rainLineRisk != "" {
		weatherData += "\n" + rainLineRisk
	}

	// User profile: structured > preferences text > default.
	if ec.Profile != nil {
		userProfile = FormatProfileForPrompt(UserProfileFromCore(ec.Profile))
	} else if ec.Preferences != "" {
		userProfile = ec.Preferences
	} else {
		userProfile = "No specific profile provided."
	}

	if ec.PriorEval != nil {
		evalHistory = fmt.Sprintf("Prior evaluation at %s: %s",
			ec.PriorEval.EvaluatedAt.Format("2006-01-02 15:04"), ec.PriorEval.RawLLMResponse)
	} else {
		evalHistory = "No prior evaluations"
	}

	subscriberFeedback := formatFeedback(ec.Feedback)

	return renderPrompt(stormEvalPromptTemplate, promptData{
		WeatherData:        weatherData,
		RegionName:         regionName,
		Resorts:            resortsStr,
		UserProfile:        userProfile,
		StormWindow:        stormWindow,
		EvaluationHistory:  evalHistory,
		PromptVersion:      stormEvalPromptVersion,
		ModelConsensus:     modelConsensus,
		ForecastDiscussion: forecastDiscussion,
		SubscriberFeedback: subscriberFeedback,
	})
}

// buildStormWindowFallback creates storm window text from opportunity attributes (no snapshot).
func buildStormWindowFallback(opps []core.Opportunity) string {
	var parts []string
	for _, opp := range opps {
		attrs, err := DecodePowderAttrs(opp.Attributes)
		if err == nil && attrs.SnowfallIn > 0 {
			parts = append(parts, fmt.Sprintf("- %s: %.1f\" total forecasted snowfall (%s)",
				opp.Subtitle, attrs.SnowfallIn, attrs.WeatherWindow))
		}
	}
	if len(parts) > 0 {
		return strings.Join(parts, "\n")
	}
	return "No storm window detected."
}

// buildWeatherDataFallback creates weather data text from opportunity attributes (no snapshot).
func buildWeatherDataFallback(opps []core.Opportunity) string {
	var b strings.Builder
	for _, opp := range opps {
		attrs, err := DecodePowderAttrs(opp.Attributes)
		if err == nil {
			fmt.Fprintf(&b, "### %s\n", opp.Title)
			if attrs.SnowfallIn > 0 {
				fmt.Fprintf(&b, "- Expected snowfall: %.0f inches\n", attrs.SnowfallIn)
			}
			if attrs.Consensus > 0 {
				fmt.Fprintf(&b, "- Model consensus: %.0f%%\n", attrs.Consensus*100)
			}
			fmt.Fprintf(&b, "- Friction: %s\n", attrs.FrictionTier)
			fmt.Fprintf(&b, "- Window: %s\n\n", opp.Subtitle)
		}
	}
	return b.String()
}

type promptData struct {
	WeatherData        string
	RegionName         string
	Resorts            string
	UserProfile        string
	StormWindow        string
	EvaluationHistory  string
	PromptVersion      string
	ModelConsensus     string
	ForecastDiscussion string
	SubscriberFeedback string
}

func renderPrompt(template string, data promptData) string {
	r := template
	r = strings.ReplaceAll(r, "{{.WeatherData}}", data.WeatherData)
	r = strings.ReplaceAll(r, "{{.RegionName}}", data.RegionName)
	r = strings.ReplaceAll(r, "{{.Resorts}}", data.Resorts)
	r = strings.ReplaceAll(r, "{{.UserProfile}}", data.UserProfile)
	r = strings.ReplaceAll(r, "{{.StormWindow}}", data.StormWindow)
	r = strings.ReplaceAll(r, "{{.EvaluationHistory}}", data.EvaluationHistory)
	r = strings.ReplaceAll(r, "{{.PromptVersion}}", data.PromptVersion)
	r = strings.ReplaceAll(r, "{{.ModelConsensus}}", data.ModelConsensus)
	r = strings.ReplaceAll(r, "{{.ForecastDiscussion}}", data.ForecastDiscussion)
	r = strings.ReplaceAll(r, "{{.SubscriberFeedback}}", data.SubscriberFeedback)
	return r
}

// formatFeedback renders user feedback entries into prompt context.
func formatFeedback(entries []core.FeedbackEntry) string {
	if len(entries) == 0 {
		return "No subscriber feedback yet."
	}

	var b strings.Builder
	b.WriteString("The subscriber has provided feedback on past evaluations. Use this to calibrate your tier assignments and recommendations.\n\n")

	for _, fb := range entries {
		emoji := "\xf0\x9f\x91\x8d" // 👍
		if fb.Rating == "down" {
			emoji = "\xf0\x9f\x91\x8e" // 👎
		}

		fmt.Fprintf(&b, "- %s %s", emoji, fb.OpportunityTitle)

		// Include what the evaluation said so the LLM knows what's being judged.
		if fb.EvalScore != "" || fb.EvalSummary != "" {
			b.WriteString(" — ")
			if fb.EvalScore != "" {
				fmt.Fprintf(&b, "was rated %s", fb.EvalScore)
			}
			if fb.EvalSummary != "" {
				fmt.Fprintf(&b, ": \"%s\"", fb.EvalSummary)
			}
		}
		b.WriteString("\n")

		if fb.Note != "" {
			fmt.Fprintf(&b, "  Subscriber: \"%s\"\n", fb.Note)
		}
	}

	return b.String()
}

// BuildPromptForTrace is an exported wrapper around buildPrompt for the trace CLI command.
func BuildPromptForTrace(ec core.EvalContext) string {
	return buildPrompt(ec)
}

// FormatDetectionForPrompt converts detection results into a human-readable summary.
func FormatDetectionForPrompt(detection weather.DetectionResult, now time.Time) string {
	if !detection.Detected || len(detection.Windows) == 0 {
		return "No storm window detected."
	}
	var b strings.Builder
	for i, w := range detection.Windows {
		if i > 0 {
			b.WriteString("\n")
		}
		rangeLabel := "extended-range (8-16 days out)"
		if w.IsNearRange {
			rangeLabel = "near-range (1-7 days out)"
		}
		leadDays := int(w.StartDate.Sub(now).Hours()/24) + 1
		if leadDays < 1 {
			leadDays = 1
		}
		fmt.Fprintf(&b, "- %s to %s (%s, %d days out): %.1f\" total",
			w.StartDate.Format("Mon Jan 2"), w.EndDate.Format("Mon Jan 2"),
			rangeLabel, leadDays, w.TotalIn)
	}
	return b.String()
}
