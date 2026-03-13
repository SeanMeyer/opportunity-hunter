package weather

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

var nwsBaseURL = "https://api.weather.gov"

const nwsUserAgent = "(opportunity-hunter, contact@example.com)"

// NWSClient fetches near-range forecasts from the National Weather Service API.
type NWSClient struct {
	client    *http.Client
	gridMu    sync.Mutex
	gridCache map[string]nwsGrid
}

type nwsGrid struct {
	WFO   string
	GridX int
	GridY int
}

func NewNWSClient(client *http.Client) *NWSClient {
	return &NWSClient{
		client:    client,
		gridCache: make(map[string]nwsGrid),
	}
}

// Fetch retrieves a forecast from NWS for a US region.
func (c *NWSClient) Fetch(ctx context.Context, region Region) (Forecast, error) {
	grid, err := c.resolveGrid(ctx, region.Latitude, region.Longitude)
	if err != nil {
		return Forecast{}, fmt.Errorf("nws: resolve grid for region %s: %w", region.ID, err)
	}

	raw, err := c.fetchGridpoint(ctx, grid)
	if err != nil {
		return Forecast{}, fmt.Errorf("nws: fetch gridpoint for region %s: %w", region.ID, err)
	}

	loc := time.UTC
	if region.Timezone != "" {
		if l, tzErr := time.LoadLocation(region.Timezone); tzErr == nil {
			loc = l
		}
	}

	daily, err := parseGridpointForecast(raw, loc)
	if err != nil {
		return Forecast{}, fmt.Errorf("nws: parse gridpoint for region %s: %w", region.ID, err)
	}

	return Forecast{
		RegionID:  region.ID,
		FetchedAt: time.Now().UTC(),
		Source:    "nws",
		DailyData: daily,
	}, nil
}

func (c *NWSClient) resolveGrid(ctx context.Context, lat, lon float64) (nwsGrid, error) {
	key := gridCacheKey(lat, lon)

	c.gridMu.Lock()
	if grid, ok := c.gridCache[key]; ok {
		c.gridMu.Unlock()
		return grid, nil
	}
	c.gridMu.Unlock()

	lat4 := roundTo4(lat)
	lon4 := roundTo4(lon)
	url := fmt.Sprintf("%s/points/%.4f,%.4f", nwsBaseURL, lat4, lon4)

	body, statusCode, err := c.get(ctx, url)
	if err != nil {
		return nwsGrid{}, err
	}
	if statusCode == http.StatusNotFound {
		return nwsGrid{}, fmt.Errorf("coordinates %.4f,%.4f not covered by NWS (not a US location)", lat4, lon4)
	}
	if statusCode != http.StatusOK {
		return nwsGrid{}, fmt.Errorf("points endpoint returned HTTP %d", statusCode)
	}

	var resp nwsPointsResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nwsGrid{}, fmt.Errorf("decode points response: %w", err)
	}

	grid := nwsGrid{
		WFO:   resp.Properties.GridID,
		GridX: resp.Properties.GridX,
		GridY: resp.Properties.GridY,
	}

	c.gridMu.Lock()
	c.gridCache[key] = grid
	c.gridMu.Unlock()

	return grid, nil
}

func (c *NWSClient) fetchGridpoint(ctx context.Context, grid nwsGrid) ([]byte, error) {
	url := fmt.Sprintf("%s/gridpoints/%s/%d,%d", nwsBaseURL, grid.WFO, grid.GridX, grid.GridY)
	body, statusCode, err := c.get(ctx, url)
	if err != nil {
		return nil, err
	}
	if statusCode != http.StatusOK {
		return nil, fmt.Errorf("gridpoints endpoint returned HTTP %d", statusCode)
	}
	return body, nil
}

