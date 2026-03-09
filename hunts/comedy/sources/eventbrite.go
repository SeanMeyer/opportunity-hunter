package sources

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/seanmeyer/opportunity-hunter/core"
)

// Eventbrite fetches comedy events from the Eventbrite API.
type Eventbrite struct {
	token      string
	httpClient *http.Client
}

func NewEventbrite(token string, httpClient *http.Client) *Eventbrite {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}
	return &Eventbrite{token: token, httpClient: httpClient}
}

func (e *Eventbrite) Name() string { return "eventbrite" }

func (e *Eventbrite) Scan(ctx context.Context, region core.ScanRegion) ([]core.RawItem, error) {
	params := url.Values{
		"location.latitude":  {fmt.Sprintf("%f", region.Latitude)},
		"location.longitude": {fmt.Sprintf("%f", region.Longitude)},
		"location.within":    {fmt.Sprintf("%dmi", region.RadiusMi)},
		"q":                  {"comedy"},
		"expand":             {"venue"},
	}

	reqURL := "https://www.eventbriteapi.com/v3/events/search/?" + params.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+e.token)

	resp, err := e.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("eventbrite request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("eventbrite %d: %s", resp.StatusCode, string(body))
	}

	var result ebSearchResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	var items []core.RawItem
	for _, ev := range result.Events {
		item := ebEventToRawItem(ev)
		if item != nil {
			items = append(items, *item)
		}
	}
	return items, nil
}

func ebEventToRawItem(e ebEvent) *core.RawItem {
	if e.Start.UTC == "" {
		return nil
	}

	item := &core.RawItem{
		SourceID:  e.ID,
		Source:    "eventbrite",
		Title:     e.Name.Text,
		StartTime: e.Start.UTC,
		TicketURL: e.URL,
	}

	if e.Venue != nil {
		item.VenueName = e.Venue.Name
		item.VenueAddress = e.Venue.Address.LocalizedAddress
		if lat, err := strconv.ParseFloat(e.Venue.Latitude, 64); err == nil {
			item.VenueLatitude = lat
		}
		if lon, err := strconv.ParseFloat(e.Venue.Longitude, 64); err == nil {
			item.VenueLongitude = lon
		}
		item.Subtitle = e.Venue.Name
	}

	eventJSON, _ := json.Marshal(e)
	item.RawJSON = string(eventJSON)

	return item
}

type ebSearchResponse struct {
	Events []ebEvent `json:"events"`
}

type ebEvent struct {
	ID    string `json:"id"`
	Name  struct {
		Text string `json:"text"`
	} `json:"name"`
	URL   string `json:"url"`
	Start struct {
		UTC string `json:"utc"`
	} `json:"start"`
	Venue *ebVenue `json:"venue"`
}

type ebVenue struct {
	Name      string `json:"name"`
	Latitude  string `json:"latitude"`
	Longitude string `json:"longitude"`
	Address   struct {
		LocalizedAddress string `json:"localized_address_display"`
	} `json:"address"`
}
