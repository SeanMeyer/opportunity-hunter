package weather

import (
	"encoding/json"
	"math"
	"strings"
	"testing"
	"time"
)

func TestDirectionFromOpenMeteo(t *testing.T) {
	for _, tc := range []struct {
		name, values string
		want         *float64
		variable     bool
	}{
		{"north wrap", "[350,10]", directionPtr(0), false},
		{"north is real", "[0,null]", directionPtr(0), false},
		{"unknown", "[null,null]", nil, false},
		{"opposing", "[90,270]", nil, true},
		{"invalid", "[-1,361]", nil, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var raw openMeteoRawResponse
			err := json.Unmarshal([]byte(`{"hourly":{"time":["2026-03-12T06:00","2026-03-12T07:00"],"temperature_2m":[-2,-2],"precipitation":[1,1],"wind_speed_10m":[10,10],"wind_gusts_10m":[20,20],"wind_direction_10m":`+tc.values+`}}`), &raw)
			if err != nil {
				t.Fatal(err)
			}
			h, err := extractSingleModelData(raw.Hourly)
			if err != nil {
				t.Fatal(err)
			}
			days, err := parseOpenMeteoHourly(h)
			if err != nil {
				t.Fatal(err)
			}
			assertDirection(t, days[0].Day, tc.want, tc.variable)
			assertDirection(t, days[0].Night, nil, false)
		})
	}
	if !strings.Contains(buildOpenMeteoURL(openMeteoQuery{}), "wind_direction_10m") {
		t.Fatal("direction not requested")
	}
}

func TestNWSWindDirectionIntervalAndLocalTime(t *testing.T) {
	body := []byte(`{"properties":{"windDirection":{"values":[{"validTime":"2026-03-12T12:00:00Z/PT2H","value":270},{"validTime":"2026-03-13T00:00:00Z/PT2H","value":0},{"validTime":"2026-03-13T02:00:00Z/PT1H","value":null}]}}}`)
	days, err := parseGridpointForecast(body, time.FixedZone("local", -6*3600))
	if err != nil {
		t.Fatal(err)
	}
	if len(days) != 1 {
		t.Fatalf("days: %d", len(days))
	}
	assertDirection(t, days[0].Day, directionPtr(270), false)
	assertDirection(t, days[0].Night, directionPtr(0), false)
}

func TestOldForecastDirectionRemainsUnknown(t *testing.T) {
	var h HalfDay
	if err := json.Unmarshal([]byte(`{"WindGustKmh":20}`), &h); err != nil {
		t.Fatal(err)
	}
	assertDirection(t, h, nil, false)
}

func TestCalmHoursDoNotBiasDirection(t *testing.T) {
	var raw openMeteoRawResponse
	if err := json.Unmarshal([]byte(`{"hourly":{"time":["2026-03-12T06:00","2026-03-12T07:00"],"temperature_2m":[-2,-2],"precipitation":[1,1],"wind_speed_10m":[0,20],"wind_gusts_10m":[0,30],"wind_direction_10m":[0,270]}}`), &raw); err != nil {
		t.Fatal(err)
	}
	h, err := extractSingleModelData(raw.Hourly)
	if err != nil {
		t.Fatal(err)
	}
	days, err := parseOpenMeteoHourly(h)
	if err != nil {
		t.Fatal(err)
	}
	assertDirection(t, days[0].Day, directionPtr(270), false)
	body := []byte(`{"properties":{"windSpeed":{"values":[{"validTime":"2026-03-12T06:00:00Z/PT1H","value":0},{"validTime":"2026-03-12T07:00:00Z/PT1H","value":20}]},"windDirection":{"values":[{"validTime":"2026-03-12T06:00:00Z/PT1H","value":0},{"validTime":"2026-03-12T07:00:00Z/PT1H","value":270}]}}}`)
	days, err = parseGridpointForecast(body, time.UTC)
	if err != nil {
		t.Fatal(err)
	}
	assertDirection(t, days[0].Day, directionPtr(270), false)
}

func TestMultiModelDirectionsStayWithTheirModel(t *testing.T) {
	var raw openMeteoRawResponse
	err := json.Unmarshal([]byte(`{"hourly":{"time":["2026-03-12T06:00","2026-03-12T07:00"],"temperature_2m_a":[-2,-2],"precipitation_a":[1,1],"wind_direction_10m_a":[270,null],"temperature_2m_b":[-2],"precipitation_b":[1],"wind_direction_10m_b":[90,180]}}`), &raw)
	if err != nil {
		t.Fatal(err)
	}
	models := extractMultiModelData(raw.Hourly, []string{"a", "b"})
	for model, want := range map[string]float64{"a": 270, "b": 90} {
		days, err := parseOpenMeteoHourly(models[model])
		if err != nil {
			t.Fatal(err)
		}
		assertDirection(t, days[0].Day, directionPtr(want), false)
		saved, err := json.Marshal(days)
		if err != nil {
			t.Fatal(err)
		}
		var restored []DailyForecast
		if err = json.Unmarshal(saved, &restored); err != nil {
			t.Fatal(err)
		}
		assertDirection(t, restored[0].Day, directionPtr(want), false)
	}
}

func directionPtr(v float64) *float64 { return &v }
func assertDirection(t *testing.T, h HalfDay, want *float64, variable bool) {
	t.Helper()
	if h.WindDirectionVariable != variable {
		t.Fatalf("variable=%v, want %v", h.WindDirectionVariable, variable)
	}
	if want == nil {
		if h.WindDirectionDeg != nil {
			t.Fatalf("unexpected direction %v", *h.WindDirectionDeg)
		}
		return
	}
	if h.WindDirectionDeg == nil {
		t.Fatal("direction missing")
	}
	difference := math.Abs(*h.WindDirectionDeg - *want)
	difference = math.Min(difference, 360-difference)
	if difference > 0.01 {
		t.Fatalf("direction=%v, want %v", *h.WindDirectionDeg, *want)
	}
}
