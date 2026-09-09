package powder

import (
	"github.com/seanmeyer/opportunity-hunter/hunts/powder/weather"
	"strings"
	"testing"
	"time"
)

func TestPromptUsesSkiSessionSnowNotCalendarNight(t *testing.T) {
	opening, during := 25.4, 5.08
	got := FormatConsolidatedWeatherForPrompt([]weather.Forecast{{DailyData: []weather.DailyForecast{{Date: time.Date(2026, 3, 14, 0, 0, 0, 0, time.UTC), Night: weather.HalfDay{SnowfallCM: 254}, SkiSession: &weather.SkiSessionSnow{OpenHour: 9, CloseHour: 16, OpeningSnowCM: &opening, DuringSkiingSnowCM: &during}}}}}, nil)
	for _, want := range []string{"Opening snow", "During skiing", "10.0\"", "2.0\"", "09:00", "16:00", "assumed"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %s", want)
		}
	}
	if strings.Contains(got, "100.0\"") || strings.Contains(got, "Night Snow") {
		t.Fatal("legacy bucket presented as ski-session snow")
	}
	legacy := FormatConsolidatedWeatherForPrompt([]weather.Forecast{{DailyData: []weather.DailyForecast{{Night: weather.HalfDay{SnowfallCM: 254}}}}}, nil)
	if !strings.Contains(legacy, "Unknown") || strings.Contains(legacy, "100.0\"") {
		t.Fatal("legacy aggregate treated as reconstructable opening snow")
	}
}

func TestPromptFlagsConsensusForPreviousEveningSnow(t *testing.T) {
	opening := 25.4
	got := FormatConsolidatedWeatherForPrompt([]weather.Forecast{{DailyData: []weather.DailyForecast{{SkiSession: &weather.SkiSessionSnow{OpenHour: 9, CloseHour: 16, OpeningSnowCM: &opening}}}}}, nil)
	if !strings.Contains(got, "see consensus") {
		t.Fatal("opening snow from the previous evening needs the consensus reference")
	}
}
