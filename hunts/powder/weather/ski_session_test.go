package weather

import (
	"encoding/json"
	"math"
	"testing"
	"time"
)

func TestSkiSessionCombinesPreviousEveningAndOpeningMorning(t *testing.T) {
	start := time.Date(2026, 3, 13, 0, 0, 0, 0, time.UTC)
	h := openMeteoHourlyData{}
	for i := 0; i <= 48; i++ {
		end := start.Add(time.Duration(i) * time.Hour)
		h.Time = append(h.Time, end.Format("2006-01-02T15:04"))
		h.Temperature2m = append(h.Temperature2m, -5)
		amount := 1.0
		// A large Friday morning snowfall must not leak into Saturday opening.
		if i == 8 {
			amount = 100
		}
		h.Precipitation = append(h.Precipitation, amount)
	}
	days, err := parseOpenMeteoHourly(h)
	if err != nil {
		t.Fatal(err)
	}
	perHour := SnowfallFromPrecip(1, -5, 0)
	session := days[1].SkiSession
	if session == nil {
		t.Fatal("missing ski-session totals")
	}
	assertSnow(t, session.OpeningSnowCM, 17*perHour)
	assertSnow(t, session.DuringSkiingSnowCM, 7*perHour)
	if session.OpenHour != 9 || session.CloseHour != 16 {
		t.Fatal("expected explicit 09–16 default hours")
	}
	if days[0].SkiSession.OpeningSnowCM != nil {
		t.Fatal("incomplete first opening period must not look complete")
	}
}

func TestSkiSessionMissingHourIsUnknownNotZero(t *testing.T) {
	hours := map[time.Time]float64{}
	start := time.Date(2026, 3, 13, 16, 0, 0, 0, time.UTC)
	for i := 0; i < 24; i++ {
		if i != 5 {
			hours[start.Add(time.Duration(i)*time.Hour)] = 0
		}
	}
	days := []DailyForecast{{Date: time.Date(2026, 3, 14, 0, 0, 0, 0, time.UTC)}}
	attachSkiSessions(days, hours, time.UTC)
	if days[0].SkiSession.OpeningSnowCM != nil {
		t.Fatal("missing hour reported as no snow")
	}
	assertSnow(t, days[0].SkiSession.DuringSkiingSnowCM, 0)
}

func TestNWSSkiSessionUsesLocalDateAndIntervalStarts(t *testing.T) {
	loc := time.FixedZone("resort", -6*3600)
	start := time.Date(2026, 3, 13, 16, 0, 0, 0, loc)
	values := []map[string]any{}
	for i := 0; i < 24; i++ {
		values = append(values, map[string]any{"validTime": start.Add(time.Duration(i)*time.Hour).Format(time.RFC3339) + "/PT1H", "value": -5})
	}
	body, _ := json.Marshal(map[string]any{"properties": map[string]any{
		"temperature":               map[string]any{"values": values},
		"quantitativePrecipitation": map[string]any{"values": []map[string]any{{"validTime": start.Format(time.RFC3339) + "/PT24H", "value": 24}}},
	}})
	days, err := parseGridpointForecast(body, loc)
	if err != nil {
		t.Fatal(err)
	}
	assertSnow(t, days[1].SkiSession.OpeningSnowCM, 17*SnowfallFromPrecip(1, -5, 0))
	assertSnow(t, days[1].SkiSession.DuringSkiingSnowCM, 7*SnowfallFromPrecip(1, -5, 0))
}

func TestLegacySnapshotHasNoInventedSkiSession(t *testing.T) {
	var day DailyForecast
	if err := json.Unmarshal([]byte(`{"Day":{"SnowfallCM":5},"Night":{"SnowfallCM":10}}`), &day); err != nil {
		t.Fatal(err)
	}
	if day.SkiSession != nil {
		t.Fatal("legacy aggregates cannot reconstruct opening snow")
	}
}

func TestNWSSkiSessionUsesConstantTemperatureAcrossInterval(t *testing.T) {
	body := []byte(`{"properties":{"temperature":{"values":[{"validTime":"2026-03-13T16:00:00Z/PT24H","value":-5}]},"quantitativePrecipitation":{"values":[{"validTime":"2026-03-13T16:00:00Z/PT24H","value":24}]}}}`)
	days, err := parseGridpointForecast(body, time.UTC)
	if err != nil {
		t.Fatal(err)
	}
	assertSnow(t, days[1].SkiSession.OpeningSnowCM, 17*SnowfallFromPrecip(1, -5, 0))
}

