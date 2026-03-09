package movies

import (
	"encoding/json"

	"github.com/seanmeyer/opportunity-hunter/core"
)

// MovieAttrs holds movie-specific attributes.
type MovieAttrs struct {
	Genre       []string `json:"genre"`
	Director    string   `json:"director"`
	Cast        []string `json:"cast"`
	TMDBRating  float64  `json:"tmdb_rating"`
	ReleaseType string   `json:"release_type"` // "theatrical", "streaming", "screening"
	Service     string   `json:"service"`      // streaming service name, empty for theatrical
}

func (a MovieAttrs) Encode() core.Attributes {
	b, _ := json.Marshal(a)
	return b
}

func DecodeMovieAttrs(raw core.Attributes) (MovieAttrs, error) {
	var a MovieAttrs
	return a, json.Unmarshal(raw, &a)
}
