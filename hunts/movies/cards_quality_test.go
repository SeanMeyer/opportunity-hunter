package movies

import (
	"github.com/seanmeyer/opportunity-hunter/core"
	"testing"
)

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
