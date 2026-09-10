package movies

import (
	"github.com/seanmeyer/opportunity-hunter/core"
	"strings"
	"testing"
	"time"
)

func TestMovieReleaseAndDetailsLink(t *testing.T) {
	c := (&moviesCardRenderer{}).RenderCard(core.Opportunity{Source: "tmdb", SourceID: "tmdb-123", StartTime: time.Now().AddDate(0, 0, -2)}, core.Pick{}, core.Venue{})
	if !strings.HasPrefix(c.DateDisplay, "Released ") || c.ActionURL != "https://www.themoviedb.org/movie/123" || c.ActionLabel != "View movie" {
		t.Fatalf("unexpected card: %+v", c)
	}
}

func TestCardShowsKnownTicketPriceWithoutBlankRelease(t *testing.T) {
	price := 35.0
	c := (&moviesCardRenderer{}).RenderCard(core.Opportunity{PriceMin: &price, Attributes: []byte("{}")}, core.Pick{}, core.Venue{})
	found := false
	for _, f := range c.Fields {
		if f.Label == "Price" && f.Value == "$35" {
			found = true
		}
		if f.Value == "" {
			t.Errorf("blank field %s", f.Label)
		}
	}
	if !found {
		t.Fatal("known listing ticket price missing")
	}
}
