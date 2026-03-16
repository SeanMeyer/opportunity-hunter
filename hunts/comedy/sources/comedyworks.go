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
	downtownURL  = "https://comedyworks.com/events?downtown=1"
	downtownName = "Comedy Works Downtown"
	downtownAddr = "1226 15th St, Denver, CO 80202"
	downtownLat  = 39.7475
	downtownLon  = -104.9994

	southURL  = "https://comedyworks.com/events?south=1"
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

	type pageInfo struct {
		url       string
		venueName string
		venueAddr string
		venueLat  float64
		venueLon  float64
	}

	pages := []pageInfo{
		{downtownURL, downtownName, downtownAddr, downtownLat, downtownLon},
		{southURL, southName, southAddr, southLat, southLon},
	}

	for _, page := range pages {
		c := colly.NewCollector(
			colly.AllowedDomains("comedyworks.com", "www.comedyworks.com"),
		)

		var scrapeErr error

		c.OnHTML("li", func(e *colly.HTMLElement) {
			name := strings.TrimSpace(e.ChildText("h3 a"))
			if name == "" {
				return
			}

			// Extract date and ticket link from <a> elements.
			var showTime time.Time
			var ticketLink string
			e.ForEach("a", func(_ int, a *colly.HTMLElement) {
				text := strings.TrimSpace(a.Text)
				href := a.Attr("href")
				if strings.EqualFold(text, "Buy Tickets") {
					if href != "" && !strings.HasPrefix(href, "http") {
						href = "https://comedyworks.com" + href
					}
					ticketLink = href
				} else if text != name && showTime.IsZero() {
					if t := parseFlexibleDate(text); !t.IsZero() {
						showTime = t
					}
				}
			})

			if showTime.IsZero() {
				return
			}

			// Fall back to detail page link if no Buy Tickets link.
			if ticketLink == "" {
				if href := e.ChildAttr("h3 a", "href"); href != "" {
					if !strings.HasPrefix(href, "http") {
						href = "https://comedyworks.com" + href
					}
					ticketLink = href
				}
			}

			raw, _ := json.Marshal(map[string]string{
				"name": name, "date": showTime.Format("Jan 2, 2006"), "url": ticketLink, "venue": page.venueName,
			})

			items = append(items, core.RawItem{
				SourceID:       fmt.Sprintf("cw-%s-%s", slugify(name), showTime.Format("2006-01-02")),
				Source:         "comedyworks",
				Title:          name,
				Subtitle:       page.venueName,
				VenueName:      page.venueName,
				VenueAddress:   page.venueAddr,
				VenueLatitude:  page.venueLat,
				VenueLongitude: page.venueLon,
				StartTime:      showTime.Format(time.RFC3339),
				TicketURL:      ticketLink,
				RawJSON:        string(raw),
			})
		})

		c.OnError(func(_ *colly.Response, err error) {
			scrapeErr = fmt.Errorf("scrape comedyworks: %w", err)
		})

		if err := c.Visit(page.url); err != nil {
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

