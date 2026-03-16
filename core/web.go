package core

// ScoreTier maps to CSS classes in the web UI.
type ScoreTier string

const (
	ScoreHigh   ScoreTier = "high"
	ScoreMedium ScoreTier = "medium"
	ScoreLow    ScoreTier = "low"
	ScoreNone   ScoreTier = "none"
)

// Sort key constants. Hunts use these in SortOption.Value to reference
// built-in sort behaviors in the web layer.
const (
	SortByScore  = "score"
	SortByDate   = "date"
	SortByTier   = "tier"
	SortByRegion = "region"
)

// SortOption defines a sort choice for the web UI toolbar.
type SortOption struct {
	Value string // one of the SortBy* constants
	Label string
}

// FilterOption defines a filter choice for the web UI toolbar.
type FilterOption struct {
	Value string
	Label string
}

// WebConfig holds toolbar configuration for a hunt's web UI.
type WebConfig struct {
	SortOptions   []SortOption
	FilterOptions []FilterOption
	DefaultSort   string // must appear in SortOptions
}

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

	// SortScore is a normalized [0,1] score for sorting. Higher = better.
	// Comedy/performing/movies: pick.Score (already 0-1).
	// Powder: normalized from snowfall inches (e.g. clamp(inches/30, 0, 1)).
	SortScore float64

	// DateSort is a unix timestamp for date-based sorting. 0 = no date.
	DateSort int64

	// DateDisplay is the formatted date string for summary views.
	// Set by card renderers alongside DateSort to avoid re-parsing from Fields.
	DateDisplay string
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
