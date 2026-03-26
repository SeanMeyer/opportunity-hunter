# Web UI Overhaul Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add operational visibility (run history, logs, costs, manual triggers) and modernize the web UI with a dark, information-dense theme.

**Architecture:** Bottom-up — new storage tables and queries first, then the slog handler, then pipeline integration, then web routes and templates. Each task produces a working, testable increment. The concurrency guard is a shared component wired into both the daemon loop and the web layer.

**Tech Stack:** Go 1.25, SQLite via `modernc.org/sqlite`, `html/template`, `log/slog`, `crypto/rand` (for run IDs)

**Spec:** `docs/superpowers/specs/2026-03-25-web-ui-overhaul-design.md`

**Mockups:** `.superpowers/brainstorm/50671-1774500580/style-dense.html` (cards), `dashboard.html` (status page)

---

### Task 1: Pipeline Runs — Storage Layer

Add the `pipeline_runs` table and CRUD methods.

**Files:**
- Modify: `storage/schema.sql` (add table DDL)
- Create: `storage/runs.go` (new file for run queries)
- Create: `storage/runs_test.go`

- [ ] **Step 1: Write failing test for InsertRun and FinishRun**

```go
// storage/runs_test.go
package storage_test

import (
	"context"
	"testing"
	"time"

	"github.com/seanmeyer/opportunity-hunter/storage"
	"github.com/seanmeyer/opportunity-hunter/testutil"
)

func TestInsertAndFinishRun(t *testing.T) {
	db := testutil.NewTestDB(t)
	ctx := context.Background()

	runID, err := db.InsertRun(ctx, "comedy", "scheduled")
	if err != nil {
		t.Fatal(err)
	}
	if runID == "" {
		t.Fatal("expected non-empty run ID")
	}

	err = db.FinishRun(ctx, runID, storage.RunResult{
		Status:       "ok",
		Scanned:      4,
		Evaluated:    4,
		Notified:     2,
		CostUSD:      0.04,
		ErrorSummary: "",
	})
	if err != nil {
		t.Fatal(err)
	}

	run, err := db.LatestRun(ctx, "comedy")
	if err != nil {
		t.Fatal(err)
	}
	if run == nil {
		t.Fatal("expected a run")
	}
	if run.Status != "ok" || run.Scanned != 4 || run.Notified != 2 {
		t.Errorf("got %+v", run)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./storage/ -run TestInsertAndFinishRun -v`
Expected: FAIL — `InsertRun` method does not exist

- [ ] **Step 3: Add pipeline_runs table to schema.sql**

Add to the end of `storage/schema.sql`:

```sql
CREATE TABLE IF NOT EXISTS pipeline_runs (
    id TEXT PRIMARY KEY,
    hunt_name TEXT NOT NULL,
    started_at TEXT NOT NULL,
    finished_at TEXT,
    status TEXT NOT NULL DEFAULT 'running',
    scanned INTEGER NOT NULL DEFAULT 0,
    evaluated INTEGER NOT NULL DEFAULT 0,
    notified INTEGER NOT NULL DEFAULT 0,
    cost_usd REAL NOT NULL DEFAULT 0,
    error_summary TEXT,
    trigger TEXT NOT NULL DEFAULT 'scheduled'
);
CREATE INDEX IF NOT EXISTS idx_pipeline_runs_hunt_started ON pipeline_runs (hunt_name, started_at);
```

- [ ] **Step 4: Implement storage/runs.go**

```go
// storage/runs.go
package storage

import (
	"context"
	"crypto/rand"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// PipelineRun represents a single pipeline execution record.
type PipelineRun struct {
	ID           string
	HuntName     string
	StartedAt    time.Time
	FinishedAt   *time.Time
	Status       string
	Scanned      int
	Evaluated    int
	Notified     int
	CostUSD      float64
	ErrorSummary string
	Trigger      string
}

// RunResult holds the outcome of a pipeline run for FinishRun.
type RunResult struct {
	Status       string
	Scanned      int
	Evaluated    int
	Notified     int
	CostUSD      float64
	ErrorSummary string
}

func generateRunID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return fmt.Sprintf("%x", b)
}

// InsertRun creates a new pipeline run record and returns its ID.
func (d *DB) InsertRun(ctx context.Context, huntName, trigger string) (string, error) {
	id := generateRunID()
	_, err := d.db.ExecContext(ctx,
		`INSERT INTO pipeline_runs (id, hunt_name, started_at, trigger)
		 VALUES (?, ?, ?, ?)`,
		id, huntName, time.Now().Format(time.RFC3339), trigger,
	)
	return id, err
}

// FinishRun updates a run record with its outcome.
func (d *DB) FinishRun(ctx context.Context, runID string, r RunResult) error {
	_, err := d.db.ExecContext(ctx,
		`UPDATE pipeline_runs
		 SET finished_at = ?, status = ?, scanned = ?, evaluated = ?,
		     notified = ?, cost_usd = ?, error_summary = ?
		 WHERE id = ?`,
		time.Now().Format(time.RFC3339), r.Status, r.Scanned, r.Evaluated,
		r.Notified, r.CostUSD, nullIfEmpty(r.ErrorSummary), runID,
	)
	return err
}

func nullIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// LatestRun returns the most recent run for a hunt, or nil if none exist.
func (d *DB) LatestRun(ctx context.Context, huntName string) (*PipelineRun, error) {
	row := d.db.QueryRowContext(ctx,
		`SELECT id, hunt_name, started_at, finished_at, status, scanned,
		        evaluated, notified, cost_usd, COALESCE(error_summary, ''), trigger
		 FROM pipeline_runs WHERE hunt_name = ?
		 ORDER BY started_at DESC LIMIT 1`, huntName,
	)
	var r PipelineRun
	var startedAt, trigger string
	var finishedAt *string
	err := row.Scan(&r.ID, &r.HuntName, &startedAt, &finishedAt, &r.Status,
		&r.Scanned, &r.Evaluated, &r.Notified, &r.CostUSD, &r.ErrorSummary, &trigger)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	r.StartedAt, _ = time.Parse(time.RFC3339, startedAt)
	if finishedAt != nil {
		t, _ := time.Parse(time.RFC3339, *finishedAt)
		r.FinishedAt = &t
	}
	r.Trigger = trigger
	return &r, nil
}

// RecentRuns returns the last N runs, optionally filtered by hunt.
func (d *DB) RecentRuns(ctx context.Context, huntName string, limit int) ([]PipelineRun, error) {
	query := `SELECT id, hunt_name, started_at, finished_at, status, scanned,
	                 evaluated, notified, cost_usd, COALESCE(error_summary, ''), trigger
	          FROM pipeline_runs`
	var args []any
	if huntName != "" {
		query += " WHERE hunt_name = ?"
		args = append(args, huntName)
	}
	query += " ORDER BY started_at DESC LIMIT ?"
	args = append(args, limit)

	rows, err := d.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var runs []PipelineRun
	for rows.Next() {
		var r PipelineRun
		var startedAt, trigger string
		var finishedAt *string
		if err := rows.Scan(&r.ID, &r.HuntName, &startedAt, &finishedAt, &r.Status,
			&r.Scanned, &r.Evaluated, &r.Notified, &r.CostUSD, &r.ErrorSummary, &trigger); err != nil {
			return nil, err
		}
		r.StartedAt, _ = time.Parse(time.RFC3339, startedAt)
		if finishedAt != nil {
			t, _ := time.Parse(time.RFC3339, *finishedAt)
			r.FinishedAt = &t
		}
		r.Trigger = trigger
		runs = append(runs, r)
	}
	return runs, rows.Err()
}
```

