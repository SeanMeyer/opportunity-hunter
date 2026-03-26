# Web UI Overhaul — Design Spec

## Problem

The web UI lacks operational visibility. There is no persistent run history, no log viewer, no way to trigger manual runs, and the current status banner is in-memory only (lost on restart). The visual style is also dated — large white cards on gray, limited information density, and walking-only distance that produces absurd results for distant venues.

## Goals

1. See at a glance whether hunts are running, when they last ran, and whether they succeeded
2. View structured logs filtered by hunt and severity
3. Trigger a manual pipeline run for any individual hunt
4. Surface cost tracking (total and per-hunt) in the UI
5. Show smarter distance information (walking vs driving)
6. Add urgency alerts to cards for time-sensitive opportunities
7. Modernize the visual style — dark, information-dense, approachable

## Non-Goals

- Real-time log streaming (polling on page load is sufficient)
- Multi-user auth or access control
- Mobile-specific responsive layout (desktop-first, readable on tablet)
- Changing the pipeline logic itself (scan, eval, notify steps stay the same)

## Design

### 1. Visual Overhaul

**Theme:** Dark, information-dense, with warm accent colors.

- **Background:** Dark navy (`#1a1a2e`)
- **Cards/panels:** Darker panels (`#222238`)
- **Primary accent:** Indigo/purple (`#6366f1`, `#a5b4fc`)
- **Text:** Light grays (`#f3f4f6` primary, `#9ca3af` secondary, `#6b7280` muted)
- **Success/warning/error:** Green `#86efac`, yellow `#fbbf24`, red `#ef4444`
- **Border radius:** 8px on cards, 6px on buttons/pills
- **Font sizes:** Compact — 0.7-0.85rem for most content, 0.92rem for card titles

**Header:** Dark background (`#12121f`), app title left, "System Status" button right (with green/yellow/red dot indicating overall health). Hunt tabs below as pills.

**Cards:** Horizontal layout with score badge on the left edge, title + date on one line, reason text below, then venue/price/distance/urgency inline. Expandable `<details>` for the full field list. Feedback thumbs and action link on the same bottom row. Approximately 5 cards visible without scrolling.

**Toolbar:** Dark panel matching the theme. Sort/filter on the left, per-hunt run status + "Run Now" button on the right. "Run Now" uses a subdued ghost/outline style (not bright).

### 2. Per-Hunt Toolbar Enhancements

The existing toolbar (sort, filter, gear) gains three new elements on the right side:

- **Run status indicator:** Green/yellow/red dot + "Ran 12m ago · 4 scanned, 2 notified"
- **Next scan time:** "Next: 5:30 PM"
- **Run Now button:** Outline/ghost style. Triggers a manual pipeline run for this hunt only.

Data source: The run status comes from the new `pipeline_runs` table (see section 6) via a new `db.LatestRun(ctx, huntName)` query. The next scan time comes from the existing `hunt_schedules.next_scan_at` column. When no runs exist yet for a hunt (first start or newly enabled), show "No runs yet" in place of the status indicator.

### 3. System Status Dashboard

Accessed via the "System Status" button in the header. This is a separate page (`/status`) that shows cross-cutting system information. The full header (title + hunt tabs) remains visible, so users can navigate back to any hunt tab directly. The "System Status" button shows as active/highlighted when on this page.

**Layout (top to bottom):**

#### Hunt Health Cards
A 4-column grid (one per enabled hunt) showing:
- Hunt name
- Status (Healthy / N warnings / Error) with color-coded top border
- Last run time (relative)
- Scanned/notified counts from last run
- Next scheduled run

#### Cost Bar
A single row showing:
- Current month name
- Total monthly spend (large text)
- Per-hunt breakdown inline
- Monthly budget indicator

Data source: Existing `MonthlySpend()` query on `eval_costs` table (already returns `Total` and `ByHunt` map). Budget comes from per-hunt `Schedule.MaxMonthlySpendUSD` (currently only set by powder hunt). The cost bar sums all per-hunt budgets that are non-nil to show a total budget. If no hunts define a budget, omit the budget indicator entirely.

