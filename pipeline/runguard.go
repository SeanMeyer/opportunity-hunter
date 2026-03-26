package pipeline

import "sync"

// RunGuard prevents concurrent pipeline runs. Only one hunt can run at a time
// (across both scheduled and manual triggers) to avoid SQLite contention.
type RunGuard struct {
	mu      sync.Mutex
	running string // empty = idle, otherwise the hunt name
}

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