- [ ] **Step 5: Run test to verify it passes**

Run: `go test ./storage/ -run TestInsertAndFinishRun -v`
Expected: PASS

- [ ] **Step 6: Write test for RecentRuns with hunt filter**

```go
func TestRecentRuns(t *testing.T) {
	db := testutil.NewTestDB(t)
	ctx := context.Background()

	// Insert runs for two hunts
	db.InsertRun(ctx, "comedy", "scheduled")
	db.InsertRun(ctx, "powder", "scheduled")
	db.InsertRun(ctx, "comedy", "manual")

	all, err := db.RecentRuns(ctx, "", 50)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 3 {
		t.Fatalf("expected 3 runs, got %d", len(all))
	}

	comedyOnly, err := db.RecentRuns(ctx, "comedy", 50)
	if err != nil {
		t.Fatal(err)
	}
	if len(comedyOnly) != 2 {
		t.Fatalf("expected 2 comedy runs, got %d", len(comedyOnly))
	}
}
```

- [ ] **Step 7: Run test to verify it passes**

Run: `go test ./storage/ -run TestRecentRuns -v`
Expected: PASS

- [ ] **Step 8: Commit**

```bash
git add storage/schema.sql storage/runs.go storage/runs_test.go
git commit -m "feat: add pipeline_runs table and storage methods"
```

---

### Task 2: Run Logs — Storage Layer

Add the `run_logs` table and query methods.

**Files:**
- Modify: `storage/schema.sql` (add table DDL)
- Create: `storage/logs.go`
- Create: `storage/logs_test.go`

- [ ] **Step 1: Write failing test for InsertLogs and RecentLogs**

```go
// storage/logs_test.go
package storage_test

import (
	"context"
	"testing"
	"time"

	"github.com/seanmeyer/opportunity-hunter/storage"
	"github.com/seanmeyer/opportunity-hunter/testutil"
)

func TestInsertAndQueryLogs(t *testing.T) {
	db := testutil.NewTestDB(t)
	ctx := context.Background()

	logs := []storage.RunLog{
		{RunID: "run-1", HuntName: "comedy", Timestamp: time.Now(), Level: "info", Message: "scan complete", Attrs: "{}"},
		{RunID: "run-1", HuntName: "comedy", Timestamp: time.Now(), Level: "warn", Message: "rate limit", Attrs: "{}"},
		{RunID: "", HuntName: "", Timestamp: time.Now(), Level: "info", Message: "daemon started", Attrs: "{}"},
	}

	if err := db.InsertLogs(ctx, logs); err != nil {
		t.Fatal(err)
	}

	// All logs
	all, err := db.RecentLogs(ctx, "", "", 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 3 {
		t.Fatalf("expected 3, got %d", len(all))
	}

	// Filter by hunt
	comedyLogs, err := db.RecentLogs(ctx, "comedy", "", 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(comedyLogs) != 2 {
		t.Fatalf("expected 2 comedy logs, got %d", len(comedyLogs))
	}

	// Filter by level
	warns, err := db.RecentLogs(ctx, "", "warn", 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(warns) != 1 {
		t.Fatalf("expected 1 warn, got %d", len(warns))
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./storage/ -run TestInsertAndQueryLogs -v`
Expected: FAIL

- [ ] **Step 3: Add run_logs table to schema.sql**

Add to the end of `storage/schema.sql`:

```sql
CREATE TABLE IF NOT EXISTS run_logs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    run_id TEXT,
    hunt_name TEXT,
    timestamp TEXT NOT NULL,
    level TEXT NOT NULL,
    message TEXT NOT NULL,
    attrs TEXT NOT NULL DEFAULT '{}'
);
CREATE INDEX IF NOT EXISTS idx_run_logs_hunt_ts ON run_logs (hunt_name, timestamp);
CREATE INDEX IF NOT EXISTS idx_run_logs_run ON run_logs (run_id);
```

