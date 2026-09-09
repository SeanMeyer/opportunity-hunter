package powder

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	genai "google.golang.org/genai"

	"github.com/seanmeyer/opportunity-hunter/core"
	"github.com/seanmeyer/opportunity-hunter/hunts/powder/weather"
	"github.com/seanmeyer/opportunity-hunter/llm"
)

type powderEvaluator struct {
	llm interface {
		TwoStepAdvice(context.Context, string, *genai.Schema) (llm.TwoStepResult, error)
	}
}

func (e *powderEvaluator) Evaluate(ctx context.Context, ec core.EvalContext) (*core.EvalResult, error) {
	prompt := buildPrompt(ec)

	// Use the two-step Gemini evaluation: research with grounding, then structured extraction.
	twoStep, err := e.llm.TwoStepAdvice(ctx, prompt, stormEvalSchema())
	if err != nil {
		return nil, fmt.Errorf("powder evaluate: %w", err)
	}

	structured := twoStep.Structured

	tier := weather.NormalizeTier(weather.Tier(stringField(structured, "tier")))
	recommendation := stringField(structured, "recommendation")
	switch tier {
	case weather.TierDropEverything, weather.TierRecommended, weather.TierWatch, weather.TierSkip:
	default:
		return nil, fmt.Errorf("powder evaluate: invalid verdict %q", tier)
	}
	if strings.TrimSpace(recommendation) == "" {
		return nil, fmt.Errorf("powder evaluate: missing recommendation")
	}
	summary := stringField(structured, "summary")
	strategy := stringField(structured, "strategy")
	snowQuality := stringField(structured, "snow_quality")
	crowdEstimate := stringField(structured, "crowd_estimate")
	informationEdge := stringField(structured, "information_edge")
	closureRisk := stringField(structured, "closure_risk")
	bestSkiDay := stringField(structured, "best_ski_day")
	bestSkiDayReason := stringField(structured, "best_ski_day_reason")

	keyFactors := weather.KeyFactors{
		Pros: stringSliceField(structured, "key_factors_pros"),
		Cons: stringSliceField(structured, "key_factors_cons"),
	}
	logistics := weather.LogisticsSummary{
		Lodging:            stringField(structured, "logistics_lodging"),
		Transportation:     stringField(structured, "logistics_transportation"),
		RoadConditions:     stringField(structured, "logistics_road_conditions"),
		FlightCost:         stringField(structured, "logistics_flight_cost"),
		CarRental:          stringField(structured, "logistics_car_rental"),
		LodgingCost:        stringField(structured, "logistics_lodging_cost"),
		TotalEstimatedCost: stringField(structured, "logistics_total_estimated_cost"),
	}
	resortInsights := parseResortInsights(structured)
	dayByDay := parseDayByDay(structured)

	groundingSources := twoStep.Sources
	if len(groundingSources) == 0 {
		groundingSources = stringSliceField(structured, "research_sources")
	}

	evalAttrs := EvalAttrs{
		Tier:             string(tier),
		Recommendation:   recommendation,
		Summary:          summary,
		Strategy:         strategy,
		SnowQuality:      snowQuality,
		CrowdEstimate:    crowdEstimate,
		InformationEdge:  informationEdge,
		ClosureRisk:      closureRisk,
		BestSkiDay:       bestSkiDay,
		BestSkiDayReason: bestSkiDayReason,
		KeyFactors:       keyFactors,
		Logistics:        logistics,
		ResortInsights:   resortInsights,
		DayByDay:         dayByDay,
		GroundingSources: groundingSources,
	}

	var picks []core.Pick
	for _, opp := range ec.Opportunities {
		attrs, _ := DecodePowderAttrs(opp.Attributes)

		displayScore := string(tier)
		score := weather.TierScore(tier)

		// Classify change relative to prior evaluation.
		changeClass := weather.ChangeNew
		if ec.PriorEval != nil {
			priorTier := extractPriorTier(ec.PriorEval)
			priorSnow := attrs.SnowfallIn
			for _, priorPick := range ec.PriorPicks {
				if priorPick.OpportunityID == opp.ID {
					priorTier = weather.NormalizeTier(weather.Tier(priorPick.DisplayScore))
					if a, err := DecodePowderAttrs(priorPick.Attributes); err == nil {
						priorSnow = a.SnowfallIn
					}
					break
				}
			}
			changeClass = Compare(priorTier, tier, priorSnow, attrs.SnowfallIn)
		}
		attrs.ChangeClass = string(changeClass)
		combined := mergeAttrs(attrs, evalAttrs)

		picks = append(picks, core.Pick{
			OpportunityID: opp.ID,
			Score:         score,
			DisplayScore:  displayScore,
			Reason:        recommendation,
			Attributes:    combined,
		})
	}

	return &core.EvalResult{
		Evaluation: core.Evaluation{
			HuntName:           "powder",
			EvaluatedAt:        time.Now(),
			RawLLMResponse:     twoStep.Research,
			StructuredResponse: twoStep.RawJSON,
			RenderedPrompt:     twoStep.RenderedPrompt,
			CostUSD:            twoStep.CostUSD,
		},
		Picks: picks,
	}, nil
}

