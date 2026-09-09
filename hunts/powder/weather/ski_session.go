package weather

import (
	"math"
	"time"
)

// SkiSessionSnow uses explicitly assumed resort-local operating hours. These
// estimates describe snowfall, not preserved/untracked depth or confirmed access.
type SkiSessionSnow struct {
	OpenHour           int
	CloseHour          int
	OpeningSnowCM      *float64 // Previous day's close through this day's opening.
	DuringSkiingSnowCM *float64 // This day's opening through closing.
}

// hourlySnow is keyed by interval START in UTC; loc defines resort-local boundaries.
// Requiring all hours avoids presenting truncated forecasts as full totals.
func attachSkiSessions(days []DailyForecast, hourlySnow map[time.Time]float64, loc *time.Location) {
	for i := range days {
		date := days[i].Date
		opening := time.Date(date.Year(), date.Month(), date.Day(), 9, 0, 0, 0, loc)
		closing := time.Date(date.Year(), date.Month(), date.Day(), 16, 0, 0, 0, loc)
		previousClose := closing.AddDate(0, 0, -1)
		days[i].SkiSession = &SkiSessionSnow{OpenHour: 9, CloseHour: 16,
			OpeningSnowCM:      completeSnowTotal(hourlySnow, previousClose, opening),
			DuringSkiingSnowCM: completeSnowTotal(hourlySnow, opening, closing)}
	}
}

func completeSnowTotal(hours map[time.Time]float64, start, end time.Time) *float64 {
	total := 0.0
	for at := start; at.Before(end); at = at.Add(time.Hour) {
		snow, ok := hours[at.UTC()]
		if !ok || math.IsNaN(snow) || math.IsInf(snow, 0) || snow < 0 {
			return nil
		}
		total += snow
	}
	return &total
}
