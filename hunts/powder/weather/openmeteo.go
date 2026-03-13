package weather

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

const openMeteoEndpoint = "https://api.open-meteo.com/v1/forecast"

const openMeteoHourlyVars = "temperature_2m,precipitation,wind_speed_10m,wind_gusts_10m,freezing_level_height,cloud_cover"

var openMeteoModels = []string{"gfs_seamless", "ecmwf_ifs025"}

const openMeteoHRRRModel = "gfs_hrrr"

type openMeteoHourlyData struct {
	Time                []string
	Temperature2m       []float64
	Precipitation       []float64
	WindSpeed10m        []float64
	WindGusts10m        []float64
	FreezingLevelHeight []float64
	CloudCover          []float64
}

type openMeteoRawResponse struct {
	Hourly map[string]json.RawMessage `json:"hourly"`
}

// OpenMeteoClient fetches weather forecasts from the Open-Meteo public API.
type OpenMeteoClient struct {
	client *http.Client
}

func NewOpenMeteoClient(client *http.Client) *OpenMeteoClient {
	return &OpenMeteoClient{client: client}
}

type openMeteoQuery struct {
	RegionID   string
	ResortID   string
	Lat        float64
	Lon        float64
	ElevationM int
	Country    string
	Timezone   string
}

// FetchForResort retrieves multi-model forecasts from Open-Meteo for a single resort.
func (c *OpenMeteoClient) FetchForResort(ctx context.Context, q openMeteoQuery) ([]Forecast, error) {
	reqURL := buildOpenMeteoURL(q)
	label := q.ResortID
	if label == "" {
		label = q.RegionID
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("building open-meteo request for %s: %w", label, err)
	}

	resp, err := retryDo(ctx, c.client, req)
	if err != nil {
		slog.ErrorContext(ctx, "open-meteo request failed", "resort_id", label, "error", err)
		return nil, fmt.Errorf("open-meteo request for %s: %w", label, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		slog.ErrorContext(ctx, "open-meteo returned non-200", "resort_id", label, "status", resp.StatusCode)
		return nil, fmt.Errorf("open-meteo returned HTTP %d for %s", resp.StatusCode, label)
	}

	var raw openMeteoRawResponse
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		slog.ErrorContext(ctx, "open-meteo response decode failed", "resort_id", label, "error", err)
		return nil, fmt.Errorf("decoding open-meteo response for %s: %w", label, err)
	}

	now := time.Now()
	models := openMeteoModels
	if q.Country == "US" {
		models = append(append([]string{}, openMeteoModels...), openMeteoHRRRModel)
	}
	perModel := extractMultiModelData(raw.Hourly, models)

	if len(perModel) == 0 {
		h, err := extractSingleModelData(raw.Hourly)
		if err != nil {
			return nil, fmt.Errorf("parsing open-meteo hourly data for %s: %w", label, err)
		}
		daily, err := parseOpenMeteoHourly(h)
		if err != nil {
			return nil, fmt.Errorf("parsing open-meteo hourly data for %s: %w", label, err)
		}
		return []Forecast{{
			RegionID: q.RegionID, ResortID: q.ResortID,
			FetchedAt: now, Source: "open_meteo", DailyData: daily,
		}}, nil
	}

	var forecasts []Forecast
	for model, h := range perModel {
		daily, err := parseOpenMeteoHourly(h)
		if err != nil {
			slog.WarnContext(ctx, "open-meteo model parse failed, skipping",
				"resort_id", label, "model", model, "error", err)
			continue
		}
		forecasts = append(forecasts, Forecast{
			RegionID:  q.RegionID,
			ResortID:  q.ResortID,
			FetchedAt: now,
			Source:    "open_meteo",
			Model:     model,
			DailyData: daily,
		})
	}

	if len(forecasts) == 0 {
		return nil, fmt.Errorf("no valid model data parsed from open-meteo for %s", label)
	}

	sort.Slice(forecasts, func(i, j int) bool {
		return forecasts[i].Model < forecasts[j].Model
	})

	return forecasts, nil
}

