package web

import (
	"context"
	"embed"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
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
}

// New creates a web server.
func New(db *storage.DB, hunts []HuntInfo) (*Server, error) {
	tmpl, err := template.New("").ParseFS(templateFS, "templates/*.html")
	if err != nil {
		return nil, fmt.Errorf("parse templates: %w", err)
	}

	names := make([]string, len(hunts))
	for i, h := range hunts {
		names[i] = h.Name
	}

	return &Server{db: db, tmpl: tmpl, hunts: hunts, huntNames: names}, nil
}

// SetStatus updates the last pipeline run status.
func (s *Server) SetStatus(status *StatusInfo) {
	s.lastStatus = status
}

// Handler returns the HTTP handler for the web UI.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /", s.handleIndex)
	mux.HandleFunc("POST /preferences", s.handleSavePreferences)
	mux.HandleFunc("POST /feedback", s.handleSaveFeedback)
	return mux
}

type pageData struct {
	Hunts           []string
	ActiveHunt      string
	Cards           []core.CardData
	Preferences     string
	FeedbackOptions []core.FeedbackOption
	FeedbackItems   []feedbackItem
	Status          *StatusInfo
	CurrentOppID    int64
	CurrentTitle    string
}

type feedbackItem struct {
	ID    int64
	Title string
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	activeHunt := r.URL.Query().Get("hunt")
	if activeHunt == "" && len(s.huntNames) > 0 {
		activeHunt = s.huntNames[0]
	}

	ctx := r.Context()
	data := pageData{
		Hunts:      s.huntNames,
		ActiveHunt: activeHunt,
		Status:     s.lastStatus,
	}

	// Load preferences.
	prefs, _ := s.db.GetPreferences(ctx, activeHunt)
	data.Preferences = prefs

	// Load feedback options for active hunt.
	for _, h := range s.hunts {
		if h.Name == activeHunt {
			data.FeedbackOptions = h.FeedbackOptions
			break
		}
	}

	// Load cards: get evaluated/notified opportunities with their latest picks.
	data.Cards = s.loadCards(ctx, activeHunt)

	if err := s.tmpl.ExecuteTemplate(w, "layout.html", data); err != nil {
		slog.Error("render template", "err", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

func (s *Server) loadCards(ctx context.Context, huntName string) []core.CardData {
	// Get opportunities in evaluated or notified state.
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
			pick := picks[0] // latest pick

			var venue core.Venue
			if opp.VenueID != nil {
				venue, _ = s.db.GetVenue(ctx, *opp.VenueID)
			}

			// Find card renderer for this hunt.
			var renderer core.CardRenderer
			for _, h := range s.hunts {
				if h.Name == huntName {
					renderer = h.CardRenderer
					break
				}
			}
			if renderer != nil {
				cards = append(cards, renderer.RenderCard(opp, pick, venue))
			} else {
				// Default card rendering.
				cards = append(cards, core.CardData{
					Title:    opp.Title,
					Subtitle: opp.Subtitle,
					Score:    pick.DisplayScore,
					Reason:   pick.Reason,
					Urgency:  pick.Urgency,
				})
			}
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
		}
	}

	if _, err := s.db.SaveFeedback(r.Context(), fb); err != nil {
		slog.Error("save feedback", "err", err)
		http.Error(w, "Error saving feedback", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/?hunt="+hunt, http.StatusSeeOther)
}
