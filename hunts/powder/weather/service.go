package weather

import (
	"context"
	"log/slog"
	"math"
	"net/http"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"
)

// Service fetches weather for a region from all applicable sources.
type Service struct {
	openMeteo *OpenMeteoClient
	nws       *NWSClient
}

func NewService(openMeteo *OpenMeteoClient, nws *NWSClient) *Service {
	return &Service{openMeteo: openMeteo, nws: nws}
}

// FetchAll fetches forecasts for each resort in a region from all applicable sources.
// Errors for individual resorts/sources are logged but not fatal.
func (s *Service) FetchAll(ctx context.Context, region Region, resorts []Resort) (FetchResult, error) {
	if len(resorts) == 0 {
		return FetchResult{}, nil
	}

	var (
		mu         sync.Mutex
		forecasts  []Forecast
		discussion *ForecastDiscussion
	)

	g, gctx := errgroup.WithContext(ctx)

	for _, resort := range resorts {
		resort := resort
		g.Go(func() error {
			q := openMeteoQuery{
				RegionID:   region.ID,
				ResortID:   resort.ID,
				Lat:        resort.Latitude,
				Lon:        resort.Longitude,
				ElevationM: resort.MidMountainElevationM(),
				Country:    region.Country,
				Timezone:   region.Timezone,
			}
			result, err := s.openMeteo.FetchForResort(gctx, q)
			if err != nil {
				slog.WarnContext(gctx, "open-meteo fetch failed for resort",
					"region_id", region.ID, "resort_id", resort.ID, "error", err)
				return nil
			}
			mu.Lock()
			forecasts = append(forecasts, result...)
			mu.Unlock()
			return nil
		})
	}

	if region.Country == "US" {
		for _, resort := range resorts {
			resort := resort
			g.Go(func() error {
				f, err := s.nws.Fetch(gctx, Region{
					ID:        region.ID,
					Latitude:  resort.Latitude,
					Longitude: resort.Longitude,
					Country:   region.Country,
					Timezone:  region.Timezone,
				})
				if err != nil {
					slog.WarnContext(gctx, "nws fetch failed for resort",
						"region_id", region.ID, "resort_id", resort.ID, "error", err)
					return nil
				}
				f.ResortID = resort.ID
				mu.Lock()
				forecasts = append(forecasts, f)
				mu.Unlock()
				return nil
			})
		}

		g.Go(func() error {
			wfo := s.nws.WFO(gctx, resorts[0].Latitude, resorts[0].Longitude)
			if wfo == "" {
				return nil
			}
			afd, err := s.nws.FetchAFD(gctx, wfo)
			if err != nil {
				slog.WarnContext(gctx, "nws afd fetch failed",
					"region_id", region.ID, "wfo", wfo, "error", err)
				return nil
			}
			if afd.Text != "" {
				mu.Lock()
				discussion = &afd
				mu.Unlock()
			}
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return FetchResult{}, err
	}

	if len(forecasts) == 0 {
		return FetchResult{}, nil
	}

	return FetchResult{
		Forecasts:  forecasts,
		Discussion: discussion,
	}, nil
}

const (
	retryMaxAttempts = 3
	retryBaseDelay   = 2 * time.Second
)

// retryDo executes an HTTP request with exponential backoff for transient failures.
func retryDo(ctx context.Context, client *http.Client, req *http.Request) (*http.Response, error) {
	var lastErr error
	for attempt := range retryMaxAttempts {
		if attempt > 0 {
			delay := time.Duration(math.Pow(2, float64(attempt-1))) * retryBaseDelay
			slog.InfoContext(ctx, "retrying request",
				"url", req.URL.String(), "attempt", attempt+1, "delay", delay)
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(delay):
			}
		}

		resp, err := client.Do(req)
		if err != nil {
			lastErr = err
			continue
		}

		if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
			resp.Body.Close()
			lastErr = &retryableStatusError{StatusCode: resp.StatusCode, URL: req.URL.String()}
			continue
		}

		return resp, nil
	}
	return nil, lastErr
}

type retryableStatusError struct {
	StatusCode int
	URL        string
}

func (e *retryableStatusError) Error() string {
	return "retryable HTTP " + http.StatusText(e.StatusCode) + " from " + e.URL
}
