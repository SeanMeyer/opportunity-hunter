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

// Ticketmaster fetches comedy events from the Ticketmaster Discovery API.
type Ticketmaster struct {
	apiKey     string
	httpClient *http.Client
}

func NewTicketmaster(apiKey string, httpClient *http.Client) *Ticketmaster {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}
	return &Ticketmaster{apiKey: apiKey, httpClient: httpClient}
}

func (t *Ticketmaster) Name() string { return "ticketmaster-comedy" }

func (t *Ticketmaster) Scan(ctx context.Context, region core.ScanRegion) ([]core.RawItem, error) {
	var allItems []core.RawItem
	page := 0

	for {
		params := url.Values{
			"apikey":              {t.apiKey},
			"classificationName": {"Comedy"},
			"latlong":            {fmt.Sprintf("%f,%f", region.Latitude, region.Longitude)},
			"radius":             {strconv.Itoa(region.RadiusMi)},
			"unit":               {"miles"},
			"size":               {"100"},
			"page":               {strconv.Itoa(page)},
			"sort":               {"date,asc"},
		}

		reqURL := "https://app.ticketmaster.com/discovery/v2/events.json?" + params.Encode()
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
		if err != nil {
			return nil, fmt.Errorf("create request: %w", err)
		}

		resp, err := t.httpClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("ticketmaster request: %w", err)
		}
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("read response: %w", err)
		}
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("ticketmaster %d: %s", resp.StatusCode, string(body))
		}

		var result discoveryResponse
		if err := json.Unmarshal(body, &result); err != nil {
			return nil, fmt.Errorf("parse response: %w", err)
		}

		for _, event := range result.Embedded.Events {
			item := eventToRawItem(event)
			if item != nil {
				allItems = append(allItems, *item)
			}
		}

		if result.Page.Number >= result.Page.TotalPages-1 {
			break
		}
		page++
	}

	return allItems, nil
}

func eventToRawItem(e event) *core.RawItem {
	if e.Dates.Start.DateTime == "" && e.Dates.Start.LocalDate == "" {
		return nil
	}

	item := &core.RawItem{
		SourceID:  e.ID,
		Source:    "ticketmaster",
		Title:     e.Name,
		StartTime: resolveShowTime(e),
		TicketURL: e.URL,
	}

	if len(e.Embedded.Venues) > 0 {
		v := e.Embedded.Venues[0]
		item.VenueName = v.Name
		item.VenueAddress = v.Address.Line1 + ", " + v.City.Name + ", " + v.State.StateCode
		if lat, err := strconv.ParseFloat(v.Location.Latitude, 64); err == nil {
			item.VenueLatitude = lat
		}
		if lon, err := strconv.ParseFloat(v.Location.Longitude, 64); err == nil {
			item.VenueLongitude = lon
		}
		item.Subtitle = v.Name
	}

	if len(e.PriceRanges) > 0 {
		min := e.PriceRanges[0].Min
		max := e.PriceRanges[0].Max
		item.PriceMin = &min
		item.PriceMax = &max
	}

	eventJSON, _ := json.Marshal(e)
	item.RawJSON = string(eventJSON)

	return item
}

func resolveShowTime(e event) string {
	if e.Dates.Start.LocalDate != "" && e.Dates.Start.LocalTime != "" && e.Dates.Timezone != "" {
		loc, err := time.LoadLocation(e.Dates.Timezone)
		if err == nil {
			localStr := e.Dates.Start.LocalDate + "T" + e.Dates.Start.LocalTime
			t, err := time.ParseInLocation("2006-01-02T15:04:05", localStr, loc)
			if err == nil {
				return t.Format(time.RFC3339)
			}
		}
	}
	return e.Dates.Start.DateTime
}

type discoveryResponse struct {
	Embedded struct {
		Events []event `json:"events"`
	} `json:"_embedded"`
	Page struct {
		Number     int `json:"number"`
		TotalPages int `json:"totalPages"`
	} `json:"page"`
}

type event struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	URL   string `json:"url"`
	Dates struct {
		Start struct {
			DateTime  string `json:"dateTime"`
			LocalDate string `json:"localDate"`
			LocalTime string `json:"localTime"`
		} `json:"start"`
		Timezone string `json:"timezone"`
	} `json:"dates"`
	Embedded struct {
		Venues []struct {
			Name    string `json:"name"`
			Address struct {
				Line1 string `json:"line1"`
			} `json:"address"`
			City struct {
				Name string `json:"name"`
			} `json:"city"`
			State struct {
				StateCode string `json:"stateCode"`
			} `json:"state"`
			Location struct {
				Latitude  string `json:"latitude"`
				Longitude string `json:"longitude"`
			} `json:"location"`
		} `json:"venues"`
	} `json:"_embedded"`
	PriceRanges []struct {
		Min float64 `json:"min"`
		Max float64 `json:"max"`
	} `json:"priceRanges"`
}
