package distance

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

// Result holds a distance calculation result.
type Result struct {
	Minutes    int
	DistanceMi float64
}

// Client queries the Google Routes API for distance calculations.
type Client struct {
	apiKey  string
	http    *http.Client
	baseURL string
}

// NewClient creates a distance client. The apiKey is the Google Maps API key.
func NewClient(apiKey string, httpClient *http.Client) *Client {
	return &Client{
		apiKey:  apiKey,
		http:    httpClient,
		baseURL: "https://routes.googleapis.com",
	}
}

// GetDistance calculates the distance between origin and destination using the given mode.
// Mode should be "WALK" or "DRIVE".
func (c *Client) GetDistance(ctx context.Context, origin, destination, mode string) (Result, error) {
	body := routeMatrixRequest{
		Origins: []routeMatrixWaypoint{{
			Waypoint: waypoint{Address: origin},
		}},
		Destinations: []routeMatrixWaypoint{{
			Waypoint: waypoint{Address: destination},
		}},
		TravelMode: mode,
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return Result{}, fmt.Errorf("marshal request: %w", err)
	}

	url := c.baseURL + "/distanceMatrix/v2:computeRouteMatrix"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(jsonBody))
	if err != nil {
		return Result{}, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Goog-Api-Key", c.apiKey)
	req.Header.Set("X-Goog-FieldMask", "originIndex,destinationIndex,distanceMeters,duration,status,condition")

	resp, err := c.http.Do(req)
	if err != nil {
		return Result{}, fmt.Errorf("routes api request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errResp struct {
			Error struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		json.NewDecoder(resp.Body).Decode(&errResp)
		return Result{}, fmt.Errorf("routes api %d: %s", resp.StatusCode, errResp.Error.Message)
	}

	var results []routeMatrixElement
	if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
		return Result{}, fmt.Errorf("decode response: %w", err)
	}

	if len(results) == 0 {
		return Result{}, fmt.Errorf("empty response")
	}

	elem := results[0]
	if elem.Condition != "ROUTE_EXISTS" {
		return Result{}, fmt.Errorf("no route: condition=%s", elem.Condition)
	}

	minutes := parseDurationSeconds(elem.Duration) / 60
	miles := float64(elem.DistanceMeters) / 1609.344

	return Result{
		Minutes:    minutes,
		DistanceMi: miles,
	}, nil
}

func parseDurationSeconds(s string) int {
	s = strings.TrimSuffix(s, "s")
	n, _ := strconv.Atoi(s)
	return n
}

type routeMatrixRequest struct {
	Origins      []routeMatrixWaypoint `json:"origins"`
	Destinations []routeMatrixWaypoint `json:"destinations"`
	TravelMode   string                `json:"travelMode"`
}

type routeMatrixWaypoint struct {
	Waypoint waypoint `json:"waypoint"`
}

type waypoint struct {
	Address string `json:"address"`
}

type routeMatrixElement struct {
	OriginIndex      int    `json:"originIndex"`
	DestinationIndex int    `json:"destinationIndex"`
	Status           any    `json:"status"`
	Condition        string `json:"condition"`
	DistanceMeters   int    `json:"distanceMeters"`
	Duration         string `json:"duration"`
}
