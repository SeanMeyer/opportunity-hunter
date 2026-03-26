package web

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"sort"
	"strconv"
	"time"

	"github.com/seanmeyer/opportunity-hunter/core"
	"github.com/seanmeyer/opportunity-hunter/storage"
)

//go:embed templates/*.html
var templateFS embed.FS

// HuntInfo holds runtime info about a registered hunt for the web UI.
type HuntInfo struct {
	Name            string
	CardRenderer    core.CardRenderer
	FeedbackOptions []core.FeedbackOption
	WebConfig       core.WebConfig
}

// StatusInfo holds pipeline run status for display.
type StatusInfo struct {
	LastRun   time.Time
	Summary   string
	HasErrors bool
}

// Server is the web UI server.
type Server struct {
	db          *storage.DB
	tmpl        *template.Template
	hunts       []HuntInfo
	huntNames   []string
	lastStatus  *StatusInfo
	homeAddress string
	runFunc     func(context.Context, string)
}

// New creates a web server. homeAddress is used for distance enrichment (empty = skip).
// An optional runFunc callback may be provided; it is called (in a goroutine) when the
// user triggers a manual run via POST /run.
func New(db *storage.DB, hunts []HuntInfo, homeAddress string, runFunc ...func(context.Context, string)) (*Server, error) {
	tmpl, err := template.New("").ParseFS(templateFS, "templates/*.html")
	if err != nil {
		return nil, fmt.Errorf("parse templates: %w", err)
	}

	names := make([]string, len(hunts))
	for i, h := range hunts {
		names[i] = h.Name
	}

	s := &Server{db: db, tmpl: tmpl, hunts: hunts, huntNames: names, homeAddress: homeAddress}
	if len(runFunc) > 0 {
		s.runFunc = runFunc[0]
	}
	return s, nil
}

// SetStatus updates the last pipeline run status.
func (s *Server) SetStatus(status *StatusInfo) {
	s.lastStatus = status
}

// FormatDistance returns a human-friendly distance string.
// Shows walking for close venues (<=30 min), driving otherwise.
func FormatDistance(walkingMinutes, drivingMinutes int) string {
	if walkingMinutes <= 30 {
		return fmt.Sprintf("%d min walk", walkingMinutes)
	}
	return fmt.Sprintf("%d min drive", drivingMinutes)
}

// validHunt reports whether name is a registered hunt.
func (s *Server) validHunt(name string) bool {
	for _, h := range s.huntNames {
		if h == name {
			return true
		}
	}
	return false
}

// handleRun handles POST /run — triggers a manual pipeline run for a single hunt.
func (s *Server) handleRun(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	huntName := r.FormValue("hunt")
	if !s.validHunt(huntName) {
		http.Error(w, "unknown hunt", http.StatusBadRequest)
		return
	}
	if s.runFunc != nil {
		go s.runFunc(context.Background(), huntName)
	}
	http.Redirect(w, r, "/?hunt="+huntName, http.StatusSeeOther)
}

// Handler returns the HTTP handler for the web UI.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /", s.handleIndex)
	mux.HandleFunc("POST /preferences", s.handleSavePreferences)
	mux.HandleFunc("POST /feedback", s.handleSaveFeedback)
	mux.HandleFunc("POST /schedule", s.handleSaveSchedule)
	mux.HandleFunc("POST /run", s.handleRun)
	return mux
}

// cardItem wraps a card with its opportunity ID for feedback forms.
type cardItem struct {
	Card  core.CardData
	OppID int64
}

// ScheduleInfo holds schedule data for template rendering.
type ScheduleInfo struct {
	IntervalMinutes int
	NextScan        string // formatted for display
	StartHour       int
	StartMinute     int
	StartTime       string // "HH:MM" for the time input
	StartDay        int    // 0=Sunday..6=Saturday, -1=N/A
}

