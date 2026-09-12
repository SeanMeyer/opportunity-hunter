package web

import (
	"github.com/seanmeyer/opportunity-hunter/core"
	"sort"
	"strings"
)

func venueOptions(cards []core.CardData, selected string) []core.FilterOption {
	names := map[string]string{}
	addresses := map[string]string{}
	for _, c := range cards {
		if c.VenueKey != "" && c.VenueName != "" && (names[c.VenueKey] == "" || c.VenueName < names[c.VenueKey]) {
			names[c.VenueKey] = c.VenueName
			addresses[c.VenueKey] = c.VenueAddress
		}
	}
	labelCounts := map[string]int{}
	for _, name := range names {
		labelCounts[strings.ToLower(name)]++
	}
	var options []core.FilterOption
	for key, name := range names {
		if labelCounts[strings.ToLower(name)] > 1 {
			address := addresses[key]
			if address == "" {
				address = "address unavailable"
			}
			name += " · " + address
		}
		options = append(options, core.FilterOption{Value: key, Label: name})
	}
	sort.Slice(options, func(i, j int) bool { return options[i].Label < options[j].Label })
	if selected != "" && names[selected] == "" {
		options = append(options, core.FilterOption{Value: selected, Label: "Selected venue (no current events)"})
	}
	return options
}

func filterByVenue(cards []core.CardData, selected string) []core.CardData {
	if selected == "" {
		return cards
	}
	var out []core.CardData
	for _, c := range cards {
		if c.VenueKey == selected {
			out = append(out, c)
		}
	}
	return out
}
