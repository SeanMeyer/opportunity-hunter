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

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	switch args[0] {
	case "run":
		return runDaemon()
	case "scan":
		return runScan()
	case "eval":
		return runEval()
	case "web":
		return runWeb()
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
  version   Print version and exit`)
}

// registeredHunts returns all hunts. Import hunt packages here.
// Hunts are registered but only enabled ones are initialized.
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

	// Notifier — use first hunt's webhook for now, error webhook separate.
	notifier := &noopNotifier{}
	var pipeNotifier pipeline.Notifier = notifier

	// Pipeline.
	pipe := pipeline.New(db, costTracker, pipeNotifier)

	// Web server.
	huntInfos := buildHuntInfos(hunts)
	webServer, err := web.New(db, huntInfos)
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

	// Initial scan + eval.
	slog.Info("running initial pipeline")
	result := pipe.RunAll(ctx, hunts)
	logResult(result)

	// Schedule loop.
	scanTicker := time.NewTicker(12 * time.Hour)
	defer scanTicker.Stop()

	slog.Info("daemon started", "hunts", len(hunts), "web_port", cfg.WebPort)

	for {
		select {
		case <-ctx.Done():
			slog.Info("shutting down")
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			httpServer.Shutdown(shutdownCtx)
			return 0
		case <-scanTicker.C:
			slog.Info("scheduled pipeline run")
			result := pipe.RunAll(ctx, hunts)
			logResult(result)
			webServer.SetStatus(toStatusInfo(result))
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
	notifier := &noopNotifier{}
	pipe := pipeline.New(db, costTracker, notifier)

	// Only scan step — set dry run conceptually by using noop notifier.
	result := pipe.RunAll(ctx, hunts)
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

	// Create per-hunt notifiers from config.
	notifier := &noopNotifier{}
	pipe := pipeline.New(db, costTracker, notifier)

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

	webServer, err := web.New(db, huntInfos)
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
		}
		infos = append(infos, info)
	}
	return infos
}

func logResult(result core.PipelineResult) {
	for _, hr := range result.HuntResults {
		slog.Info("hunt result",
			"hunt", hr.HuntName,
			"scanned", hr.Scanned,
			"evaluated", hr.Evaluated,
			"notified", hr.Notified,
			"errors", len(hr.Errors),
		)
	}
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

// noopNotifier discards all notifications (for scan-only and dry-run modes).
type noopNotifier struct{}

func (n *noopNotifier) ExecuteActions(_ []core.NotifyAction) error { return nil }
func (n *noopNotifier) PostError(_ string) error                   { return nil }

// Ensure notify package is used (will be needed when we wire real notifiers).
var _ = notify.NewClient