func (c *NWSClient) get(ctx context.Context, url string) ([]byte, int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, 0, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("User-Agent", nwsUserAgent)
	req.Header.Set("Accept", "application/geo+json")

	resp, err := retryDo(ctx, c.client, req)
	if err != nil {
		return nil, 0, fmt.Errorf("http get %s: %w", url, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("read response body: %w", err)
	}
	return body, resp.StatusCode, nil
}

func parseGridpointForecast(body []byte, loc *time.Location) ([]DailyForecast, error) {
	var resp nwsGridpointResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("decode gridpoint response: %w", err)
	}

	type dayAccum struct {
		snowCM, shelteredSnowCM, precipMM      float64
		tempMin, tempMax                       float64
		tempInit                               bool
		totalSnowPrecipMM, weightedSLRSum      float64
		rainHours, mixedHours                  int
		daySnowCM, dayShelteredSnowCM, dayPrecipMM float64
		dayTempMax, dayWindMax, dayGustMax     float64
		dayInit                                bool
		nightSnowCM, nightShelteredSnowCM, nightPrecipMM float64
		nightTempMin, nightWindMax, nightGustMax float64
		nightInit                              bool
		dayCloudCoverSum, nightCloudCoverSum   float64
		dayCloudCoverCount, nightCloudCoverCount int
	}
	byDate := make(map[string]*dayAccum)
	getAcc := func(dateKey string) *dayAccum {
		if a, ok := byDate[dateKey]; ok {
			return a
		}
		a := &dayAccum{}
		byDate[dateKey] = a
		return a
	}

	walkHourly := func(values []nwsValue, fn func(acc *dayAccum, hour int, val float64)) {
		for _, v := range values {
			if v.Value == nil {
				continue
			}
			start, dur, err := parseISO8601Interval(v.ValidTime)
			if err != nil || dur <= 0 {
				continue
			}
			end := start.Add(dur)
			hourlyVal := *v.Value / dur.Hours()
			for t := start; t.Before(end); t = t.Add(time.Hour) {
				local := t.In(loc)
				dateKey := local.Format("2006-01-02")
				fn(getAcc(dateKey), local.Hour(), hourlyVal)
			}
		}
	}

	// Step 1: Build per-hour temperature lookup.
	hourlyTemp := make(map[string]float64)
	for _, v := range resp.Properties.Temperature.Values {
		if v.Value == nil {
			continue
		}
		start, dur, err := parseISO8601Interval(v.ValidTime)
		if err != nil || dur <= 0 {
			continue
		}
		end := start.Add(dur)
		hourlyVal := *v.Value / dur.Hours()
		for t := start; t.Before(end); t = t.Add(time.Hour) {
			local := t.In(loc)
			dateKey := local.Format("2006-01-02")
			hourKey := fmt.Sprintf("%s-%02d", dateKey, local.Hour())
			hourlyTemp[hourKey] = hourlyVal

			acc := getAcc(dateKey)
			c := hourlyVal
			if !acc.tempInit {
				acc.tempMin, acc.tempMax = c, c
				acc.tempInit = true
			} else {
				if c < acc.tempMin {
					acc.tempMin = c
				}
				if c > acc.tempMax {
					acc.tempMax = c
				}
			}
			if local.Hour() >= 6 && local.Hour() < 18 {
				if !acc.dayInit {
					acc.dayTempMax = c
					acc.dayInit = true
				} else if c > acc.dayTempMax {
					acc.dayTempMax = c
				}
			} else {
				if !acc.nightInit {
					acc.nightTempMin = c
					acc.nightInit = true
				} else if c < acc.nightTempMin {
					acc.nightTempMin = c
				}
			}
		}
	}

	// Step 1b: Build per-hour wind speed lookup.
	hourlyWind := make(map[string]float64)
	for _, v := range resp.Properties.WindSpeed.Values {
		if v.Value == nil {
			continue
		}
		start, dur, err := parseISO8601Interval(v.ValidTime)
		if err != nil || dur <= 0 {
			continue
		}
		end := start.Add(dur)
		hourlyVal := *v.Value / dur.Hours()
		for t := start; t.Before(end); t = t.Add(time.Hour) {
			local := t.In(loc)
			dateKey := local.Format("2006-01-02")
			hourKey := fmt.Sprintf("%s-%02d", dateKey, local.Hour())
			hourlyWind[hourKey] = hourlyVal
		}
	}

	// Step 2: Process precipitation with SLR-adjusted snowfall.
	for _, v := range resp.Properties.QuantitativePrecipitation.Values {
		if v.Value == nil {
			continue
		}
		start, dur, err := parseISO8601Interval(v.ValidTime)
		if err != nil || dur <= 0 {
			continue
		}
		end := start.Add(dur)
		hourlyMM := *v.Value / dur.Hours()
		for t := start; t.Before(end); t = t.Add(time.Hour) {
			local := t.In(loc)
			dateKey := local.Format("2006-01-02")
			hour := local.Hour()
			hourKey := fmt.Sprintf("%s-%02d", dateKey, hour)

			acc := getAcc(dateKey)
			acc.precipMM += hourlyMM

			tempC, hasTemp := hourlyTemp[hourKey]
			if !hasTemp {
				if acc.tempInit {
					tempC = (acc.tempMin + acc.tempMax) / 2
				}
			}

			windKmh := hourlyWind[hourKey]
			windMs := windKmh / 3.6

			snowCM := SnowfallFromPrecip(hourlyMM, tempC, windMs)
			shelteredWindMs := windMs * WindShelterFactor
			shelteredSnowCM := SnowfallFromPrecip(hourlyMM, tempC, shelteredWindMs)
			density := CalculateDensity(tempC, windMs)
			slr := SLRFromDensity(density)

			if hourlyMM > 0 {
				if IsRain(tempC) {
					acc.rainHours++
				} else if IsMixedPrecip(tempC) {
					acc.mixedHours++
				}
			}
			if snowCM > 0 && hourlyMM > 0 {
				acc.totalSnowPrecipMM += hourlyMM
				acc.weightedSLRSum += hourlyMM * slr
			}

			acc.snowCM += snowCM
			acc.shelteredSnowCM += shelteredSnowCM
			if hour >= 6 && hour < 18 {
				acc.daySnowCM += snowCM
				acc.dayShelteredSnowCM += shelteredSnowCM
				acc.dayPrecipMM += hourlyMM
			} else {
				acc.nightSnowCM += snowCM
				acc.nightShelteredSnowCM += shelteredSnowCM
				acc.nightPrecipMM += hourlyMM
			}
		}
	}

	// Step 3: Wind speed and gusts.
	walkHourly(resp.Properties.WindSpeed.Values, func(acc *dayAccum, hour int, kmh float64) {
		if hour >= 6 && hour < 18 {
			if kmh > acc.dayWindMax {
				acc.dayWindMax = kmh
			}
		} else {
			if kmh > acc.nightWindMax {
				acc.nightWindMax = kmh
			}
		}
	})
	walkHourly(resp.Properties.WindGust.Values, func(acc *dayAccum, hour int, kmh float64) {
		if hour >= 6 && hour < 18 {
			if kmh > acc.dayGustMax {
				acc.dayGustMax = kmh
			}
		} else {
			if kmh > acc.nightGustMax {
				acc.nightGustMax = kmh
			}
		}
	})

	// Step 4: Cloud cover.
	walkHourly(resp.Properties.SkyCover.Values, func(acc *dayAccum, hour int, pct float64) {
		if hour >= 6 && hour < 18 {
			acc.dayCloudCoverSum += pct
			acc.dayCloudCoverCount++
		} else {
			acc.nightCloudCoverSum += pct
			acc.nightCloudCoverCount++
		}
	})

	dates := make([]string, 0, len(byDate))
	for d := range byDate {
		dates = append(dates, d)
	}
	sort.Strings(dates)

	daily := make([]DailyForecast, 0, len(dates))
	for _, d := range dates {
		t, err := time.Parse("2006-01-02", d)
		if err != nil {
			continue
		}
		acc := byDate[d]

		var slRatio float64
		if acc.totalSnowPrecipMM > 0 {
			slRatio = acc.weightedSLRSum / acc.totalSnowPrecipMM
		}

		daily = append(daily, DailyForecast{
			Date:                t.UTC(),
			SnowfallCM:          acc.snowCM,
			ShelteredSnowfallCM: acc.shelteredSnowCM,
			TemperatureMinC:     acc.tempMin,
			TemperatureMaxC:     acc.tempMax,
			PrecipitationMM:     acc.precipMM,
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
				CloudCoverPct:       safeDivide(acc.dayCloudCoverSum, float64(acc.dayCloudCoverCount)),
			},
			Night: HalfDay{
				SnowfallCM:          acc.nightSnowCM,
				ShelteredSnowfallCM: acc.nightShelteredSnowCM,
				TemperatureC:        acc.nightTempMin,
				PrecipitationMM:     acc.nightPrecipMM,
				WindSpeedKmh:        acc.nightWindMax,
				WindGustKmh:         acc.nightGustMax,
				CloudCoverPct:       safeDivide(acc.nightCloudCoverSum, float64(acc.nightCloudCoverCount)),
			},
		})
	}
	return daily, nil
}

