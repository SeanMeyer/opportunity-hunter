package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/seanmeyer/opportunity-hunter/core"
	"github.com/seanmeyer/opportunity-hunter/hunts/comedy"
	cs "github.com/seanmeyer/opportunity-hunter/hunts/comedy/sources"
	"github.com/seanmeyer/opportunity-hunter/hunts/powder"
	"github.com/seanmeyer/opportunity-hunter/hunts/powder/catalog"
	ps "github.com/seanmeyer/opportunity-hunter/hunts/powder/sources"
	"github.com/seanmeyer/opportunity-hunter/hunts/powder/weather"
	"github.com/seanmeyer/opportunity-hunter/pipeline"
	"github.com/seanmeyer/opportunity-hunter/storage"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type transport struct {
	mu      sync.Mutex
	Results []string
}

func (t *transport) RoundTrip(r *http.Request) (*http.Response, error) {
	resp, e := http.DefaultTransport.RoundTrip(r)
	status := "network error"
	if resp != nil {
		status = resp.Status
	}
	t.mu.Lock()
	t.Results = append(t.Results, r.URL.Host+": "+status)
	t.mu.Unlock()
	return resp, e
}

type sourceHunt struct {
	core.Hunt
	s core.Source
}

func (h sourceHunt) Sources() []core.Source { return []core.Source{h.s} }
func (h sourceHunt) MultiDateKey(r core.RawItem) string {
	if m, ok := h.Hunt.(core.MultiDateMerger); ok {
		return m.MultiDateKey(r)
	}
	return ""
}
func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: source-cycle OUTPUT_DIRECTORY")
		os.Exit(2)
	}
	dir := os.Args[1]
	if e := os.MkdirAll(dir, 0700); e != nil {
		panic(e)
	}
	var prior []json.RawMessage
	history := filepath.Join(dir, "cycles.json")
	if data, e := os.ReadFile(history); e == nil {
		if e := json.Unmarshal(data, &prior); e != nil {
			panic(e)
		}
	}
	if len(prior) >= 4 {
		fmt.Println("Trial complete: four source cycles recorded")
		return
	}
	logfile, e := os.OpenFile(filepath.Join(dir, "source.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if e != nil {
		panic(e)
	}
	defer logfile.Close()
	slog.SetDefault(slog.New(slog.NewTextHandler(logfile, nil)))
	db, e := storage.Open(filepath.Join(dir, "trial.db"))
	if e != nil {
		panic(e)
	}
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	tr := &transport{}
	client := &http.Client{Timeout: 20 * time.Second, Transport: tr}
	regions := catalog.RegionsForUser(39.75, -104.99)
	var selected []catalog.RegionWithResorts
	for _, r := range regions {
		if r.Region.Country == "US" && len(r.Resorts) > 0 {
			r.Resorts = r.Resorts[:1]
			selected = []catalog.RegionWithResorts{r}
			break
		}
	}
	svc := weather.NewService(weather.NewOpenMeteoClient(client), weather.NewNWSClient(client))
	hunts := []core.Hunt{sourceHunt{&comedy.ComedyHunt{}, cs.NewComedyWorks()}, sourceHunt{&powder.PowderHunt{}, ps.NewWeatherSource(svc, selected)}}
	p := pipeline.New(db, core.NewCostTracker(0, nil), quietNotifier{}, core.ScanRegion{}, "")
	started := time.Now()
	result := p.ScanAll(ctx, hunts)
	counts := map[string]int{}
	for _, h := range hunts {
		o, e := db.GetByState(ctx, h.Name(), core.Discovered)
		if e != nil {
			panic(e)
		}
		counts[h.Name()] = len(o)
	}
	record := map[string]any{"cycle": len(prior) + 1, "started_at": started.UTC(), "duration_seconds": time.Since(started).Seconds(), "result": result, "stored_discovered_counts": counts, "weather_requests": tr.Results, "weather_region": selected[0].Region.Name, "ai_calls": 0, "external_notifications": 0}
	data, _ := json.MarshalIndent(record, "", "  ")
	prior = append(prior, data)
	all, _ := json.MarshalIndent(prior, "", "  ")
	if e = os.WriteFile(history, all, 0600); e != nil {
		panic(e)
	}
	fmt.Println(string(data))
	fmt.Println("History:", history)
}

type quietNotifier struct{}

func (quietNotifier) ExecuteActions(string, []core.NotifyAction) (map[string]string, error) {
	panic("source-only monitor attempted notification")
}
func (quietNotifier) PostError(string) error {
	panic("source-only monitor attempted external error message")
}