- [ ] **Step 4: Implement storage/logs.go**

```go
// storage/logs.go
package storage

import (
	"context"
	"time"
)

// RunLog represents a single structured log entry.
type RunLog struct {
	ID        int64
	RunID     string
	HuntName  string
	Timestamp time.Time
	Level     string
	Message   string
	Attrs     string
}

// InsertLogs batch-inserts log entries.
func (d *DB) InsertLogs(ctx context.Context, logs []RunLog) error {
	if len(logs) == 0 {
		return nil
	}
	tx, err := d.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx,
		`INSERT INTO run_logs (run_id, hunt_name, timestamp, level, message, attrs)
		 VALUES (?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, l := range logs {
		runID := nullIfEmpty(l.RunID)
		huntName := nullIfEmpty(l.HuntName)
		_, err := stmt.ExecContext(ctx, runID, huntName,
			l.Timestamp.Format(time.RFC3339), l.Level, l.Message, l.Attrs)
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}

// RecentLogs returns the most recent log entries, optionally filtered.
func (d *DB) RecentLogs(ctx context.Context, huntName, level string, limit int) ([]RunLog, error) {
	query := `SELECT id, COALESCE(run_id, ''), COALESCE(hunt_name, ''),
	                 timestamp, level, message, attrs
	          FROM run_logs WHERE 1=1`
	var args []any
	if huntName != "" {
		query += " AND hunt_name = ?"
		args = append(args, huntName)
	}
	if level != "" {
		query += " AND level = ?"
		args = append(args, level)
	}
	query += " ORDER BY timestamp DESC, id DESC LIMIT ?"
	args = append(args, limit)

	rows, err := d.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []RunLog
	for rows.Next() {
		var l RunLog
		var ts string
		if err := rows.Scan(&l.ID, &l.RunID, &l.HuntName, &ts,
			&l.Level, &l.Message, &l.Attrs); err != nil {
			return nil, err
		}
		l.Timestamp, _ = time.Parse(time.RFC3339, ts)
		results = append(results, l)
	}
	return results, rows.Err()
}

// PruneLogs deletes log entries older than the given duration.
func (d *DB) PruneLogs(ctx context.Context, olderThan time.Duration) (int64, error) {
	cutoff := time.Now().Add(-olderThan).Format(time.RFC3339)
	result, err := d.db.ExecContext(ctx,
		`DELETE FROM run_logs WHERE timestamp < ?`, cutoff)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}
```

- [ ] **Step 5: Run test to verify it passes**

Run: `go test ./storage/ -run TestInsertAndQueryLogs -v`
Expected: PASS

- [ ] **Step 6: Write test for PruneLogs**

```go
func TestPruneLogs(t *testing.T) {
	db := testutil.NewTestDB(t)
	ctx := context.Background()

	old := storage.RunLog{
		Timestamp: time.Now().Add(-8 * 24 * time.Hour),
		Level: "info", Message: "old log", Attrs: "{}",
	}
	recent := storage.RunLog{
		Timestamp: time.Now(),
		Level: "info", Message: "recent log", Attrs: "{}",
	}
	db.InsertLogs(ctx, []storage.RunLog{old, recent})

	deleted, err := db.PruneLogs(ctx, 7*24*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if deleted != 1 {
		t.Fatalf("expected 1 deleted, got %d", deleted)
	}

	remaining, _ := db.RecentLogs(ctx, "", "", 100)
	if len(remaining) != 1 {
		t.Fatalf("expected 1 remaining, got %d", len(remaining))
	}
}
```

- [ ] **Step 7: Run test to verify it passes**

Run: `go test ./storage/ -run TestPruneLogs -v`
Expected: PASS

- [ ] **Step 8: Commit**

```bash
git add storage/schema.sql storage/logs.go storage/logs_test.go
git commit -m "feat: add run_logs table and storage methods"
```

---

### Task 3: Custom slog Handler for DB Logging

Write a `slog.Handler` that tees log output to both stdout and the `run_logs` table with buffering.

**Files:**
- Create: `storage/loghandler.go`
- Create: `storage/loghandler_test.go`

- [ ] **Step 1: Write failing test**

```go
// storage/loghandler_test.go
package storage_test

import (
	"context"
	"log/slog"
	"testing"

	"github.com/seanmeyer/opportunity-hunter/storage"
	"github.com/seanmeyer/opportunity-hunter/testutil"
)

func TestDBLogHandler(t *testing.T) {
	db := testutil.NewTestDB(t)
	ctx := context.Background()

	handler := storage.NewDBLogHandler(db, slog.LevelInfo)
	logger := slog.New(handler)

	logger.InfoContext(ctx, "scan complete", "hunt", "comedy")
	logger.WarnContext(ctx, "rate limit hit", "hunt", "movies")
	logger.InfoContext(ctx, "daemon started") // no hunt

	handler.Flush()

	logs, err := db.RecentLogs(ctx, "", "", 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(logs) != 3 {
		t.Fatalf("expected 3 logs, got %d", len(logs))
	}

	// Verify hunt extraction
	comedyLogs, _ := db.RecentLogs(ctx, "comedy", "", 100)
	if len(comedyLogs) != 1 {
		t.Fatalf("expected 1 comedy log, got %d", len(comedyLogs))
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./storage/ -run TestDBLogHandler -v`
Expected: FAIL

- [ ] **Step 3: Implement storage/loghandler.go**

```go
// storage/loghandler.go
package storage

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"slices"
	"sync"
	"time"
)

// contextKey is unexported to avoid collisions.
type contextKey string

const runIDKey contextKey = "run_id"

// ContextWithRunID returns a context carrying the given run ID for log association.
func ContextWithRunID(ctx context.Context, runID string) context.Context {
	return context.WithValue(ctx, runIDKey, runID)
}

// logBuffer is the shared write buffer for all handler instances derived from the same root.
type logBuffer struct {
	mu       sync.Mutex
	buf      []RunLog
	db       *DB
	stopOnce sync.Once
	stopCh   chan struct{}
	doneCh   chan struct{}
}

// DBLogHandler is a slog.Handler that writes to both stdout (JSON) and a SQLite run_logs table.
type DBLogHandler struct {
	shared *logBuffer
	stdout slog.Handler
	level  slog.Level
	attrs  []slog.Attr
	groups []string
}

const (
	logBufSize    = 50
	logFlushEvery = 2 * time.Second
)

// NewDBLogHandler creates a handler that logs to both stdout and the database.
func NewDBLogHandler(db *DB, level slog.Level) *DBLogHandler {
	shared := &logBuffer{
		buf:    make([]RunLog, 0, logBufSize),
		db:     db,
		stopCh: make(chan struct{}),
		doneCh: make(chan struct{}),
	}
	h := &DBLogHandler{
		shared: shared,
		stdout: slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level}),
		level:  level,
	}
	go h.flushLoop()
	return h
}

func (h *DBLogHandler) flushLoop() {
	defer close(h.shared.doneCh)
	ticker := time.NewTicker(logFlushEvery)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			h.Flush()
		case <-h.shared.stopCh:
			h.Flush()
			return
		}
	}
}