#### Run History Table
Columns: Time, Hunt, Result, Scanned, Evaluated, Notified, Duration, Cost.

- Filterable by hunt (pill buttons above the table)
- Clickable rows expand to show that run's logs inline
- Shows the last ~50 runs
- Result column: green checkmark "OK", yellow warning "Warn", red X "Error"

Data source: New `pipeline_runs` table.

#### Log Viewer
Monospace log output styled like a terminal. Filterable by:
- Severity: All / Errors / Warnings (pill buttons)
- Hunt: All hunts / specific hunt (pill buttons)

Shows the most recent ~200 log entries. Newest at top. Color-coded: green INFO, yellow WARN, red ERROR. Each line shows timestamp, level, hunt tag, message.

Data source: New `run_logs` table.

### 4. Smart Distance Display

Currently the UI only shows walking distance, producing results like "180 min walk" for distant venues.

**New behavior:**
- If walking time <= 30 minutes: show walking icon + "N min walk"
- If walking time > 30 minutes: show driving icon + "N min drive"
- The distance client already supports both modes (`WALK` and `DRIVE`)

**Implementation:** Extend the pipeline's enrichment step (not the web layer) to compute both walking and driving distances during scan. The `distance_cache` table already has a `mode` column with a unique constraint on `(venue_id, home_address, mode)`, so walking and driving are stored as separate rows. The distance client already supports both modes (`WALK` and `DRIVE`).

During card enrichment in `web.go`, read both cached values from storage. Add a `DrivingMinutes` field to `core.Venue` alongside the existing `WalkingMinutes`. Display logic selects which to show based on the 30-minute walking threshold. No Google Routes API calls happen during page loads — all distance data is pre-populated by the pipeline.

### 5. Urgency Alerts

A new concept for time-sensitive callouts on cards. Displayed as a warning-colored inline badge (yellow text with warning triangle icon).

**Examples:** "Presale ends Friday", "Last 50 tickets", "Added today", "Selling fast"

**Data model:** Urgency is already a field on `core.CardData` (the `Urgency string` field in `core/web.go`). Today it's populated by some hunts during evaluation. The change is primarily visual — the current rendering is plain text; the new rendering is a styled badge.

Hunts can populate urgency during scanning (from source data like ticket availability) or during evaluation (LLM can flag time-sensitive picks). No schema change needed.

### 6. Backend: Pipeline Run History

**New table: `pipeline_runs`**

| Column | Type | Description |
|--------|------|-------------|
| id | TEXT PK | UUID, serves as the run_id |
| hunt_name | TEXT | Which hunt this run was for |
| started_at | TEXT | RFC3339 timestamp |
| finished_at | TEXT | RFC3339 timestamp |
| status | TEXT | "ok", "warn", "error" |
| scanned | INTEGER | Count of opportunities scanned |
| evaluated | INTEGER | Count evaluated |
| notified | INTEGER | Count notified |
| cost_usd | REAL | Cost of this run's LLM calls |
| error_summary | TEXT | Nullable, error message if status != ok |
| trigger | TEXT | "scheduled" or "manual" |

**Index:** `(hunt_name, started_at)`

The pipeline's `Run()` function creates a run record at start, updates it at completion. The `cost_usd` is computed by snapshotting `costTracker.ForHunt(name)` before and after the run and taking the delta. The run ID is passed through the pipeline context so logs can be associated.

### 7. Backend: Structured Log Storage

**New table: `run_logs`**

| Column | Type | Description |
|--------|------|-------------|
| id | INTEGER PK | Auto-increment |
| run_id | TEXT | FK to pipeline_runs.id, nullable (some logs are outside runs) |
| hunt_name | TEXT | Nullable (system-wide logs have no hunt) |
| timestamp | TEXT | RFC3339 |
| level | TEXT | "info", "warn", "error" |
| message | TEXT | Log message |
| attrs | TEXT | JSON blob of structured slog fields |

**Index:** `(hunt_name, timestamp)`, `(run_id)`

**Implementation:** Add a custom `slog.Handler` that writes to both stdout (existing JSON handler) and the `run_logs` table. The handler extracts the `hunt` attribute from structured log fields to populate `hunt_name`. A `run_id` is injected into the logger context at the start of each pipeline run.