// EvalAttrs holds the rich evaluation data from Gemini.
type EvalAttrs struct {
	Tier             string                   `json:"tier"`
	Recommendation   string                   `json:"recommendation"`
	Summary          string                   `json:"summary"`
	Strategy         string                   `json:"strategy"`
	SnowQuality      string                   `json:"snow_quality"`
	CrowdEstimate    string                   `json:"crowd_estimate"`
	InformationEdge  string                   `json:"information_edge"`
	ClosureRisk      string                   `json:"closure_risk"`
	BestSkiDay       string                   `json:"best_ski_day"`
	BestSkiDayReason string                   `json:"best_ski_day_reason"`
	KeyFactors       weather.KeyFactors       `json:"key_factors"`
	Logistics        weather.LogisticsSummary `json:"logistics"`
	ResortInsights   []weather.ResortInsight  `json:"resort_insights"`
	DayByDay         []weather.DayEvaluation  `json:"day_by_day"`
	GroundingSources []string                 `json:"grounding_sources"`
}

func mergeAttrs(powder PowderAttrs, eval EvalAttrs) core.Attributes {
	merged := map[string]any{
		"snowfall_in":         powder.SnowfallIn,
		"consensus":           powder.Consensus,
		"friction_tier":       powder.FrictionTier,
		"change_class":        powder.ChangeClass,
		"weather_window":      powder.WeatherWindow,
		"storm_group":         powder.StormGroup,
		"tier":                eval.Tier,
		"recommendation":      eval.Recommendation,
		"summary":             eval.Summary,
		"strategy":            eval.Strategy,
		"snow_quality":        eval.SnowQuality,
		"crowd_estimate":      eval.CrowdEstimate,
		"information_edge":    eval.InformationEdge,
		"closure_risk":        eval.ClosureRisk,
		"best_ski_day":        eval.BestSkiDay,
		"best_ski_day_reason": eval.BestSkiDayReason,
		"key_factors":         eval.KeyFactors,
		"logistics":           eval.Logistics,
		"resort_insights":     eval.ResortInsights,
		"day_by_day":          eval.DayByDay,
		"grounding_sources":   eval.GroundingSources,
	}
	b, _ := json.Marshal(merged)
	return b
}

