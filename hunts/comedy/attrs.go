package comedy

import (
	"encoding/json"

	"github.com/seanmeyer/opportunity-hunter/core"
)

// ComedyAttrs holds comedy-specific attributes.
type ComedyAttrs struct {
	SellOutRisk string `json:"sell_out_risk"` // low, medium, high
}

func (a ComedyAttrs) Encode() core.Attributes {
	b, _ := json.Marshal(a)
	return b
}

func DecodeComedyAttrs(raw core.Attributes) (ComedyAttrs, error) {
	var a ComedyAttrs
	return a, json.Unmarshal(raw, &a)
}