func TestSessionAcrossDaylightSaving(t *testing.T) {
	loc, err := time.LoadLocation("America/Denver")
	if err != nil {
		t.Fatal(err)
	}
	for _, date := range []time.Time{time.Date(2026, 3, 8, 0, 0, 0, 0, time.UTC), time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)} {
		open := time.Date(date.Year(), date.Month(), date.Day(), 9, 0, 0, 0, loc)
		close := time.Date(date.Year(), date.Month(), date.Day(), 16, 0, 0, 0, loc)
		start := close.AddDate(0, 0, -1)
		hours := map[time.Time]float64{}
		for at := start; at.Before(close); at = at.Add(time.Hour) {
			hours[at.UTC()] = 1
		}
		days := []DailyForecast{{Date: date}}
		attachSkiSessions(days, hours, loc)
		assertSnow(t, days[0].SkiSession.OpeningSnowCM, open.Sub(start).Hours())
		assertSnow(t, days[0].SkiSession.DuringSkiingSnowCM, 7)
	}
}

func TestOpenMeteoNullHourCannotCompleteOpening(t *testing.T) {
	start := time.Date(2026, 3, 13, 16, 0, 0, 0, time.UTC)
	times := []string{}
	temp := []any{}
	precip := []any{}
	for i := 1; i <= 24; i++ {
		times = append(times, start.Add(time.Duration(i)*time.Hour).Format("2006-01-02T15:04"))
		temp = append(temp, -5)
		precip = append(precip, 1)
	}
	precip[4] = nil
	body, _ := json.Marshal(map[string]any{"hourly": map[string]any{"time": times, "temperature_2m": temp, "precipitation": precip}})
	var raw openMeteoRawResponse
	json.Unmarshal(body, &raw)
	h, err := extractSingleModelData(raw.Hourly)
	if err != nil {
		t.Fatal(err)
	}
	days, err := parseOpenMeteoHourly(h)
	if err != nil {
		t.Fatal(err)
	}
	if days[1].SkiSession.OpeningSnowCM != nil {
		t.Fatal("null precipitation incorrectly counted as a covered zero-snow hour")
	}
	assertSnow(t, days[1].SkiSession.DuringSkiingSnowCM, 7*SnowfallFromPrecip(1, -5, 0))
}

func TestOpenMeteoSessionUsesResortTimezoneAcrossSpringDST(t *testing.T) {
	loc, err := time.LoadLocation("America/Denver")
	if err != nil {
		t.Fatal(err)
	}
	start := time.Date(2026, 3, 7, 16, 0, 0, 0, loc)
	end := time.Date(2026, 3, 8, 16, 0, 0, 0, loc)
	h := openMeteoHourlyData{Location: loc}
	for at := start.Add(time.Hour); !at.After(end); at = at.Add(time.Hour) {
		h.Time = append(h.Time, at.Format("2006-01-02T15:04"))
		h.Temperature2m = append(h.Temperature2m, -5)
		h.Precipitation = append(h.Precipitation, 1)
	}
	days, err := parseOpenMeteoHourly(h)
	if err != nil {
		t.Fatal(err)
	}
	assertSnow(t, days[1].SkiSession.OpeningSnowCM, 16*SnowfallFromPrecip(1, -5, 0))
	assertSnow(t, days[1].SkiSession.DuringSkiingSnowCM, 7*SnowfallFromPrecip(1, -5, 0))
	encoded, err := json.Marshal(days)
	if err != nil {
		t.Fatal(err)
	}
	var restored []DailyForecast
	if err = json.Unmarshal(encoded, &restored); err != nil {
		t.Fatal(err)
	}
	assertSnow(t, restored[1].SkiSession.OpeningSnowCM, 16*SnowfallFromPrecip(1, -5, 0))
}

func TestNWSRepeatedClockHourKeepsSeparateTemperatures(t *testing.T) {
	loc, err := time.LoadLocation("America/Denver")
	if err != nil {
		t.Fatal(err)
	}
	start := time.Date(2026, 10, 31, 16, 0, 0, 0, loc)
	opening := time.Date(2026, 11, 1, 9, 0, 0, 0, loc)
	temps := []map[string]any{}
	precip := []map[string]any{}
	want := 0.0
	for at := start; at.Before(opening); at = at.Add(time.Hour) {
		temp := -5.0
		_, offset := at.Zone()
		if at.Hour() == 1 && offset == -6*3600 {
			temp = -15
		}
		interval := at.Format(time.RFC3339) + "/PT1H"
		temps = append(temps, map[string]any{"validTime": interval, "value": temp})
		precip = append(precip, map[string]any{"validTime": interval, "value": 1})
		want += SnowfallFromPrecip(1, temp, 0)
	}
	body, _ := json.Marshal(map[string]any{"properties": map[string]any{"temperature": map[string]any{"values": temps}, "quantitativePrecipitation": map[string]any{"values": precip}}})
	days, err := parseGridpointForecast(body, loc)
	if err != nil {
		t.Fatal(err)
	}
	assertSnow(t, days[1].SkiSession.OpeningSnowCM, want)
}

func assertSnow(t *testing.T, got *float64, want float64) {
	t.Helper()
	if got == nil || math.Abs(*got-want) > 0.001 {
		t.Fatalf("snow %v, want %.3f", got, want)
	}
}