// stormEvalSchema defines the structured output schema for Gemini.
func stormEvalSchema() *genai.Schema {
	return &genai.Schema{
		Type: genai.TypeObject,
		Properties: map[string]*genai.Schema{
			"tier": {
				Type: genai.TypeString,
				Enum: []string{"DROP_EVERYTHING", "RECOMMENDED", "WATCH", "SKIP"},
			},
			"recommendation": {Type: genai.TypeString, Description: "The overall judgment: 2-4 sentences explaining whether this person should go, decisive tradeoffs, uncertainty and what would change the decision. Preserve the research assessment."},
			"summary": {
				Type:        genai.TypeString,
				Description: "One sentence summarizing the verdict and its main reason, not just snowfall.",
			},
			"resort_insights": {
				Type: genai.TypeArray,
				Items: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"resort":  {Type: genai.TypeString},
						"insight": {Type: genai.TypeString},
					},
					Required: []string{"resort", "insight"},
				},
			},
			"strategy":         {Type: genai.TypeString},
			"snow_quality":     {Type: genai.TypeString},
			"crowd_estimate":   {Type: genai.TypeString},
			"information_edge": {Type: genai.TypeString},
			"closure_risk":     {Type: genai.TypeString},
			"best_ski_day": {
				Type:        genai.TypeString,
				Description: "The best date to ski in YYYY-MM-DD format, or Unknown / Not applicable if no suitable date is supported.",
			},
			"best_ski_day_reason":       {Type: genai.TypeString},
			"key_factors_pros":          {Type: genai.TypeArray, Items: &genai.Schema{Type: genai.TypeString}},
			"key_factors_cons":          {Type: genai.TypeArray, Items: &genai.Schema{Type: genai.TypeString}},
			"logistics_lodging":         {Type: genai.TypeString},
			"logistics_transportation":  {Type: genai.TypeString},
			"logistics_road_conditions": {Type: genai.TypeString},
			"logistics_flight_cost":     {Type: genai.TypeString},
			"logistics_car_rental":      {Type: genai.TypeString},
			"logistics_lodging_cost": {
				Type:        genai.TypeString,
				Description: "Lodging cost per night, identifying verified quote versus estimate and assumptions. Unknown or Not applicable is valid.",
			},
			"logistics_total_estimated_cost": {
				Type:        genai.TypeString,
				Description: "Total trip cost with assumptions if supported by research. Unknown or Not applicable is valid; never manufacture a range.",
			},
			"day_by_day": {
				Type: genai.TypeArray,
				Items: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"date":           {Type: genai.TypeString},
						"snowfall":       {Type: genai.TypeString},
						"conditions":     {Type: genai.TypeString},
						"recommendation": {Type: genai.TypeString},
					},
					Required: []string{"date", "snowfall", "conditions", "recommendation"},
				},
			},
			"research_sources": {
				Type:  genai.TypeArray,
				Items: &genai.Schema{Type: genai.TypeString},
			},
		},
		Required: []string{
			"tier", "recommendation", "summary", "resort_insights", "strategy",
			"snow_quality", "crowd_estimate", "information_edge", "closure_risk",
			"best_ski_day", "best_ski_day_reason",
			"key_factors_pros", "key_factors_cons",
			"logistics_lodging", "logistics_transportation",
			"logistics_road_conditions", "logistics_flight_cost", "logistics_car_rental",
			"logistics_lodging_cost", "logistics_total_estimated_cost",
			"day_by_day", "research_sources",
		},
	}
}

// JSON field extraction helpers.

func parseDayByDay(m map[string]any) []weather.DayEvaluation {
	raw, ok := m["day_by_day"]
	if !ok {
		return nil
	}
	items, ok := raw.([]any)
	if !ok {
		return nil
	}
	result := make([]weather.DayEvaluation, 0, len(items))
	for _, item := range items {
		entry, ok := item.(map[string]any)
		if !ok {
			continue
		}
		dateStr := stringField(entry, "date")
		t, _ := time.Parse("2006-01-02", dateStr)
		result = append(result, weather.DayEvaluation{
			Date:           t,
			Snowfall:       stringField(entry, "snowfall"),
			Conditions:     stringField(entry, "conditions"),
			Recommendation: stringField(entry, "recommendation"),
		})
	}
	return result
}

func parseResortInsights(m map[string]any) []weather.ResortInsight {
	raw, ok := m["resort_insights"]
	if !ok {
		return nil
	}
	items, ok := raw.([]any)
	if !ok {
		return nil
	}
	result := make([]weather.ResortInsight, 0, len(items))
	for _, item := range items {
		entry, ok := item.(map[string]any)
		if !ok {
			continue
		}
		result = append(result, weather.ResortInsight{
			Resort:  stringField(entry, "resort"),
			Insight: stringField(entry, "insight"),
		})
	}
	return result
}

func stringField(m map[string]any, key string) string {
	v, ok := m[key]
	if !ok {
		return ""
	}
	s, _ := v.(string)
	return s
}

// extractPriorTier gets the tier from a prior evaluation's raw LLM response.
func extractPriorTier(eval *core.Evaluation) weather.Tier {
	if eval == nil {
		return weather.TierOnTheRadar
	}
	response := eval.StructuredResponse
	if response == "" {
		response = eval.RawLLMResponse
	} // older saved JSON research
	var parsed map[string]any
	if json.Unmarshal([]byte(response), &parsed) == nil {
		if t, ok := parsed["tier"].(string); ok && t != "" {
			return weather.NormalizeTier(weather.Tier(t))
		}
	}
	return weather.TierOnTheRadar
}

func stringSliceField(m map[string]any, key string) []string {
	v, ok := m[key]
	if !ok {
		return nil
	}
	items, ok := v.([]any)
	if !ok {
		return nil
	}
	result := make([]string, 0, len(items))
	for _, item := range items {
		if s, ok := item.(string); ok {
			result = append(result, s)
		}
	}
	return result
}
