package web

import (
	"github.com/seanmeyer/opportunity-hunter/core"
	"testing"
)

func TestVerdictFiltersAndSortUseDisplayLabels(t *testing.T) {
	cards := []core.CardData{{Score: "Skip"}, {Score: "Watch"}, {Score: "Recommended"}, {Score: "Drop Everything"}}
	for _, value := range []string{"SKIP", "WATCH", "RECOMMENDED", "DROP_EVERYTHING", "WORTH_A_LOOK", "ON_THE_RADAR"} {
		if got := filterCards(cards, value); len(got) != 1 {
			t.Errorf("filter %s returned %d", value, len(got))
		}
	}
	sortCards(cards, core.SortByTier)
	for i, want := range []string{"Drop Everything", "Recommended", "Watch", "Skip"} {
		if cards[i].Score != want {
			t.Fatalf("bad tier ordering: %+v", cards)
		}
	}
}
