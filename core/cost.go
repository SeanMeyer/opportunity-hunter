package core

// CostTracker tracks LLM spending across hunts.
// Fields are unexported; access through methods only.
// No mutex needed since hunts run sequentially.
type CostTracker struct {
	total  float64
	byHunt map[string]float64
}

// NewCostTracker creates a CostTracker seeded with initial values.
func NewCostTracker(total float64, byHunt map[string]float64) *CostTracker {
	if byHunt == nil {
		byHunt = make(map[string]float64)
	}
	return &CostTracker{total: total, byHunt: byHunt}
}

// Add records a cost for a hunt.
func (ct *CostTracker) Add(hunt string, cost float64) {
	ct.total += cost
	ct.byHunt[hunt] += cost
}

// Total returns the total spend across all hunts.
func (ct *CostTracker) Total() float64 {
	return ct.total
}

// ForHunt returns the total spend for a specific hunt.
func (ct *CostTracker) ForHunt(hunt string) float64 {
	return ct.byHunt[hunt]
}