// Enabled reports whether the handler handles records at the given level.
func (h *DBLogHandler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= h.level
}

// Handle processes a log record: writes to stdout and buffers for DB.
func (h *DBLogHandler) Handle(ctx context.Context, r slog.Record) error {
	// Always write to stdout
	_ = h.stdout.Handle(ctx, r)

	// Extract hunt and run_id from record attrs and context
	var huntName, runID string
	attrs := make(map[string]any)

	// Collect pre-set attrs from WithAttrs
	for _, a := range h.attrs {
		if a.Key == "hunt" {
			huntName = a.Value.String()
		}
		attrs[a.Key] = a.Value.Any()
	}

	r.Attrs(func(a slog.Attr) bool {
		if a.Key == "hunt" {
			huntName = a.Value.String()
		}
		attrs[a.Key] = a.Value.Any()
		return true
	})

	if v, ok := ctx.Value(runIDKey).(string); ok {
		runID = v
	}

	attrsJSON, _ := json.Marshal(attrs)

	entry := RunLog{
		RunID:     runID,
		HuntName:  huntName,
		Timestamp: r.Time,
		Level:     r.Level.String(),
		Message:   r.Message,
		Attrs:     string(attrsJSON),
	}

	h.shared.mu.Lock()
	h.shared.buf = append(h.shared.buf, entry)
	full := len(h.shared.buf) >= logBufSize
	h.shared.mu.Unlock()

	if full {
		h.Flush()
	}

	return nil
}

// WithAttrs returns a new handler with the given attrs pre-set.
// The child shares the parent's buffer (via shared pointer) so all handlers
// derived from the same root flush to the same DB batch.
func (h *DBLogHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &DBLogHandler{
		shared: h.shared,
		stdout: h.stdout.WithAttrs(attrs),
		level:  h.level,
		attrs:  append(slices.Clone(h.attrs), attrs...),
		groups: h.groups,
	}
}

// WithGroup returns a new handler with the given group name.
func (h *DBLogHandler) WithGroup(name string) slog.Handler {
	return &DBLogHandler{
		shared: h.shared,
		stdout: h.stdout.WithGroup(name),
		level:  h.level,
		attrs:  h.attrs,
		groups: append(slices.Clone(h.groups), name),
	}
}

// Flush writes buffered logs to the database.
func (h *DBLogHandler) Flush() {
	h.shared.mu.Lock()
	if len(h.shared.buf) == 0 {
		h.shared.mu.Unlock()
		return
	}
	toFlush := h.shared.buf
	h.shared.buf = make([]RunLog, 0, logBufSize)
	h.shared.mu.Unlock()

	_ = h.shared.db.InsertLogs(context.Background(), toFlush)
}