func parseISO8601Interval(s string) (time.Time, time.Duration, error) {
	parts := strings.SplitN(s, "/", 2)
	if len(parts) != 2 {
		return time.Time{}, 0, fmt.Errorf("invalid interval %q: missing /", s)
	}
	t, err := time.Parse(time.RFC3339, parts[0])
	if err != nil {
		return time.Time{}, 0, fmt.Errorf("parse interval start %q: %w", parts[0], err)
	}
	dur, err := parseISO8601Duration(parts[1])
	if err != nil {
		return time.Time{}, 0, fmt.Errorf("parse interval duration %q: %w", parts[1], err)
	}
	return t, dur, nil
}

func parseISO8601Duration(s string) (time.Duration, error) {
	if !strings.HasPrefix(s, "P") {
		return 0, fmt.Errorf("duration %q must start with P", s)
	}
	s = s[1:]

	var totalDur time.Duration
	timeSection := false

	for len(s) > 0 {
		if s[0] == 'T' {
			timeSection = true
			s = s[1:]
			continue
		}
		i := 0
		for i < len(s) && (s[i] >= '0' && s[i] <= '9') {
			i++
		}
		if i == 0 || i >= len(s) {
			return 0, fmt.Errorf("unexpected token in duration %q at %q", s, s)
		}
		n, err := strconv.Atoi(s[:i])
		if err != nil {
			return 0, err
		}
		unit := s[i]
		s = s[i+1:]

		switch {
		case !timeSection && unit == 'D':
			totalDur += time.Duration(n) * 24 * time.Hour
		case !timeSection && unit == 'W':
			totalDur += time.Duration(n) * 7 * 24 * time.Hour
		case timeSection && unit == 'H':
			totalDur += time.Duration(n) * time.Hour
		case timeSection && unit == 'M':
			totalDur += time.Duration(n) * time.Minute
		case timeSection && unit == 'S':
			totalDur += time.Duration(n) * time.Second
		default:
			return 0, fmt.Errorf("unsupported duration unit %q in %q", string(unit), s)
		}
	}
	return totalDur, nil
}

