package main

import (
	"bufio"
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/seanmeyer/opportunity-hunter/config"
	"github.com/seanmeyer/opportunity-hunter/core"
	"github.com/seanmeyer/opportunity-hunter/hunts/comedy"
	"github.com/seanmeyer/opportunity-hunter/hunts/movies"
	"github.com/seanmeyer/opportunity-hunter/hunts/performing"
	"github.com/seanmeyer/opportunity-hunter/hunts/powder"
	"github.com/seanmeyer/opportunity-hunter/notify"
	"github.com/seanmeyer/opportunity-hunter/pipeline"
	"github.com/seanmeyer/opportunity-hunter/storage"
	"github.com/seanmeyer/opportunity-hunter/web"
)

var version = "dev"

func main() {
	loadEnvFile(".env")
	os.Exit(run(os.Args[1:]))
}

func loadEnvFile(path string) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if os.Getenv(key) == "" {
			os.Setenv(key, value)
		}
	}
}

func run(args []string) int {
	if len(args) == 0 {
		printUsage()
		return 1
	}

	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))

	switch args[0] {
	case "run":
		return runDaemon()
	case "scan":
		return runScan()
	case "eval":
		return runEval()
	case "web":
		return runWeb()
	case "profile":
		return runProfile(args[1:])
	case "trace":
		return runTrace(args[1:])
	case "version":
		fmt.Println(version)
		return 0
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", args[0])
		printUsage()
		return 1
	}
}

func printUsage() {
	fmt.Fprintln(os.Stderr, `Usage: opportunity-hunter <command>

Commands:
  run       Start the daemon (scan + evaluate + notify + web UI)
  scan      Run scan only (no LLM, no Discord)
  eval      Run evaluation only
  web       Start the web UI only
  profile   View or set user profile
  trace     Single-region debug (scan + detect + render prompt, no LLM)
  version   Print version and exit`)
}

// registeredHunts returns all hunts. Import hunt packages here.
// Hunts are registered but only enabled ones are initialized.
const defaultScanRadiusMi = 30

func homeRegion(cfg config.Config) core.ScanRegion {
	return core.ScanRegion{
		Latitude:  cfg.HomeLatitude,
		Longitude: cfg.HomeLongitude,
		RadiusMi:  defaultScanRadiusMi,
	}
}

func registeredHunts() []core.Hunt {
	return []core.Hunt{
		&comedy.ComedyHunt{},
		&performing.PerformingHunt{},
		&movies.MoviesHunt{},
		&powder.PowderHunt{},
	}
}

func initHunts(ctx context.Context, cfg config.Config) ([]core.Hunt, error) {
	var enabled []core.Hunt
	for _, hunt := range registeredHunts() {
		name := hunt.Name()
		if !cfg.EnabledHunts[name] {
			slog.Info("hunt disabled", "hunt", name)
			continue
		}
		if err := hunt.Init(ctx, os.Getenv); err != nil {
			slog.Error("hunt init failed", "hunt", name, "err", err)
			continue
		}
		if _, err := core.ValidateHunt(hunt); err != nil {
			slog.Error("hunt validation failed", "hunt", name, "err", err)
			continue
		}
		enabled = append(enabled, hunt)
	}
	if len(enabled) == 0 {
		return nil, fmt.Errorf("no hunts enabled or initialized")
	}
	return enabled, nil
}

func seedProfileFromEnv(ctx context.Context, db *storage.DB) {
	homeBase := os.Getenv("HOME_BASE")
	if homeBase == "" {
		return
	}

	homeLat := parseFloatEnv("HOME_LATITUDE", 0)
	homeLon := parseFloatEnv("HOME_LONGITUDE", 0)

	var passes []string
	if p := os.Getenv("PASSES"); p != "" {
		for _, s := range strings.Split(p, ",") {
			passes = append(passes, strings.TrimSpace(s))
		}
	}

	ptoDays := 0
	if p := os.Getenv("PTO_DAYS"); p != "" {
		if v, err := strconv.Atoi(p); err == nil {
			ptoDays = v
		}
	}

	profile := &core.UserProfile{
		HuntName:   "", // global profile
		HomeBase:   homeBase,
		HomeLat:    homeLat,
		HomeLon:    homeLon,
		Passes:     passes,
		SkillLevel: os.Getenv("SKILL_LEVEL"),
		RemoteWork: os.Getenv("REMOTE_WORK") == "true" || os.Getenv("REMOTE_WORK") == "1",
		PTODays:    ptoDays,
	}

	if err := db.SaveProfileIfNotExists(ctx, profile); err != nil {
		slog.Warn("failed to seed profile", "err", err)
	}
}