func buildOpenMeteoURL(q openMeteoQuery) string {
	params := url.Values{}
	params.Set("latitude", strconv.FormatFloat(q.Lat, 'f', -1, 64))
	params.Set("longitude", strconv.FormatFloat(q.Lon, 'f', -1, 64))
	params.Set("hourly", openMeteoHourlyVars)
	params.Set("forecast_days", "16")

	if q.ElevationM > 0 {
		params.Set("elevation", strconv.Itoa(q.ElevationM))
	}

	models := openMeteoModels
	if q.Country == "US" {
		models = append(append([]string{}, openMeteoModels...), openMeteoHRRRModel)
	}
	params.Set("models", strings.Join(models, ","))

	tz := q.Timezone
	if tz == "" {
		tz = "auto"
	}
	params.Set("timezone", tz)
	return openMeteoEndpoint + "?" + params.Encode()
}

func extractMultiModelData(hourly map[string]json.RawMessage, models []string) map[string]openMeteoHourlyData {
	timeArr := decodeStringArray(hourly["time"])
	if len(timeArr) == 0 {
		return nil
	}

	result := make(map[string]openMeteoHourlyData)
	for _, model := range models {
		suffix := "_" + model
		temp := decodeFloat64Array(hourly["temperature_2m"+suffix])
		precip := decodeFloat64Array(hourly["precipitation"+suffix])
		wind := decodeFloat64Array(hourly["wind_speed_10m"+suffix])
		gust := decodeFloat64Array(hourly["wind_gusts_10m"+suffix])
		fzLvl := decodeFloat64Array(hourly["freezing_level_height"+suffix])
		cloudCover := decodeFloat64Array(hourly["cloud_cover"+suffix])

		if len(temp) == 0 || len(precip) == 0 {
			continue
		}

		modelLen := len(temp)
		if len(precip) < modelLen {
			modelLen = len(precip)
		}
		modelTime := timeArr
		if modelLen < len(timeArr) {
			modelTime = timeArr[:modelLen]
		}

		validLen := detectPlaceholderCutoff(temp[:modelLen], precip[:modelLen])
		if validLen == 0 {
			continue
		}

		result[model] = openMeteoHourlyData{
			Time:                modelTime[:validLen],
			Temperature2m:       temp[:validLen],
			Precipitation:       precip[:validLen],
			WindSpeed10m:        truncFloat64(wind, validLen),
			WindGusts10m:        truncFloat64(gust, validLen),
			FreezingLevelHeight: truncFloat64(fzLvl, validLen),
			CloudCover:          truncFloat64(cloudCover, validLen),
		}
	}
	return result
}

func extractSingleModelData(hourly map[string]json.RawMessage) (openMeteoHourlyData, error) {
	timeArr := decodeStringArray(hourly["time"])
	if len(timeArr) == 0 {
		return openMeteoHourlyData{}, fmt.Errorf("hourly.time array is empty")
	}
	return openMeteoHourlyData{
		Time:                timeArr,
		Temperature2m:       decodeFloat64Array(hourly["temperature_2m"]),
		Precipitation:       decodeFloat64Array(hourly["precipitation"]),
		WindSpeed10m:        decodeFloat64Array(hourly["wind_speed_10m"]),
		WindGusts10m:        decodeFloat64Array(hourly["wind_gusts_10m"]),
		FreezingLevelHeight: decodeFloat64Array(hourly["freezing_level_height"]),
		CloudCover:          decodeFloat64Array(hourly["cloud_cover"]),
	}, nil
}

func detectPlaceholderCutoff(temp, precip []float64) int {
	n := len(temp)
	if n == 0 {
		return 0
	}
	cutoff := n
	for i := n - 1; i >= 0; i-- {
		if temp[i] == 0 && precip[i] == 0 {
			cutoff = i
		} else {
			break
		}
	}
	placeholderHours := n - cutoff
	if placeholderHours < 6 {
		return n
	}
	return cutoff
}

func truncFloat64(arr []float64, maxLen int) []float64 {
	if len(arr) <= maxLen {
		return arr
	}
	return arr[:maxLen]
}

func decodeFloat64Array(raw json.RawMessage) []float64 {
	if raw == nil {
		return nil
	}
	var arr []float64
	if err := json.Unmarshal(raw, &arr); err != nil {
		return nil
	}
	return arr
}

func decodeStringArray(raw json.RawMessage) []string {
	if raw == nil {
		return nil
	}
	var arr []string
	if err := json.Unmarshal(raw, &arr); err != nil {
		return nil
	}
	return arr
}