func gridCacheKey(lat, lon float64) string {
	return fmt.Sprintf("%.4f,%.4f", roundTo4(lat), roundTo4(lon))
}

func roundTo4(v float64) float64 {
	return math.Round(v*10000) / 10000
}

// WFO returns the Weather Forecast Office code for the given coordinates.
func (c *NWSClient) WFO(ctx context.Context, lat, lon float64) string {
	grid, err := c.resolveGrid(ctx, lat, lon)
	if err != nil {
		return ""
	}
	return grid.WFO
}

// FetchAFD retrieves the latest Area Forecast Discussion from the NWS.
func (c *NWSClient) FetchAFD(ctx context.Context, wfo string) (ForecastDiscussion, error) {
	if wfo == "" {
		return ForecastDiscussion{}, fmt.Errorf("nws: empty WFO code")
	}

	listURL := fmt.Sprintf("%s/products/types/AFD/locations/%s", nwsBaseURL, wfo)
	listBody, statusCode, err := c.get(ctx, listURL)
	if err != nil {
		return ForecastDiscussion{}, fmt.Errorf("nws: list AFDs for %s: %w", wfo, err)
	}
	if statusCode != http.StatusOK {
		return ForecastDiscussion{}, fmt.Errorf("nws: list AFDs returned HTTP %d for %s", statusCode, wfo)
	}

	var listResp nwsProductListResponse
	if err := json.Unmarshal(listBody, &listResp); err != nil {
		return ForecastDiscussion{}, fmt.Errorf("nws: decode AFD list for %s: %w", wfo, err)
	}

	if len(listResp.Graph) == 0 {
		return ForecastDiscussion{}, fmt.Errorf("nws: no AFD products found for %s", wfo)
	}

	productID := listResp.Graph[0].ID
	productURL := fmt.Sprintf("%s/products/%s", nwsBaseURL, productID)
	productBody, statusCode, err := c.get(ctx, productURL)
	if err != nil {
		return ForecastDiscussion{}, fmt.Errorf("nws: fetch AFD %s: %w", productID, err)
	}
	if statusCode != http.StatusOK {
		return ForecastDiscussion{}, fmt.Errorf("nws: fetch AFD returned HTTP %d for %s", statusCode, productID)
	}

	var productResp nwsProductResponse
	if err := json.Unmarshal(productBody, &productResp); err != nil {
		return ForecastDiscussion{}, fmt.Errorf("nws: decode AFD %s: %w", productID, err)
	}

	issuedAt, _ := time.Parse(time.RFC3339, listResp.Graph[0].IssuanceTime)

	return ForecastDiscussion{
		WFO:       wfo,
		IssuedAt:  issuedAt,
		Text:      productResp.ProductText,
		FetchedAt: time.Now().UTC(),
	}, nil
}

// NWS API response types.

type nwsProductListResponse struct {
	Graph []nwsProductEntry `json:"@graph"`
}

type nwsProductEntry struct {
	ID           string `json:"id"`
	IssuanceTime string `json:"issuanceTime"`
}

type nwsProductResponse struct {
	ProductText string `json:"productText"`
}

type nwsPointsResponse struct {
	Properties struct {
		GridID string `json:"gridId"`
		GridX  int    `json:"gridX"`
		GridY  int    `json:"gridY"`
	} `json:"properties"`
}

type nwsGridpointResponse struct {
	Properties struct {
		SnowfallAmount            nwsTimeSeries `json:"snowfallAmount"`
		Temperature               nwsTimeSeries `json:"temperature"`
		QuantitativePrecipitation nwsTimeSeries `json:"quantitativePrecipitation"`
		WindSpeed                 nwsTimeSeries `json:"windSpeed"`
		WindGust                  nwsTimeSeries `json:"windGust"`
		SkyCover                  nwsTimeSeries `json:"skyCover"`
	} `json:"properties"`
}

type nwsTimeSeries struct {
	Values []nwsValue `json:"values"`
}

type nwsValue struct {
	ValidTime string   `json:"validTime"`
	Value     *float64 `json:"value"`
}
