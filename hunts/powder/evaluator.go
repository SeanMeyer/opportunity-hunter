package powder

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	genai "google.golang.org/genai"

	"github.com/seanmeyer/opportunity-hunter/core"
	"github.com/seanmeyer/opportunity-hunter/hunts/powder/weather"
	"github.com/seanmeyer/opportunity-hunter/llm"
)

type powderEvaluator struct {
	llm *llm.Client
}

func (e *powderEvaluator) Evaluate(ctx context.Context, ec core.EvalContext) (*core.EvalResult, error) {
	prompt := buildPrompt(ec)

	// Use the two-step Gemini evaluation: research with grounding, then structured extraction.
	twoStep, err := e.llm.TwoStep(ctx, prompt, stormEvalSchema())
	if err != nil {
		return nil, fmt.Errorf("powder evaluate: %w", err)
	}

	structured := twoStep.Structured

	tier := weather.Tier(stringField(structured, "tier"))
	recommendation := stringField(structured, "recommendation")
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
		score := 0.5
		switch tier {
		case weather.TierDropEverything:
			score = 0.95
		case weather.TierWorthALook:
			score = 0.75
		}

		// Classify change relative to prior evaluation.
		changeClass := weather.ChangeNew
		if ec.PriorEval != nil {
			priorTier := extractPriorTier(ec.PriorEval)
			changeClass = Compare(priorTier, tier, attrs.SnowfallIn, attrs.SnowfallIn)
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
			HuntName:       "powder",
			EvaluatedAt:    time.Now(),
			RawLLMResponse: twoStep.Research,
			RenderedPrompt: prompt,
			CostUSD:        twoStep.CostUSD,
		},
		Picks: picks,
	}, nil
}

// EvalAttrs holds the rich evaluation data from Gemini.
type EvalAttrs struct {
	Tier             string                  `json:"tier"`
	Recommendation   string                  `json:"recommendation"`
	Summary          string                  `json:"summary"`
	Strategy         string                  `json:"strategy"`
	SnowQuality      string                  `json:"snow_quality"`
	CrowdEstimate    string                  `json:"crowd_estimate"`
	InformationEdge  string                  `json:"information_edge"`
	ClosureRisk      string                  `json:"closure_risk"`
	BestSkiDay       string                  `json:"best_ski_day"`
	BestSkiDayReason string                  `json:"best_ski_day_reason"`
	KeyFactors       weather.KeyFactors      `json:"key_factors"`
	Logistics        weather.LogisticsSummary `json:"logistics"`
	ResortInsights   []weather.ResortInsight  `json:"resort_insights"`
	DayByDay         []weather.DayEvaluation  `json:"day_by_day"`
	GroundingSources []string                `json:"grounding_sources"`
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
				Enum: []string{"DROP_EVERYTHING", "WORTH_A_LOOK", "ON_THE_RADAR"},
			},
			"recommendation": {Type: genai.TypeString},
			"summary": {
				Type:        genai.TypeString,
				Description: "A short (under 80 characters) hook: snowfall amount and best day.",
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
				Description: "The single best date to ski in YYYY-MM-DD format",
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
				Description: "Estimated lodging cost per night. Always provide a dollar range estimate.",
			},
			"logistics_total_estimated_cost": {
				Type:        genai.TypeString,
				Description: "Total estimated trip cost. Always calculate a range estimate.",
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
	if eval == nil || eval.RawLLMResponse == "" {
		return weather.TierOnTheRadar
	}
	var parsed map[string]any
	if json.Unmarshal([]byte(eval.RawLLMResponse), &parsed) == nil {
		if t, ok := parsed["tier"].(string); ok && t != "" {
			return weather.Tier(t)
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