func parseOpenMeteoHourly(h openMeteoHourlyData) ([]DailyForecast, error) {
	n := len(h.Time)
	if n == 0 {
		return nil, fmt.Errorf("hourly.time array is empty")
	}
	if len(h.Temperature2m) != n || len(h.Precipitation) != n {
		return nil, fmt.Errorf("hourly arrays have inconsistent lengths (time=%d, temp=%d, precip=%d)",
			n, len(h.Temperature2m), len(h.Precipitation))
	}

	hasWind := len(h.WindSpeed10m) == n && len(h.WindGusts10m) == n
	hasFreezingLevel := len(h.FreezingLevelHeight) == n
	hasCloudCover := len(h.CloudCover) == n

	type dayAccum struct {
		date               string
		snowCM             float64
		shelteredSnowCM    float64
		tempMin            float64
		tempMax            float64
		precipMM           float64
		freezingLevelM     float64
		freezingLevelInit  bool
		totalSnowPrecipMM  float64
		weightedSLRSum     float64
		rainHours          int
		mixedHours         int
		daySnowCM              float64
		dayShelteredSnowCM     float64
		dayTempMax             float64
		dayPrecipMM            float64
		dayWindMax             float64
		dayGustMax             float64
		dayFzLvlMin            float64
		dayFzLvlMax            float64
		dayFzLvlInit           bool
		nightSnowCM            float64
		nightShelteredSnowCM   float64
		nightTempMin           float64
		nightPrecipMM          float64
		nightWindMax           float64
		nightGustMax           float64
		nightFzLvlMin          float64
		nightFzLvlMax          float64
		nightFzLvlInit         bool
		dayCloudCoverSum       float64
		dayCloudCoverCount     int
		nightCloudCoverSum     float64
		nightCloudCoverCount   int
		dayInit                bool
		nightInit              bool
		tempInit               bool
	}
	byDate := make(map[string]*dayAccum)

	for i, ts := range h.Time {
		t, err := time.Parse("2006-01-02T15:04", ts)
		if err != nil {
			continue
		}
		dateKey := t.Format("2006-01-02")
		hour := t.Hour()

		acc, ok := byDate[dateKey]
		if !ok {
			acc = &dayAccum{date: dateKey}
			byDate[dateKey] = acc
		}

		temp := h.Temperature2m[i]
		precip := h.Precipitation[i]
		var wind, gust float64
		if hasWind {
			wind = h.WindSpeed10m[i]
			gust = h.WindGusts10m[i]
		}

		windMs := wind / 3.6
		snowCM := SnowfallFromPrecip(precip, temp, windMs)
		shelteredWindMs := windMs * WindShelterFactor
		shelteredSnowCM := SnowfallFromPrecip(precip, temp, shelteredWindMs)
		density := CalculateDensity(temp, windMs)
		slr := SLRFromDensity(density)

		if precip > 0 {
			if IsRain(temp) {
				acc.rainHours++
			} else if IsMixedPrecip(temp) {
				acc.mixedHours++
			}
		}

		if snowCM > 0 && precip > 0 {
			acc.totalSnowPrecipMM += precip
			acc.weightedSLRSum += precip * slr
		}

		acc.snowCM += snowCM
		acc.shelteredSnowCM += shelteredSnowCM
		acc.precipMM += precip
		if !acc.tempInit {
			acc.tempMin = temp
			acc.tempMax = temp
			acc.tempInit = true
		} else {
			acc.tempMin = math.Min(acc.tempMin, temp)
			acc.tempMax = math.Max(acc.tempMax, temp)
		}

		if hasFreezingLevel {
			fzLvl := h.FreezingLevelHeight[i]
			if !acc.freezingLevelInit {
				acc.freezingLevelM = fzLvl
				acc.freezingLevelInit = true
			} else {
				acc.freezingLevelM = math.Min(acc.freezingLevelM, fzLvl)
			}
		}

		if hour >= 6 && hour < 18 {
			acc.daySnowCM += snowCM
			acc.dayShelteredSnowCM += shelteredSnowCM
			acc.dayPrecipMM += precip
			acc.dayWindMax = math.Max(acc.dayWindMax, wind)
			acc.dayGustMax = math.Max(acc.dayGustMax, gust)
			if !acc.dayInit {
				acc.dayTempMax = temp
				acc.dayInit = true
			} else {
				acc.dayTempMax = math.Max(acc.dayTempMax, temp)
			}
			if hasFreezingLevel {
				fzLvl := h.FreezingLevelHeight[i]
				if !acc.dayFzLvlInit {
					acc.dayFzLvlMin = fzLvl
					acc.dayFzLvlMax = fzLvl
					acc.dayFzLvlInit = true
				} else {
					acc.dayFzLvlMin = math.Min(acc.dayFzLvlMin, fzLvl)
					acc.dayFzLvlMax = math.Max(acc.dayFzLvlMax, fzLvl)
				}
			}
			if hasCloudCover {
				acc.dayCloudCoverSum += h.CloudCover[i]
				acc.dayCloudCoverCount++
			}
		} else {
			acc.nightSnowCM += snowCM
			acc.nightShelteredSnowCM += shelteredSnowCM
			acc.nightPrecipMM += precip
			acc.nightWindMax = math.Max(acc.nightWindMax, wind)
			acc.nightGustMax = math.Max(acc.nightGustMax, gust)
			if !acc.nightInit {
				acc.nightTempMin = temp
				acc.nightInit = true
			} else {
				acc.nightTempMin = math.Min(acc.nightTempMin, temp)
			}
			if hasFreezingLevel {
				fzLvl := h.FreezingLevelHeight[i]
				if !acc.nightFzLvlInit {
					acc.nightFzLvlMin = fzLvl
					acc.nightFzLvlMax = fzLvl
					acc.nightFzLvlInit = true
				} else {
					acc.nightFzLvlMin = math.Min(acc.nightFzLvlMin, fzLvl)
					acc.nightFzLvlMax = math.Max(acc.nightFzLvlMax, fzLvl)
				}
			}
			if hasCloudCover {
				acc.nightCloudCoverSum += h.CloudCover[i]
				acc.nightCloudCoverCount++
			}
		}
	}

	dates := make([]string, 0, len(byDate))
	for d := range byDate {
		dates = append(dates, d)
	}
	sort.Strings(dates)

	forecasts := make([]DailyForecast, 0, len(dates))
	for _, d := range dates {
		acc := byDate[d]
		t, _ := time.Parse("2006-01-02", d)

		var slRatio float64
		if acc.totalSnowPrecipMM > 0 {
			slRatio = acc.weightedSLRSum / acc.totalSnowPrecipMM
		}

		forecasts = append(forecasts, DailyForecast{
			Date:                t,
			SnowfallCM:          acc.snowCM,
			ShelteredSnowfallCM: acc.shelteredSnowCM,
			TemperatureMinC:     acc.tempMin,
			TemperatureMaxC:     acc.tempMax,
			PrecipitationMM:     acc.precipMM,
			FreezingLevelM:      acc.freezingLevelM,
			SLRatio:             slRatio,
			RainHours:           acc.rainHours,
			MixedHours:          acc.mixedHours,
			Day: HalfDay{
				SnowfallCM:          acc.daySnowCM,
				ShelteredSnowfallCM: acc.dayShelteredSnowCM,
				TemperatureC:        acc.dayTempMax,
				PrecipitationMM:     acc.dayPrecipMM,
				WindSpeedKmh:        acc.dayWindMax,
				WindGustKmh:         acc.dayGustMax,
				FreezingLevelMinM:   acc.dayFzLvlMin,
				FreezingLevelMaxM:   acc.dayFzLvlMax,
				CloudCoverPct:       safeDivide(acc.dayCloudCoverSum, float64(acc.dayCloudCoverCount)),
			},
			Night: HalfDay{
				SnowfallCM:          acc.nightSnowCM,
				ShelteredSnowfallCM: acc.nightShelteredSnowCM,
				TemperatureC:        acc.nightTempMin,
				PrecipitationMM:     acc.nightPrecipMM,
				WindSpeedKmh:        acc.nightWindMax,
				WindGustKmh:         acc.nightGustMax,
				FreezingLevelMinM:   acc.nightFzLvlMin,
				FreezingLevelMaxM:   acc.nightFzLvlMax,
				CloudCoverPct:       safeDivide(acc.nightCloudCoverSum, float64(acc.nightCloudCoverCount)),
			},
		})
	}
	return forecasts, nil
}

func safeDivide(num, denom float64) float64 {
	if denom == 0 {
		return 0
	}
	return num / denom
}