func parseFloatEnv(key string, fallback float64) float64 {
	s := os.Getenv(key)
	if s == "" {
		return fallback
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return fallback
	}
	return v
}

func runDaemon() int {
	cfg, err := config.FromEnv(os.Getenv)
	if err != nil {
		slog.Error("config error", "err", err)
		return 1
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	db, err := storage.Open(cfg.DBPath)
	if err != nil {
		slog.Error("open database", "err", err)
		return 1
	}
	defer db.Close()

	logHandler := storage.NewDBLogHandler(db, slog.LevelInfo)
	defer logHandler.Stop()
	slog.SetDefault(slog.New(logHandler))

	// Seed global user profile from .env on first run.
	seedProfileFromEnv(ctx, db)

	hunts, err := initHunts(ctx, cfg)
	if err != nil {
		slog.Error("init hunts", "err", err)
		return 1
	}

	// Cost tracker seeded from DB.
	monthly, err := db.MonthlySpend(ctx, time.Now())
	if err != nil {
		slog.Error("load monthly spend", "err", err)
		return 1
	}
	costTracker := core.NewCostTracker(monthly.Total, monthly.ByHunt)

	// Notifier — route to per-hunt Discord webhooks, or noop if dry-run.
	var pipeNotifier pipeline.Notifier
	if cfg.DryRun {
		pipeNotifier = &noopNotifier{}
	} else {
		pipeNotifier = newRoutingNotifier(cfg.HuntWebhooks, cfg.ErrorDiscordWebhookURL)
	}

	// Pipeline.
	pipe := pipeline.New(db, costTracker, pipeNotifier, homeRegion(cfg), cfg.HomeAddress)
	pipe.SetDryRun(cfg.DryRun)

	// Concurrency guard — prevents overlapping scheduled and manual runs.
	guard := pipeline.NewRunGuard()
	guard.TryAcquire("startup") // Reserve before HTTP can accept manual triggers.
	var manualMu sync.Mutex
	var manualRuns sync.WaitGroup

	// Web server.
	huntInfos := buildHuntInfos(hunts)

	// runFunc is called (in a goroutine) when the user triggers a manual run.
	// It must capture webServer by pointer so it can call SetStatus after creation.
	var webServer *web.Server
	runFunc := func(_ context.Context, huntName string) {
		manualMu.Lock()
		if ctx.Err() != nil {
			manualMu.Unlock()
			return
		}
		manualRuns.Add(1)
		manualMu.Unlock()
		defer manualRuns.Done()
		runCtx := ctx
		if !guard.TryAcquire(huntName) {
			slog.Info("hunt already running, skipping manual trigger", "hunt", huntName)
			return
		}
		defer guard.Release(huntName)
		runCtx = storage.ContextWithTrigger(runCtx, "manual")
		for _, h := range hunts {
			if h.Name() == huntName {
				hr := pipe.Run(runCtx, h)
				logHuntResult(hr)
				if webServer != nil {
					webServer.SetStatus(toStatusInfo(core.PipelineResult{HuntResults: []core.HuntResult{hr}}))
				}
				break
			}
		}
	}

	webServer, err = web.New(db, huntInfos, cfg.HomeAddress, runFunc)
	if err != nil {
		slog.Error("create web server", "err", err)
		return 1
	}

	addr := ":" + strconv.Itoa(cfg.WebPort)
	httpServer := &http.Server{Addr: addr, Handler: webServer.Handler()}
	go func() {
		slog.Info("web server starting", "addr", addr)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("web server error", "err", err)
		}
	}()

	// Seed schedules for all hunts (creates rows for new hunts, preserves existing).
	huntsByName := make(map[string]core.Hunt, len(hunts))
	for _, h := range hunts {
		huntsByName[h.Name()] = h
		sched := h.DefaultSchedule()
		intervalM := int(sched.ScanInterval.Minutes())
		if intervalM <= 0 {
			intervalM = 720 // default 12h
		}
		if err := db.SeedScheduleIfNotExists(ctx, h.Name(), intervalM, time.Now()); err != nil {
			slog.Warn("seed schedule", "hunt", h.Name(), "err", err)
		}
	}

	// Initial scan + eval.
	slog.Info("running initial pipeline")
	result := pipe.RunAll(ctx, hunts)
	guard.Release("startup")
	logResult(result)

	// Advance next_scan_at for all hunts after initial run.
	for _, h := range hunts {
		if err := db.AdvanceNextScan(ctx, h.Name(), time.Now()); err != nil {
			slog.Warn("advance schedule after initial run", "hunt", h.Name(), "err", err)
		}
	}

	// Per-hunt schedule loop: check every minute for due hunts.
	checkTicker := time.NewTicker(1 * time.Minute)
	defer checkTicker.Stop()

	// Hourly log pruning: remove logs older than 7 days.
	go func() {
		for {
			select {
			case <-time.After(1 * time.Hour):
				if n, err := db.PruneLogs(ctx, 7*24*time.Hour); err != nil {
					slog.Warn("prune logs", "err", err)
				} else if n > 0 {
					slog.Info("pruned logs", "count", n)
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	slog.Info("daemon started", "hunts", len(hunts), "web_port", cfg.WebPort)

	for {
		select {
		case <-ctx.Done():
			slog.Info("shutting down")
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			httpServer.Shutdown(shutdownCtx)
			manualMu.Lock()
			manualRuns.Wait()
			manualMu.Unlock()
			return 0
		case <-checkTicker.C:
			dueHunts, err := db.GetDueHunts(ctx, time.Now())
			if err != nil {
				slog.Error("check due hunts", "err", err)
				continue
			}
			for _, name := range dueHunts {
				hunt, ok := huntsByName[name]
				if !ok {
					slog.Warn("due hunt not found in registry", "hunt", name)
					continue
				}
				if !guard.TryAcquire(name) {
					slog.Info("hunt already running, skipping", "hunt", name)
					continue
				}
				slog.Info("scheduled hunt run", "hunt", name)
				hr := pipe.Run(ctx, hunt)
				guard.Release(name)
				logHuntResult(hr)
				webServer.SetStatus(toStatusInfo(core.PipelineResult{HuntResults: []core.HuntResult{hr}}))
				if err := db.AdvanceNextScan(ctx, name, time.Now()); err != nil {
					slog.Error("advance schedule", "hunt", name, "err", err)
				}
			}
		}
	}
}

func runScan() int {
	cfg, err := config.FromEnv(os.Getenv)
	if err != nil {
		slog.Error("config error", "err", err)
		return 1
	}

	ctx := context.Background()
	db, err := storage.Open(cfg.DBPath)
	if err != nil {
		slog.Error("open database", "err", err)
		return 1
	}
	defer db.Close()

	hunts, err := initHunts(ctx, cfg)
	if err != nil {
		slog.Error("init hunts", "err", err)
		return 1
	}

	costTracker := core.NewCostTracker(0, nil)
	// Scan uses noop notifier — no notifications for scan-only.
	pipe := pipeline.New(db, costTracker, &noopNotifier{}, homeRegion(cfg), cfg.HomeAddress)
	pipe.SetDryRun(true)

	result := pipe.ScanAll(ctx, hunts)
	logResult(result)
	return 0
}

func runEval() int {
	cfg, err := config.FromEnv(os.Getenv)
	if err != nil {
		slog.Error("config error", "err", err)
		return 1
	}

	ctx := context.Background()
	db, err := storage.Open(cfg.DBPath)
	if err != nil {
		slog.Error("open database", "err", err)
		return 1
	}
	defer db.Close()

	hunts, err := initHunts(ctx, cfg)
	if err != nil {
		slog.Error("init hunts", "err", err)
		return 1
	}

	monthly, _ := db.MonthlySpend(ctx, time.Now())
	costTracker := core.NewCostTracker(monthly.Total, monthly.ByHunt)

	// Eval sends real notifications if webhooks are configured.
	var pipeNotifier pipeline.Notifier
	if cfg.DryRun {
		pipeNotifier = &noopNotifier{}
	} else {
		pipeNotifier = newRoutingNotifier(cfg.HuntWebhooks, cfg.ErrorDiscordWebhookURL)
	}
	pipe := pipeline.New(db, costTracker, pipeNotifier, homeRegion(cfg), cfg.HomeAddress)
	pipe.SetDryRun(cfg.DryRun)

	result := pipe.RunAll(ctx, hunts)
	logResult(result)
	return 0
}

func runWeb() int {
	cfg, err := config.FromEnv(os.Getenv)
	if err != nil {
		slog.Error("config error", "err", err)
		return 1
	}

	db, err := storage.Open(cfg.DBPath)
	if err != nil {
		slog.Error("open database", "err", err)
		return 1
	}
	defer db.Close()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	hunts, _ := initHunts(ctx, cfg)
	huntInfos := buildHuntInfos(hunts)

	webServer, err := web.New(db, huntInfos, cfg.HomeAddress)
	if err != nil {
		slog.Error("create web server", "err", err)
		return 1
	}

	addr := ":" + strconv.Itoa(cfg.WebPort)
	fmt.Printf("Web UI running at http://localhost%s\n", addr)

	httpServer := &http.Server{Addr: addr, Handler: webServer.Handler()}
	go func() {
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("web server error", "err", err)
		}
	}()

	<-ctx.Done()
	httpServer.Shutdown(context.Background())
	return 0
}

func buildHuntInfos(hunts []core.Hunt) []web.HuntInfo {
	var infos []web.HuntInfo
	for _, h := range hunts {
		info := web.HuntInfo{Name: h.Name()}
		if wh, ok := h.(core.WebHunt); ok {
			info.CardRenderer = wh.CardRenderer()
			info.FeedbackOptions = wh.FeedbackOptions()
			info.WebConfig = wh.WebConfig()
		}
		infos = append(infos, info)
	}
	return infos
}

func logResult(result core.PipelineResult) {
	for _, hr := range result.HuntResults {
		logHuntResult(hr)
	}
}

func logHuntResult(hr core.HuntResult) {
	slog.Info("hunt result",
		"hunt", hr.HuntName,
		"scanned", hr.Scanned,
		"evaluated", hr.Evaluated,
		"notified", hr.Notified,
		"errors", len(hr.Errors),
	)
}

func toStatusInfo(result core.PipelineResult) *web.StatusInfo {
	hasErrors := result.HasErrors()
	var summary string
	if hasErrors {
		summary = fmt.Sprintf("%d hunts ran with errors", len(result.HuntResults))
	} else {
		summary = fmt.Sprintf("%d hunts ran successfully", len(result.HuntResults))
	}
	return &web.StatusInfo{
		LastRun:   time.Now(),
		Summary:   summary,
		HasErrors: hasErrors,
	}
}

func runProfile(args []string) int {
	cfg, err := config.FromEnv(os.Getenv)
	if err != nil {
		slog.Error("config error", "err", err)
		return 1
	}

	ctx := context.Background()
	db, err := storage.Open(cfg.DBPath)
	if err != nil {
		slog.Error("open database", "err", err)
		return 1
	}
	defer db.Close()

	if len(args) > 0 && args[0] == "set" {
		// Set profile from env vars.
		seedProfileFromEnv(ctx, db)
		fmt.Println("Profile updated from environment variables.")
		return 0
	}

	// View profile.
	profile, err := db.GetProfile(ctx, "")
	if err != nil {
		fmt.Println("No profile found. Set environment variables and run: opportunity-hunter profile set")
		return 0
	}

	fmt.Println("User Profile:")
	fmt.Printf("  Home: %s (%.4f, %.4f)\n", profile.HomeBase, profile.HomeLat, profile.HomeLon)
	if len(profile.Passes) > 0 {
		fmt.Printf("  Passes: %s\n", strings.Join(profile.Passes, ", "))
	}
	if profile.SkillLevel != "" {
		fmt.Printf("  Skill: %s\n", profile.SkillLevel)
	}
	if profile.RemoteWork {
		fmt.Println("  Remote work: yes")
	}
	if profile.PTODays > 0 {
		fmt.Printf("  PTO remaining: %d days\n", profile.PTODays)
	}
	return 0
}

func runTrace(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "Usage: opportunity-hunter trace <region-name>")
		fmt.Fprintln(os.Stderr, "  Runs scan + detect + renders prompt for a single region (no LLM call)")
		return 1
	}
	regionFilter := strings.ToLower(strings.Join(args, " "))

	cfg, err := config.FromEnv(os.Getenv)
	if err != nil {
		slog.Error("config error", "err", err)
		return 1
	}

	ctx := context.Background()
	db, err := storage.Open(cfg.DBPath)
	if err != nil {
		slog.Error("open database", "err", err)
		return 1
	}
	defer db.Close()

	// Init powder hunt.
	ph := &powder.PowderHunt{}
	if err := ph.Init(ctx, os.Getenv); err != nil {
		slog.Error("init powder hunt", "err", err)
		return 1
	}

	// Scan.
	fmt.Printf("Scanning weather for regions matching %q...\n", regionFilter)
	sources := ph.Sources()
	var items []core.RawItem
	for _, src := range sources {
		scanItems, err := src.Scan(ctx, core.ScanRegion{})
		if err != nil {
			slog.Error("scan failed", "source", src.Name(), "err", err)
			continue
		}
		items = append(items, scanItems...)
	}

	// Filter to matching region.
	var matched []core.RawItem
	for _, item := range items {
		if strings.Contains(strings.ToLower(item.Title), regionFilter) {
			matched = append(matched, item)
		}
	}

	if len(matched) == 0 {
		fmt.Printf("No weather detections found for %q. Available regions:\n", regionFilter)
		for _, item := range items {
			fmt.Printf("  - %s: %s\n", item.Title, item.Subtitle)
		}
		return 0
	}

	fmt.Printf("\nFound %d detection(s):\n", len(matched))
	for _, item := range matched {
		fmt.Printf("  %s: %s\n", item.Title, item.Subtitle)
	}

	// Build a mock EvalContext and render the prompt.
	opps := make([]core.Opportunity, len(matched))
	for i, item := range matched {
		startTime, _ := time.Parse(time.RFC3339, item.StartTime)
		opps[i] = core.Opportunity{
			ID:         int64(i + 1),
			HuntName:   "powder",
			Title:      item.Title,
			Subtitle:   item.Subtitle,
			StartTime:  startTime,
			Attributes: item.Attributes,
			RawData:    item.RawJSON,
		}
	}

	// Load profile for prompt.
	profile, _ := db.GetProfile(ctx, "powder")
	prefs, _ := db.GetPreferences(ctx, "powder")

	ct := core.NewCostTracker(0, nil)
	ec := core.EvalContext{
		Opportunities: opps,
		Venues:        make(map[int64]core.Venue),
		Preferences:   prefs,
		Profile:       profile,
		CostTracker:   ct,
	}

	// Use the powder evaluator's prompt builder via the exported helper.
	prompt := powder.BuildPromptForTrace(ec)

	fmt.Printf("\n=== RENDERED PROMPT (%d chars) ===\n\n", len(prompt))
	fmt.Println(prompt)

	return 0
}

// routingNotifier routes notifications to per-hunt Discord webhooks.
// Hunts without a configured webhook silently discard notifications.
type routingNotifier struct {
	clients     map[string]*notify.Client // hunt name → client
	errorClient *notify.Client
}

func newRoutingNotifier(webhooks map[string]string, errorWebhook string) *routingNotifier {
	rn := &routingNotifier{clients: make(map[string]*notify.Client)}
	for hunt, url := range webhooks {
		if url != "" {
			rn.clients[hunt] = notify.NewClient(url)
		}
	}
	if errorWebhook != "" {
		rn.errorClient = notify.NewClient(errorWebhook)
	}
	return rn
}

func (rn *routingNotifier) ExecuteActions(huntName string, actions []core.NotifyAction) (map[string]string, error) {
	client, ok := rn.clients[huntName]
	if !ok {
		slog.Info("no webhook configured, skipping notifications", "hunt", huntName)
		return nil, nil
	}
	return client.ExecuteActions(context.Background(), actions)
}

func (rn *routingNotifier) PostError(message string) error {
	if rn.errorClient == nil {
		return nil
	}
	return rn.errorClient.PostError(context.Background(), message)
}

// noopNotifier discards all notifications (for dry-run mode).
type noopNotifier struct{}

func (n *noopNotifier) ExecuteActions(_ string, _ []core.NotifyAction) (map[string]string, error) {
	return nil, nil
}
func (n *noopNotifier) PostError(_ string) error { return nil }
