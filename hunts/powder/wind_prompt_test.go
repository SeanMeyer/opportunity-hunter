package powder

import (
	"github.com/seanmeyer/opportunity-hunter/hunts/powder/weather"
	"strings"
	"testing"
	"time"
)

func TestWindDirectionReachesPrompt(t *testing.T) {
	north := 0.0
	forecasts := []weather.Forecast{{ResortID: "test", Source: "open_meteo", Model: "gfs", DailyData: []weather.DailyForecast{
		{Date: time.Date(2026, 3, 12, 0, 0, 0, 0, time.UTC), Day: weather.HalfDay{WindDirectionDeg: &north, WindGustKmh: 16}, Night: weather.HalfDay{WindDirectionVariable: true, WindGustKmh: 32}},
		{Date: time.Date(2026, 3, 13, 0, 0, 0, 0, time.UTC)},
	}}}
	got := FormatConsolidatedWeatherForPrompt(forecasts, nil)
	for _, want := range []string{"N (0°)", "Variable", "Unknown", "FROM", "open_meteo / gfs", "Night gust", "06:00", "18:00"} {
		if !strings.Contains(got, want) {
			t.Errorf("prompt missing %q", want)
		}
	}
}
