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

type contextKey string

const runIDKey contextKey = "run_id"

// ContextWithRunID returns a context carrying the given run ID for log association.
func ContextWithRunID(ctx context.Context, runID string) context.Context {
	return context.WithValue(ctx, runIDKey, runID)
}

const triggerKey contextKey = "trigger"

// ContextWithTrigger returns a context carrying a trigger type ("scheduled" or "manual").
func ContextWithTrigger(ctx context.Context, trigger string) context.Context {
	return context.WithValue(ctx, triggerKey, trigger)
}

// TriggerFromContext returns the trigger from context, defaulting to "scheduled".
func TriggerFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(triggerKey).(string); ok {
		return v
	}
	return "scheduled"
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

func (h *DBLogHandler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= h.level
}

func (h *DBLogHandler) Handle(ctx context.Context, r slog.Record) error {
	_ = h.stdout.Handle(ctx, r)

	var huntName, runID string
	attrs := make(map[string]any)

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

func (h *DBLogHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &DBLogHandler{
		shared: h.shared,
		stdout: h.stdout.WithAttrs(attrs),
		level:  h.level,
		attrs:  append(slices.Clone(h.attrs), attrs...),
		groups: h.groups,
	}
}

func (h *DBLogHandler) WithGroup(name string) slog.Handler {
	return &DBLogHandler{
		shared: h.shared,
		stdout: h.stdout.WithGroup(name),
		level:  h.level,
		attrs:  h.attrs,
		groups: append(slices.Clone(h.groups), name),
	}
}

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

func (h *DBLogHandler) Stop() {
	h.shared.stopOnce.Do(func() {
		close(h.shared.stopCh)
		<-h.shared.doneCh
	})
}