**Buffering:** The handler buffers up to 50 log entries and flushes on a 2-second interval or when the buffer is full (whichever comes first). A `Flush()` method is called on graceful shutdown to avoid losing tail logs. The buffer does not block callers — if the DB write fails, logs are still written to stdout.

**Retention:** Prune logs older than 7 days once per hour (tracked by a timestamp in the pruning goroutine, not on every daemon tick). Keeps the DB from growing unbounded.

### 8. Backend: Manual Run Trigger

**New endpoint: `POST /run`**

Parameters:
- `hunt` (required): Hunt name to trigger

Behavior:
1. Validate the hunt name exists and is enabled
2. If a run is already in progress for this hunt, return a redirect with a flash message
3. Otherwise, launch `pipe.Run(ctx, hunt)` in a goroutine
4. Redirect back to `/?hunt={hunt}` immediately

The web server needs a way to trigger runs without depending on the pipeline package directly. `web.New()` gains a new optional parameter: `RunFunc func(ctx context.Context, huntName string)`. This callback is set by the daemon; the web layer stays decoupled from the pipeline. When `RunFunc` is nil (e.g., in `web`-only mode), the "Run Now" button is hidden.

**Concurrency guard:** A global `sync.Mutex`-guarded set of hunt names currently running. This is shared between the daemon's scheduled runs and the web's manual trigger — only one hunt runs at a time across both. This avoids SQLite contention since `MaxOpenConns(1)`. The guard is checked before launching a goroutine; if busy, the handler redirects with a flash message ("already running"). Cleared when the goroutine completes.

### 9. New Routes Summary

| Route | Method | Purpose |
|-------|--------|---------|
| `/status` | GET | System status dashboard |
| `/run` | POST | Trigger manual pipeline run for a hunt |

Existing routes (`/`, `/preferences`, `/feedback`, `/schedule`) are unchanged.

### 10. Template Changes

The current two-template system (`layout.html` + `cards.html`) gains:
- **`status.html`**: The dashboard template (hunt health cards, cost bar, run history, log viewer)
- **`layout.html`**: Updated with dark theme CSS, System Status button in header
- **`cards.html`**: Updated card markup (dense horizontal layout, smart distance, urgency badges, inline details)

All CSS remains inline in `layout.html` (no external CSS files — keeps the single-binary deployment simple).

### 11. Database Migrations

Two new tables added as migrations in `sqlite.go`, following the existing `ALTER TABLE` migration pattern:

1. `CREATE TABLE IF NOT EXISTS pipeline_runs (...)`
2. `CREATE TABLE IF NOT EXISTS run_logs (...)`

Both are additive — no existing tables are modified.

## Testing

- **Run history:** Integration test that calls `pipeline.Run()` and verifies a `pipeline_runs` row is created with correct counts
- **Log storage:** Test that the custom slog handler writes to the DB and that hunt_name/run_id are extracted correctly
- **Manual trigger:** HTTP test that POSTs to `/run` and verifies a run is started (using the fake hunt)
- **Smart distance:** Unit test for the threshold logic (walking vs driving selection)
- **Dashboard queries:** Test the new storage methods (recent runs, recent logs with filters)
- **Visual:** Manual verification against the mockups in `.superpowers/brainstorm/`

## Risks

- **Log volume:** SQLite writes on every log line could slow the pipeline. Mitigation: batch inserts (buffer N logs, flush periodically) and 7-day retention pruning.
- **Manual run concurrency:** A manual run overlapping with a scheduled run could cause SQLite contention (`MaxOpenConns(1)`). Mitigation: a global concurrency guard shared between the daemon scheduler and the web trigger ensures only one hunt runs at a time across both paths.
- **DB size:** Log retention of 7 days with 4 hunts running every 12 hours produces ~few hundred rows/day. Negligible.

## Mockups

Visual mockups are in `.superpowers/brainstorm/50671-1774500580/`:
- `style-dense.html` — Card layout and dark theme direction (approved)
- `dashboard.html` — System status dashboard layout (approved)
- `status-access.html` — Header button placement options (option A selected)
