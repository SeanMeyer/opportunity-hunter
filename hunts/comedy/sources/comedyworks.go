package sources

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
	"unicode"

	"github.com/gocolly/colly/v2"

	"github.com/seanmeyer/opportunity-hunter/core"
)

const (
	downtownURL  = "https://comedyworks.com/comedians"
	downtownName = "Comedy Works Downtown"
	downtownAddr = "1226 15th St, Denver, CO 80202"
	downtownLat  = 39.7475
	downtownLon  = -104.9994

	southURL  = "https://comedyworks.com/comedians?location=south"
	southName = "Comedy Works South"
	southAddr = "5345 Landmark Pl, Greenwood Village, CO 80111"
	southLat  = 39.5966
	southLon  = -104.8988
)

// ComedyWorks scrapes the Comedy Works Denver website.
type ComedyWorks struct{}

func NewComedyWorks() *ComedyWorks { return &ComedyWorks{} }

func (s *ComedyWorks) Name() string { return "comedyworks" }

func (s *ComedyWorks) Scan(_ context.Context, _ core.ScanRegion) ([]core.RawItem, error) {
	var items []core.RawItem
	var scrapeErr error

	c := colly.NewCollector(
		colly.AllowedDomains("comedyworks.com", "www.comedyworks.com"),
	)

	c.OnHTML("li.comedian-box", func(e *colly.HTMLElement) {
		name := strings.TrimSpace(e.ChildText("h2.comedian-box-title"))
		dateStr := strings.TrimSpace(e.ChildText("p.comedian-box-date"))
		location := strings.TrimSpace(e.ChildText("p.comedian-box-location"))
		ticketLink := e.ChildAttr("a.buy-tickets", "href")

		if name == "" || dateStr == "" {
			return
		}

		showTime := parseFlexibleDate(dateStr)
		if showTime.IsZero() {
			return
		}

		if ticketLink != "" && !strings.HasPrefix(ticketLink, "http") {
			ticketLink = "https://comedyworks.com" + ticketLink
		}

		venueName, venueAddr, venueLat, venueLon := resolveVenue(location)

		raw, _ := json.Marshal(map[string]string{
			"name": name, "date": dateStr, "url": ticketLink, "location": location,
		})

		items = append(items, core.RawItem{
			SourceID:       fmt.Sprintf("cw-%s-%s", slugify(name), showTime.Format("2006-01-02")),
			Source:         "comedyworks",
			Title:          name,
			Subtitle:       venueName,
			VenueName:      venueName,
			VenueAddress:   venueAddr,
			VenueLatitude:  venueLat,
			VenueLongitude: venueLon,
			StartTime:      showTime.Format(time.RFC3339),
			TicketURL:      ticketLink,
			RawJSON:        string(raw),
		})
	})

	c.OnError(func(_ *colly.Response, err error) {
		scrapeErr = fmt.Errorf("scrape comedyworks: %w", err)
	})

	for _, url := range []string{downtownURL, southURL} {
		if err := c.Visit(url); err != nil {
			return nil, fmt.Errorf("visit comedyworks: %w", err)
		}
		if scrapeErr != nil {
			return nil, scrapeErr
		}
	}

	return items, nil
}

func parseFlexibleDate(s string) time.Time {
	loc, _ := time.LoadLocation("America/Denver")
	formats := []string{
		time.RFC3339,
		"2006-01-02T15:04:05",
		"January 2, 2006",
		"Jan 2, 2006",
		"Mon Jan 2, 2006",
		"1/2/2006",
		"2006-01-02",
	}
	for _, f := range formats {
		if t, err := time.ParseInLocation(f, strings.TrimSpace(s), loc); err == nil {
			return t
		}
	}
	return time.Time{}
}

func slugify(s string) string {
	s = strings.ToLower(s)
	s = strings.ReplaceAll(s, " ", "-")
	return strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			return r
		}
		if unicode.IsSpace(r) {
			return '-'
		}
		return -1
	}, s)
}

func resolveVenue(location string) (name, addr string, lat, lon float64) {
	loc := strings.ToLower(location)
	if strings.Contains(loc, "south") || strings.Contains(loc, "landmark") || strings.Contains(loc, "greenwood") {
		return southName, southAddr, southLat, southLon
	}
	return downtownName, downtownAddr, downtownLat, downtownLon
}