// Stop stops the flush loop and flushes remaining logs.
func (h *DBLogHandler) Stop() {
	h.shared.stopOnce.Do(func() {
		close(h.shared.stopCh)
		<-h.shared.doneCh
	})
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./storage/ -run TestDBLogHandler -v`
Expected: PASS

- [ ] **Step 5: Write test for run_id context propagation**

```go
func TestDBLogHandlerRunID(t *testing.T) {
	db := testutil.NewTestDB(t)

	handler := storage.NewDBLogHandler(db, slog.LevelInfo)
	logger := slog.New(handler)

	ctx := storage.ContextWithRunID(context.Background(), "run-abc")
	logger.InfoContext(ctx, "pipeline step", "hunt", "powder")

	handler.Flush()

	logs, _ := db.RecentLogs(context.Background(), "powder", "", 100)
	if len(logs) != 1 {
		t.Fatalf("expected 1 log, got %d", len(logs))
	}
	if logs[0].RunID != "run-abc" {
		t.Errorf("expected run_id run-abc, got %q", logs[0].RunID)
	}
}
```

- [ ] **Step 6: Run test to verify it passes**

Run: `go test ./storage/ -run TestDBLogHandlerRunID -v`
Expected: PASS

- [ ] **Step 7: Commit**

```bash
git add storage/loghandler.go storage/loghandler_test.go
git commit -m "feat: add slog handler that writes to both stdout and SQLite"
```

---

### Task 4: Concurrency Guard

A shared component that prevents multiple hunts from running simultaneously (avoids SQLite single-connection contention).

**Files:**
- Create: `pipeline/runguard.go`
- Create: `pipeline/runguard_test.go`

- [ ] **Step 1: Write failing test**

```go
// pipeline/runguard_test.go
package pipeline_test

import (
	"testing"

	"github.com/seanmeyer/opportunity-hunter/pipeline"
)

func TestRunGuard(t *testing.T) {
	g := pipeline.NewRunGuard()

	if !g.TryAcquire("comedy") {
		t.Fatal("should acquire comedy")
	}

	// Same hunt: blocked
	if g.TryAcquire("comedy") {
		t.Fatal("should not acquire comedy twice")
	}

	// Different hunt: also blocked (global guard)
	if g.TryAcquire("powder") {
		t.Fatal("should not acquire powder while comedy is running")
	}

	g.Release("comedy")

	// Now powder should work
	if !g.TryAcquire("powder") {
		t.Fatal("should acquire powder after comedy released")
	}

	if g.Running() != "powder" {
		t.Errorf("expected running=powder, got %q", g.Running())
	}

	g.Release("powder")

	if g.Running() != "" {
		t.Errorf("expected empty, got %q", g.Running())
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./pipeline/ -run TestRunGuard -v`
Expected: FAIL

- [ ] **Step 3: Implement pipeline/runguard.go**

```go
// pipeline/runguard.go
package pipeline

import "sync"

// RunGuard prevents concurrent pipeline runs. Only one hunt can run at a time
// (across both scheduled and manual triggers) to avoid SQLite contention.
type RunGuard struct {
	mu      sync.Mutex
	running string // empty = idle, otherwise the hunt name
}

// NewRunGuard creates a new idle run guard.
func NewRunGuard() *RunGuard {
	return &RunGuard{}
}

// TryAcquire attempts to start a run for the given hunt.
// Returns true if acquired, false if another hunt is already running.
func (g *RunGuard) TryAcquire(huntName string) bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.running != "" {
		return false
	}
	g.running = huntName
	return true
}

// Release marks the current run as complete.
func (g *RunGuard) Release(huntName string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.running == huntName {
		g.running = ""
	}
}

// Running returns the name of the currently running hunt, or empty string.
func (g *RunGuard) Running() string {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.running
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./pipeline/ -run TestRunGuard -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add pipeline/runguard.go pipeline/runguard_test.go
git commit -m "feat: add RunGuard for global pipeline concurrency control"
```

---

### Task 5: Pipeline Run Recording Integration

Wire `InsertRun`/`FinishRun` into `pipeline.Run()` and pass run ID via context for log association.

**Files:**
- Modify: `pipeline/pipeline.go` (wrap Run with DB recording)
- Modify: `pipeline/pipeline_test.go` (verify run record is created)

- [ ] **Step 1: Write failing test**

Add to `pipeline/pipeline_test.go`:

```go
func TestRunRecordsPipelineRun(t *testing.T) {
	db := testutil.NewTestDB(t)
	notifier := &testutil.FakeNotifier{}
	ct := core.NewCostTracker(0, nil)
	p := pipeline.New(db, ct, notifier, core.ScanRegion{}, "")

	hunt := &fake.FakeHunt{
		HuntName: "comedy",
		Srcs: []core.Source{&fake.FakeSource{
			SourceName: "test",
			Items:      makeRawItems(3),
		}},
		Eval:   &testutil.FakeEvaluator{},
		Dedupe: func(r core.RawItem) string { return r.SourceID },
		Sched:  core.Schedule{ScanInterval: time.Hour},
	}

	p.Run(context.Background(), hunt)

	run, err := db.LatestRun(context.Background(), "comedy")
	if err != nil {
		t.Fatal(err)
	}
	if run == nil {
		t.Fatal("expected a pipeline run record")
	}
	if run.Scanned != 3 {
		t.Errorf("expected 3 scanned, got %d", run.Scanned)
	}
	if run.Status != "ok" {
		t.Errorf("expected ok status, got %q", run.Status)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./pipeline/ -run TestRunRecordsPipelineRun -v`
Expected: FAIL — pipeline.Run() does not write to pipeline_runs

- [ ] **Step 3: Modify pipeline.Run() to record runs**

In `pipeline/pipeline.go`, modify the `Run()` function. At the start, call `InsertRun`. At the end (deferred), call `FinishRun` with results. Inject run ID into context via `storage.ContextWithRunID`. Compute cost delta by snapshotting `costTracker.ForHunt(name)` before and after.

The key changes:
1. Extract trigger type from context: add a `triggerKey` context key in `storage/loghandler.go` alongside `runIDKey`. Add `ContextWithTrigger(ctx, trigger)` and `TriggerFromContext(ctx)` helpers. Default to `"scheduled"` if not set.
2. Add `trigger := storage.TriggerFromContext(ctx)` then `runID, _ := p.db.InsertRun(ctx, name, trigger)` near the top
3. Add `ctx = storage.ContextWithRunID(ctx, runID)`
4. Add a deferred function that calls `p.db.FinishRun()` with the accumulated result. Compute cost delta by snapshotting `p.costTracker.ForHunt(name)` before and after. Build the `RunResult` from the `HuntResult` fields.
5. Determine status from `result.Errors`: no errors = "ok", only warn-level = "warn", any error = "error"

The daemon's scheduled run path uses the default (`"scheduled"`). The web's `RunFunc` callback in `main.go` sets `ctx = storage.ContextWithTrigger(ctx, "manual")` before calling `pipe.Run(ctx, hunt)`.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./pipeline/ -run TestRunRecordsPipelineRun -v`
Expected: PASS

- [ ] **Step 5: Run full test suite to verify no regressions**

Run: `make test`
Expected: All tests pass

- [ ] **Step 6: Commit**

```bash
git add pipeline/pipeline.go pipeline/pipeline_test.go
git commit -m "feat: record pipeline runs in DB with run_id context propagation"
```

---

### Task 6: Smart Distance — Add DrivingMinutes to Venue

Add a `DrivingMinutes` field to `core.Venue` and populate both walking and driving distances during card loading.

**Files:**
- Modify: `core/opportunity.go` (add field to Venue)
- Modify: `web/web.go` (fetch both modes in loadCards)
- Modify: `web/web_test.go` (test distance display logic)

- [ ] **Step 1: Add DrivingMinutes field to Venue**

In `core/opportunity.go`, add to the Venue struct:

```go
DrivingMinutes int     // enriched from distance_cache (mode=driving)
```

- [ ] **Step 2: Write failing test for smart distance display**

```go
// In web/web_test.go
func TestSmartDistanceDisplay(t *testing.T) {
	tests := []struct {
		name     string
		walking  int
		driving  int
		wantText string
	}{
		{"close venue", 8, 3, "8 min walk"},
		{"far venue", 90, 25, "25 min drive"},
		{"boundary", 30, 12, "30 min walk"},
		{"just over", 31, 13, "13 min drive"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := web.FormatDistance(tt.walking, tt.driving)
			if got != tt.wantText {
				t.Errorf("got %q, want %q", got, tt.wantText)
			}
		})
	}
}
```

- [ ] **Step 3: Run test to verify it fails**

Run: `go test ./web/ -run TestSmartDistanceDisplay -v`
Expected: FAIL — FormatDistance does not exist

- [ ] **Step 4: Implement FormatDistance in web/web.go**

```go
// FormatDistance returns a human-friendly distance string.
// Shows walking for close venues (<=30 min), driving otherwise.
func FormatDistance(walkingMinutes, drivingMinutes int) string {
	if walkingMinutes <= 30 {
		return fmt.Sprintf("%d min walk", walkingMinutes)
	}
	return fmt.Sprintf("%d min drive", drivingMinutes)
}
```

- [ ] **Step 5: Add driving distance enrichment to pipeline**

In `pipeline/pipeline.go`, find the venue enrichment section (where walking distance is computed) and add a parallel call for driving mode. The distance client's `GetDistance` already accepts `"DRIVE"` as a mode, and the `distance_cache` table stores them as separate rows keyed on `(venue_id, home_address, mode)`.

Add after the existing walking distance save:
```go
if _, err := distClient.GetDistance(ctx, homeAddress, venue.Address, "DRIVE"); err == nil {
	// Result is cached automatically by the storage layer
}
```

If the pipeline does not directly call the distance client (distance is enriched in `web.go` `loadCards`), then add the driving fetch alongside the existing walking fetch in `loadCards`.

- [ ] **Step 6: Modify loadCards to read both cached distances**

In `web/web.go` `loadCards()`, after the existing walking distance fetch, add:

```go
if dist, err := s.db.GetDistance(ctx, venue.ID, s.homeAddress, "driving"); err == nil {
	venue.DrivingMinutes = dist.Minutes
}
```

- [ ] **Step 7: Run tests**

Run: `go test ./web/ -run TestSmartDistanceDisplay -v`
Expected: PASS

- [ ] **Step 8: Commit**

```bash
git add core/opportunity.go web/web.go web/web_test.go
git commit -m "feat: add smart distance display (walking vs driving threshold)"
```

---

### Task 7: Wire RunGuard and Manual Trigger into Web Server

Add `RunFunc` callback and `POST /run` handler to the web server. Wire the run guard into the daemon.

**Files:**
- Modify: `web/web.go` (add RunFunc, add handleRun, register route)
- Modify: `web/web_test.go` (test manual trigger)
- Modify: `cmd/opportunity-hunter/main.go` (wire RunFunc and RunGuard into daemon loop)

- [ ] **Step 1: Write failing test for POST /run**

```go
func TestHandleRun(t *testing.T) {
	db := testutil.NewTestDB(t)
	ctx := context.Background()

	var triggered string
	hunts := []web.HuntInfo{{Name: "comedy"}, {Name: "powder"}}
	srv, err := web.New(db, hunts, "", func(ctx context.Context, hunt string) {
		triggered = hunt
	})
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	resp, err := http.PostForm(ts.URL+"/run", url.Values{"hunt": {"comedy"}})
	if err != nil {
		t.Fatal(err)
	}
	// Should redirect
	if resp.StatusCode != http.StatusOK { // after redirect
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	// Give goroutine a moment
	time.Sleep(50 * time.Millisecond)
	if triggered != "comedy" {
		t.Errorf("expected comedy triggered, got %q", triggered)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./web/ -run TestHandleRun -v`
Expected: FAIL — web.New() signature mismatch

- [ ] **Step 3: Add RunFunc to web.Server and implement handleRun**

Modify `web.New()` to accept an optional `RunFunc`:

```go
func New(db *storage.DB, hunts []HuntInfo, homeAddress string, runFunc ...func(context.Context, string)) (*Server, error)
```

Store as `s.runFunc` (nil if not provided). Add `handleRun`:

```go
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
```

Register the route in `Handler()`.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./web/ -run TestHandleRun -v`
Expected: PASS

- [ ] **Step 5: Update existing web tests to match new New() signature**

Existing tests call `web.New(db, hunts, "")` — this still works since `runFunc` is variadic. Verify:

Run: `go test ./web/ -v`
Expected: All pass

- [ ] **Step 6: Wire RunGuard into daemon loop in main.go**

In `cmd/opportunity-hunter/main.go` `runDaemon()`:
1. Create `guard := pipeline.NewRunGuard()`
2. Wrap the scheduled run with guard: `if !guard.TryAcquire(name) { continue }` / `defer guard.Release(name)`
3. Pass a `RunFunc` to `web.New()` that also uses the guard

- [ ] **Step 7: Run full test suite**

Run: `make test`
Expected: All pass

- [ ] **Step 8: Commit**

```bash
git add web/web.go web/web_test.go cmd/opportunity-hunter/main.go
git commit -m "feat: add manual run trigger with global concurrency guard"
```

---

### Task 8: Dark Theme — Layout Template

Rewrite `layout.html` with the dark theme, System Status button, and updated nav styling.

**Files:**
- Modify: `web/templates/layout.html`
- Modify: `web/web.go` (add `IsStatusPage` to template data, register `/status` route)

- [ ] **Step 1: Rewrite layout.html**

Replace the entire `layout.html` with dark theme CSS matching the mockup in `.superpowers/brainstorm/50671-1774500580/style-dense.html`. Key changes:

- Background: `#1a1a2e`, header: `#12121f`, cards: `#222238`
- Purple accents (`#6366f1`, `#a5b4fc`), warm text grays
- System Status button in header right (with health dot)
- Nav pills with rounded corners, darker inactive state
- All existing CSS classes preserved but restyled for dark theme
- `border-radius: 8px` on cards, `6px` on buttons

Reference the mockup for exact colors and spacing. Keep all CSS inline in the `<style>` block.

- [ ] **Step 2: Add IsStatusPage and HasRunFunc to template data**

In `web/web.go`, add fields to `pageData`:

```go
IsStatusPage bool
HasRunFunc   bool
```

Set `HasRunFunc: s.runFunc != nil` in `handleIndex`.

- [ ] **Step 3: Register /status route**

In `Handler()`, add: `mux.HandleFunc("/status", s.handleStatus)`

Implement a stub `handleStatus` that renders a basic `status.html` template (to be fleshed out in Task 10).

- [ ] **Step 4: Run tests and visually verify**

Run: `make test`
Expected: All pass

Run: `go run ./cmd/opportunity-hunter/ web` and open http://localhost:8080
Expected: Dark themed UI with hunt tabs

- [ ] **Step 5: Commit**

```bash
git add web/templates/layout.html web/web.go
git commit -m "feat: dark theme layout with System Status button"
```

---

### Task 9: Dense Cards Template

Rewrite `cards.html` with the information-dense card layout, per-hunt toolbar enhancements, smart distance, and urgency badges.

**Files:**
- Modify: `web/templates/cards.html`
- Modify: `web/web.go` (add LatestRun and schedule data to page context)

- [ ] **Step 1: Add run status data to handleIndex**

In `web/web.go` `handleIndex()`, load the latest run for the active hunt:

```go
latestRun, _ := s.db.LatestRun(ctx, activeHunt)
```

Pass to template as `.LatestRun`.

- [ ] **Step 2: Rewrite cards.html**

Replace `cards.html` with the dense layout matching the mockup. Key changes:

- **Toolbar:** Sort/filter left, run status + "Run Now" right. Show `LatestRun` info (relative time, counts). Show next scan from schedule. "Run Now" is ghost/outline style (border only, not filled). Hidden when `HasRunFunc` is false.
- **Cards:** Score badge left, title + date inline right, reason below, venue/price/distance/urgency all inline. Use `FormatDistance` for smart distance. Urgency rendered as yellow warning badge. `<details>` for full fields. Feedback thumbs + action link on same row.
- **Empty state:** Styled for dark theme.

Use the mockup in `.superpowers/brainstorm/50671-1774500580/style-dense.html` as the reference.

- [ ] **Step 3: Register template FuncMap and add helper functions**

In `web/web.go` `New()`, change the template parsing from:
```go
tmpl, err := template.New("").ParseFS(templateFS, "templates/*.html")
```
To:
```go
funcMap := template.FuncMap{
	"relativeTime":  relativeTime,
	"formatDistance": FormatDistance,
}
tmpl, err := template.New("").Funcs(funcMap).ParseFS(templateFS, "templates/*.html")
```

**IMPORTANT:** `Funcs()` MUST be called before `ParseFS()` — templates that reference these functions will fail to parse otherwise.

Add `relativeTime` function that converts a `time.Time` to "2m ago", "3h ago", "yesterday", etc. Add `FormatDistance` (already implemented in Task 6).

- [ ] **Step 4: Run tests and visually verify**

Run: `make test`
Expected: All pass

Run: `go run ./cmd/opportunity-hunter/ web` and open http://localhost:8080
Expected: Dense dark cards with inline details

- [ ] **Step 5: Commit**

```bash
git add web/templates/cards.html web/web.go
git commit -m "feat: information-dense card layout with per-hunt run status"
```

---

### Task 10: System Status Dashboard Template and Handler

Build the `/status` page with hunt health cards, cost bar, run history table, and log viewer.

**Files:**
- Create: `web/templates/status.html`
- Modify: `web/web.go` (implement handleStatus with all dashboard data)

- [ ] **Step 1: Define status data types and implement handleStatus**

Add these types to `web/web.go`:

```go
type huntHealth struct {
	Name     string
	Run      *storage.PipelineRun // nil if no runs yet
	Schedule *storage.ScheduleRow // nil if no schedule
}

type statusData struct {
	Hunts        []string
	ActiveHunt   string // empty on status page
	IsStatusPage bool
	HasRunFunc   bool
	Health       []huntHealth
	Costs        storage.MonthlySpendResult
	Runs         []storage.PipelineRun
	Logs         []storage.RunLog
	HuntFilter   string
	LevelFilter  string
}
```

**Template switching:** The `layout.html` template conditionally renders either `cards` or `status` content based on `.IsStatusPage`:
```html
{{if .IsStatusPage}}
  {{template "status" .}}
{{else}}
  {{template "content" .}}
{{end}}
```

Both `pageData` (for cards) and `statusData` (for status) must have the shared fields that `layout.html` needs: `Hunts`, `ActiveHunt`, `IsStatusPage`, `HasRunFunc`. Handle nil `Schedule` in the template with `{{if .Schedule}}`.

Implement `handleStatus()`:

```go
func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	huntFilter := r.URL.Query().Get("hunt")
	levelFilter := r.URL.Query().Get("level")

	// Hunt health: latest run per hunt
	var healthCards []huntHealth
	for _, name := range s.huntNames {
		run, _ := s.db.LatestRun(ctx, name)
		sched, _ := s.db.GetSchedule(ctx, name)
		healthCards = append(healthCards, huntHealth{
			Name:     name,
			Run:      run,
			Schedule: sched,
		})
	}

	// Costs
	costs, _ := s.db.MonthlySpend(ctx, time.Now())

	// Run history (filtered)
	runs, _ := s.db.RecentRuns(ctx, huntFilter, 50)

	// Logs (filtered)
	logs, _ := s.db.RecentLogs(ctx, huntFilter, levelFilter, 200)

	data := statusData{
		Hunts:       s.huntNames,
		Health:      healthCards,
		Costs:       costs,
		Runs:        runs,
		Logs:        logs,
		HuntFilter:  huntFilter,
		LevelFilter: levelFilter,
		IsStatusPage: true,
	}

	s.tmpl.ExecuteTemplate(w, "layout.html", data)
}
```

- [ ] **Step 2: Create status.html template**

Build `web/templates/status.html` matching the dashboard mockup in `.superpowers/brainstorm/50671-1774500580/dashboard.html`. Sections:

1. **Hunt health cards** — 4-column grid, colored top border (green/yellow/red), name, status text, last run relative time, counts, next scan
2. **Cost bar** — month name, total spend, per-hunt breakdown, budget indicator (if any budget defined)
3. **Run history table** — hunt filter pills, table with Time/Hunt/Result/Scanned/Evaluated/Notified/Duration/Cost columns, hover highlight
4. **Log viewer** — severity pills (All/Errors/Warnings) + hunt pills, monospace output, color-coded levels

All filter pills are links that set query params (`?hunt=comedy&level=warn`), keeping the server-rendered approach.

- [ ] **Step 3: Write test for /status endpoint**

```go
func TestStatusPage(t *testing.T) {
	srv, ts := newTestServer(t)
	resp, err := http.Get(ts.URL + "/status")
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 200 {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
	body := readBody(t, resp)
	if !strings.Contains(body, "System Status") {
		t.Error("expected status page content")
	}
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./web/ -v`
Expected: All pass

- [ ] **Step 5: Visually verify**

Run: `go run ./cmd/opportunity-hunter/ web` and open http://localhost:8080/status
Expected: Dashboard with health cards, cost bar, run history, log viewer

- [ ] **Step 6: Commit**

```bash
git add web/templates/status.html web/web.go
git commit -m "feat: system status dashboard with run history, costs, and log viewer"
```

---

### Task 11: Wire slog Handler and Log Pruning into Daemon

Replace the existing JSON slog handler with the new DB-tee handler in the daemon, and add hourly log pruning.

**Files:**
- Modify: `cmd/opportunity-hunter/main.go` (swap handler, add pruning goroutine)

- [ ] **Step 1: Replace slog handler in runDaemon**

In `cmd/opportunity-hunter/main.go`, replace:

```go
logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
```

With:

```go
logHandler := storage.NewDBLogHandler(db, slog.LevelInfo)
defer logHandler.Stop()
slog.SetDefault(slog.New(logHandler))
```

- [ ] **Step 2: Add log pruning goroutine**

After the main ticker setup, add:

```go
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
```

- [ ] **Step 3: Run full test suite**

Run: `make test`
Expected: All pass

- [ ] **Step 4: Build and smoke test**

Run: `make build && DRY_RUN=true GOOGLE_API_KEY=fake DB_PATH=:memory: ./opportunity-hunter run`
Expected: Starts up, logs appear in stdout as JSON, graceful shutdown on ctrl-c

- [ ] **Step 5: Commit**

```bash
git add cmd/opportunity-hunter/main.go
git commit -m "feat: wire DB log handler and hourly log pruning into daemon"
```

---

### Task 12: Docker Image Rebuild and Tag

Build a new Docker image with all changes and push a new release.

**Files:**
- No code changes — release process only

- [ ] **Step 1: Run full test suite one final time**

Run: `make test`
Expected: All pass

- [ ] **Step 2: Verify Docker builds**

Run: `docker build -t opportunity-hunter:test .`
Expected: Build succeeds

- [ ] **Step 3: Smoke test the container**

Run: `docker run --rm -e DRY_RUN=true -e GOOGLE_API_KEY=fake -e DB_PATH=/data/test.db -p 8080:8080 opportunity-hunter:test run`
Expected: Container starts, web UI accessible at http://localhost:8080 with dark theme

- [ ] **Step 4: Commit any remaining changes and tag**

```bash
git tag v0.2.0
git push origin main --tags
```

The GitHub Actions release workflow will build and push the multi-arch image to `ghcr.io/seanmeyer/opportunity-hunter:v0.2.0` and `:latest`.

- [ ] **Step 5: Update Unraid container**

On the Unraid server, click the container icon > Update. The new image with all UI changes will be pulled.
