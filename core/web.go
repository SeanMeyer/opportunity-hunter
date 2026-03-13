package core

// ScoreTier maps to CSS classes in the web UI.
type ScoreTier string

const (
	ScoreHigh   ScoreTier = "high"
	ScoreMedium ScoreTier = "medium"
	ScoreLow    ScoreTier = "low"
	ScoreNone   ScoreTier = "none"
)

// CardData is the universal display format for the web UI.
type CardData struct {
	OpportunityID int64
	Title         string
	Subtitle      string
	Score         string
	ScoreTier     ScoreTier
	Reason        string
	Urgency       string
	Fields        []CardField
	ActionURL     string
	ActionLabel   string
	SnowfallIn    float64 // for sorting; 0 if not applicable
}

// CardField is a hunt-specific detail row on a card.
type CardField struct {
	Icon  string
	Label string
	Value string
}

// CardRenderer converts picks into display cards for the web UI.
type CardRenderer interface {
	RenderCard(opp Opportunity, pick Pick, venue Venue) CardData
}

// FeedbackOption defines a feedback button in the web UI.
type FeedbackOption struct {
	Value string
	Label string
}