type pageData struct {
	Hunts           []string
	ActiveHunt      string
	Cards           []cardItem
	TotalCards      int // before filtering — used to keep toolbar visible
	Preferences     string
	FeedbackOptions []core.FeedbackOption
	Status          *StatusInfo
	SortBy          string
	FilterValue     string
	SortOptions     []core.SortOption
	FilterOptions   []core.FilterOption
	Schedule        *ScheduleInfo
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	activeHunt := r.URL.Query().Get("hunt")
	if activeHunt == "" && len(s.huntNames) > 0 {
		activeHunt = s.huntNames[0]
	}

	// Find hunt info for active hunt.
	var huntInfo *HuntInfo
	for i, h := range s.hunts {
		if h.Name == activeHunt {
			huntInfo = &s.hunts[i]
			break
		}
	}

	var webCfg core.WebConfig
	if huntInfo != nil {
		webCfg = huntInfo.WebConfig
	}

	sortBy := r.URL.Query().Get("sort")
	if sortBy == "" && webCfg.DefaultSort != "" {
		sortBy = webCfg.DefaultSort
	} else if sortBy == "" {
		sortBy = core.SortByScore
	}
	filterValue := r.URL.Query().Get("filter")

	ctx := r.Context()
	data := pageData{
		Hunts:       s.huntNames,
		ActiveHunt:  activeHunt,
		Status:      s.lastStatus,
		SortBy:      sortBy,
		FilterValue: filterValue,
	}

	if huntInfo != nil {
		data.SortOptions = webCfg.SortOptions
		data.FilterOptions = webCfg.FilterOptions
		data.FeedbackOptions = huntInfo.FeedbackOptions
	}

	// Load preferences.
	prefs, _ := s.db.GetPreferences(ctx, activeHunt)
	data.Preferences = prefs

	// Load schedule.
	if sched, err := s.db.GetSchedule(ctx, activeHunt); err == nil {
		data.Schedule = &ScheduleInfo{
			IntervalMinutes: sched.ScanIntervalM,
			NextScan:        sched.NextScanAt.Local().Format("Mon Jan 2, 3:04 PM"),
			StartHour:       sched.StartHour,
			StartMinute:     sched.StartMinute,
			StartTime:       fmt.Sprintf("%02d:%02d", sched.StartHour, sched.StartMinute),
			StartDay:        sched.StartDay,
		}
	}

	// Load cards.
	rawCards := s.loadCards(ctx, activeHunt, huntInfo)
	data.TotalCards = len(rawCards)

	// Filter cards.
	rawCards = filterCards(rawCards, filterValue)

	// Sort cards.
	sortCards(rawCards, sortBy)

	// Wrap with opportunity IDs for feedback forms.
	items := make([]cardItem, len(rawCards))
	for i, c := range rawCards {
		items[i] = cardItem{Card: c, OppID: c.OpportunityID}
	}
	data.Cards = items

	if err := s.tmpl.ExecuteTemplate(w, "layout.html", data); err != nil {
		slog.Error("render template", "err", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

func sortCards(cards []core.CardData, sortBy string) {
	switch sortBy {
	case core.SortByDate:
		sort.Slice(cards, func(i, j int) bool {
			return cards[i].DateSort < cards[j].DateSort
		})
	case core.SortByTier:
		sort.Slice(cards, func(i, j int) bool {
			return tierSortOrder(cards[i].Score) < tierSortOrder(cards[j].Score)
		})
	case core.SortByRegion:
		sort.Slice(cards, func(i, j int) bool {
			return cards[i].Title < cards[j].Title
		})
	default: // SortByScore and any unknown value
		sort.Slice(cards, func(i, j int) bool {
			return cards[i].SortScore > cards[j].SortScore
		})
	}
}

func tierSortOrder(score string) int {
	switch score {
	case "DROP_EVERYTHING":
		return 0
	case "WORTH_A_LOOK":
		return 1
	case "ON_THE_RADAR":
		return 2
	default:
		return 3
	}
}

func filterCards(cards []core.CardData, filterValue string) []core.CardData {
	if filterValue == "" {
		return cards
	}

	// Try numeric threshold filter (e.g. "8" means SortScore >= 0.8).
	// SortScore is normalized to [0,1], filter values are on a 1-10 scale.
	threshold, err := strconv.ParseFloat(filterValue, 64)
	if err == nil {
		normalized := threshold / 10.0
		var filtered []core.CardData
		for _, c := range cards {
			if c.SortScore >= normalized {
				filtered = append(filtered, c)
			}
		}
		return filtered
	}

	// Exact match filter (for tier-based hunts like powder).
	var filtered []core.CardData
	for _, c := range cards {
		if c.Score == filterValue {
			filtered = append(filtered, c)
		}
	}
	return filtered
}

func (s *Server) loadCards(ctx context.Context, huntName string, info *HuntInfo) []core.CardData {
	var cards []core.CardData
	for _, state := range []core.State{core.Notified, core.Evaluated, core.Reminded} {
		opps, err := s.db.GetByState(ctx, huntName, state)
		if err != nil {
			slog.Error("load opportunities", "hunt", huntName, "state", state, "err", err)
			continue
		}
		for _, opp := range opps {
			picks, err := s.db.GetPicksForOpportunity(ctx, opp.ID)
			if err != nil || len(picks) == 0 {
				continue
			}
			pick := picks[0]

			var venue core.Venue
			if opp.VenueID != nil {
				venue, _ = s.db.GetVenue(ctx, *opp.VenueID)
				if venue.ID != 0 {
					if dist, err := s.db.GetDistance(ctx, venue.ID, s.homeAddress, "walking"); err == nil {
						venue.WalkingMinutes = dist.Minutes
						venue.DistanceMi = dist.DistanceMi
					}
					if dist, err := s.db.GetDistance(ctx, venue.ID, s.homeAddress, "driving"); err == nil {
						venue.DrivingMinutes = dist.Minutes
					}
				}
			}

			var card core.CardData
			if info != nil && info.CardRenderer != nil {
				card = info.CardRenderer.RenderCard(opp, pick, venue)
			} else {
				card = core.CardData{
					Title:    opp.Title,
					Subtitle: opp.Subtitle,
					Score:    pick.DisplayScore,
					Reason:   pick.Reason,
					Urgency:  pick.Urgency,
				}
			}
			card.OpportunityID = opp.ID
			cards = append(cards, card)
		}
	}
	return cards
}

func (s *Server) handleSavePreferences(w http.ResponseWriter, r *http.Request) {
	hunt := r.FormValue("hunt")
	prefs := r.FormValue("preferences")
	if err := s.db.SavePreferences(r.Context(), hunt, prefs); err != nil {
		slog.Error("save preferences", "err", err)
		http.Error(w, "Error saving preferences", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/?hunt="+hunt, http.StatusSeeOther)
}

func (s *Server) handleSaveFeedback(w http.ResponseWriter, r *http.Request) {
	hunt := r.FormValue("hunt")
	title := r.FormValue("title")
	rating := r.FormValue("rating")
	note := r.FormValue("note")
	oppIDStr := r.FormValue("opportunity_id")

	fb := storage.FeedbackRow{
		HuntName:  hunt,
		Title:     title,
		Rating:    rating,
		Note:      note,
		CreatedAt: time.Now(),
	}
	if oppIDStr != "" {
		id, err := strconv.ParseInt(oppIDStr, 10, 64)
		if err == nil {
			fb.OpportunityID = &id

			// Snapshot the evaluation context at feedback time.
			picks, _ := s.db.GetPicksForOpportunity(r.Context(), id)
			if len(picks) > 0 {
				pick := picks[0]
				fb.EvalScore = pick.DisplayScore
				// Extract summary from pick attributes.
				var attrs map[string]any
				if pick.Attributes != nil {
					_ = json.Unmarshal(pick.Attributes, &attrs)
				}
				if attrs != nil {
					if s, ok := attrs["summary"].(string); ok {
						fb.EvalSummary = s
					}
				}
				// Fall back to reason if no summary.
				if fb.EvalSummary == "" {
					fb.EvalSummary = pick.Reason
				}
			}
		}
	}

	if _, err := s.db.SaveFeedback(r.Context(), fb); err != nil {
		slog.Error("save feedback", "err", err)
		http.Error(w, "Error saving feedback", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/?hunt="+hunt, http.StatusSeeOther)
}

func (s *Server) handleSaveSchedule(w http.ResponseWriter, r *http.Request) {
	hunt := r.FormValue("hunt")
	intervalStr := r.FormValue("interval")
	interval, err := strconv.Atoi(intervalStr)
	if err != nil || interval <= 0 {
		http.Error(w, "Invalid interval", http.StatusBadRequest)
		return
	}

	startHour, startMinute := 6, 0 // default
	if st := r.FormValue("start_time"); st != "" {
		if _, parseErr := fmt.Sscanf(st, "%d:%d", &startHour, &startMinute); parseErr != nil {
			http.Error(w, "Invalid start time", http.StatusBadRequest)
			return
		}
		if startHour < 0 || startHour > 23 || startMinute < 0 || startMinute > 59 {
			http.Error(w, "Start time out of range", http.StatusBadRequest)
			return
		}
	}

	startDay := -1 // not applicable for sub-weekly
	if interval >= 10080 {
		if dayStr := r.FormValue("start_day"); dayStr != "" {
			if _, parseErr := fmt.Sscanf(dayStr, "%d", &startDay); parseErr != nil || startDay < 0 || startDay > 6 {
				http.Error(w, "Invalid day of week", http.StatusBadRequest)
				return
			}
		} else {
			startDay = int(time.Now().Weekday()) // default to today
		}
	}

	now := time.Now()
	if err := s.db.SaveSchedule(r.Context(), hunt, interval, startHour, startMinute, startDay, now); err != nil {
		slog.Error("save schedule", "err", err)
		http.Error(w, "Error saving schedule", http.StatusInternalServerError)
		return
	}
	slog.Info("schedule updated", "hunt", hunt, "interval_m", interval,
		"start", fmt.Sprintf("%02d:%02d", startHour, startMinute), "day", startDay)
	http.Redirect(w, r, "/?hunt="+hunt, http.StatusSeeOther)
}
