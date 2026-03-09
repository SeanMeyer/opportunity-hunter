package performing

import (
	"encoding/json"

	"github.com/seanmeyer/opportunity-hunter/core"
)

// PerformingAttrs holds performing-arts-specific attributes.
type PerformingAttrs struct {
	Genre       string   `json:"genre"`        // ballet, opera, theater, musical
	ReviewScore *float64 `json:"review_score"`  // aggregated if available
	Awards      []string `json:"awards"`
}

// Encode serializes to core.Attributes.
func (a PerformingAttrs) Encode() core.Attributes {
	b, _ := json.Marshal(a)
	return b
}

// DecodePerformingAttrs deserializes from core.Attributes.
func DecodePerformingAttrs(raw core.Attributes) (PerformingAttrs, error) {
	var a PerformingAttrs
	return a, json.Unmarshal(raw, &a)
}
