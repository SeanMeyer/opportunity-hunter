-- Venues are shared across hunts, normalized on upsert.
CREATE TABLE IF NOT EXISTS venues (
    id        INTEGER PRIMARY KEY AUTOINCREMENT,
    name      TEXT NOT NULL,
    address   TEXT NOT NULL DEFAULT '',
    latitude  REAL NOT NULL DEFAULT 0,
    longitude REAL NOT NULL DEFAULT 0,
    notes     TEXT NOT NULL DEFAULT '',
    UNIQUE(name, address)
);

-- Opportunities are the universal entity produced by all hunts.
CREATE TABLE IF NOT EXISTS opportunities (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    hunt_name     TEXT NOT NULL,
    source_id     TEXT NOT NULL,
    source        TEXT NOT NULL,
    title         TEXT NOT NULL,
    subtitle      TEXT NOT NULL DEFAULT '',
    venue_id      INTEGER REFERENCES venues(id),
    start_time    TEXT NOT NULL,
    end_time      TEXT,
    price_min     REAL,
    price_max     REAL,
    ticket_url    TEXT NOT NULL DEFAULT '',
    state         TEXT NOT NULL DEFAULT 'discovered',
    attributes    TEXT NOT NULL DEFAULT '{}' CHECK(json_valid(attributes)),
    raw_data      TEXT NOT NULL DEFAULT '{}',
    discovered_at TEXT NOT NULL,
    evaluated_at  TEXT,
    notified_at   TEXT,
    reminded_at   TEXT
);

-- Evaluations store LLM analysis results.
CREATE TABLE IF NOT EXISTS evaluations (
    id                INTEGER PRIMARY KEY AUTOINCREMENT,
    hunt_name         TEXT NOT NULL,
    group_key         TEXT NOT NULL,
    evaluated_at      TEXT NOT NULL,
    skipped_reasoning TEXT NOT NULL DEFAULT '',
    raw_llm_response  TEXT NOT NULL DEFAULT '',
    rendered_prompt   TEXT NOT NULL DEFAULT '',
    cost_usd          REAL NOT NULL DEFAULT 0
);

-- Normalized picks table with foreign keys.
CREATE TABLE IF NOT EXISTS picks (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    evaluation_id   INTEGER NOT NULL REFERENCES evaluations(id),
    opportunity_id  INTEGER NOT NULL REFERENCES opportunities(id),
    score           REAL NOT NULL DEFAULT 0,
    display_score   TEXT NOT NULL DEFAULT '',
    reason          TEXT NOT NULL DEFAULT '',
    urgency         TEXT NOT NULL DEFAULT '',
    attributes      TEXT NOT NULL DEFAULT '{}' CHECK(json_valid(attributes))
);

-- User feedback on opportunities. opportunity_id nullable for manual feedback (movies).
CREATE TABLE IF NOT EXISTS feedback (
    id             INTEGER PRIMARY KEY AUTOINCREMENT,
    opportunity_id INTEGER REFERENCES opportunities(id),
    hunt_name      TEXT NOT NULL,
    title          TEXT NOT NULL,
    rating         TEXT NOT NULL,
    note           TEXT NOT NULL DEFAULT '',
    created_at     TEXT NOT NULL
);

-- Per-hunt user preferences.
CREATE TABLE IF NOT EXISTS preferences (
    id               INTEGER PRIMARY KEY AUTOINCREMENT,
    hunt_name        TEXT NOT NULL UNIQUE,
    preferences_text TEXT NOT NULL DEFAULT ''
);

-- Distance cache supports multiple travel modes.
CREATE TABLE IF NOT EXISTS distance_cache (
    venue_id     INTEGER NOT NULL REFERENCES venues(id),
    home_address TEXT NOT NULL,
    mode         TEXT NOT NULL DEFAULT 'walking',
    minutes      INTEGER NOT NULL,
    distance_mi  REAL NOT NULL,
    created_at   TEXT NOT NULL,
    UNIQUE(venue_id, home_address, mode)
);

-- Thread tracking for Discord conversations.
CREATE TABLE IF NOT EXISTS notification_threads (
    hunt_name  TEXT NOT NULL,
    group_key  TEXT NOT NULL,
    thread_id  TEXT NOT NULL,
    created_at TEXT NOT NULL,
    UNIQUE(hunt_name, group_key)
);

-- Cost tracking for LLM API usage.
CREATE TABLE IF NOT EXISTS eval_costs (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    hunt_name    TEXT NOT NULL,
    evaluated_at TEXT NOT NULL,
    cost_usd     REAL NOT NULL,
    model        TEXT NOT NULL DEFAULT '',
    success      INTEGER NOT NULL DEFAULT 1
);

-- Prompt template versioning. Evaluator loads from here, falls back to hardcoded.
CREATE TABLE IF NOT EXISTS prompt_templates (
    hunt_name  TEXT NOT NULL,
    version    TEXT NOT NULL,
    template   TEXT NOT NULL,
    active     INTEGER NOT NULL DEFAULT 0,
    created_at TEXT NOT NULL,
    UNIQUE(hunt_name, version)
);

-- Structured user profiles. Empty hunt_name = global profile.
CREATE TABLE IF NOT EXISTS user_profiles (
    hunt_name      TEXT NOT NULL DEFAULT '',
    home_base      TEXT NOT NULL DEFAULT '',
    home_lat       REAL NOT NULL DEFAULT 0,
    home_lon       REAL NOT NULL DEFAULT 0,
    passes         TEXT NOT NULL DEFAULT '[]',
    skill_level    TEXT NOT NULL DEFAULT '',
    preferences    TEXT NOT NULL DEFAULT '',
    remote_work    INTEGER NOT NULL DEFAULT 0,
    pto_days       INTEGER NOT NULL DEFAULT 0,
    blackout_dates TEXT NOT NULL DEFAULT '[]',
    extra          TEXT NOT NULL DEFAULT '{}',
    UNIQUE(hunt_name)
);

-- Per-hunt scan schedule. Persists next_scan_at across restarts.
CREATE TABLE IF NOT EXISTS hunt_schedules (
    hunt_name       TEXT NOT NULL UNIQUE,
    scan_interval_m INTEGER NOT NULL,
    next_scan_at    TEXT NOT NULL,
    updated_at      TEXT NOT NULL
);

-- Pipeline run history for observability and web UI status display.
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

-- Indexes for common queries.
CREATE INDEX IF NOT EXISTS idx_opportunities_hunt_state ON opportunities(hunt_name, state);
CREATE INDEX IF NOT EXISTS idx_opportunities_start_time ON opportunities(start_time);
CREATE INDEX IF NOT EXISTS idx_evaluations_hunt_group ON evaluations(hunt_name, group_key);
CREATE INDEX IF NOT EXISTS idx_picks_evaluation ON picks(evaluation_id);
CREATE INDEX IF NOT EXISTS idx_picks_opportunity ON picks(opportunity_id);
CREATE INDEX IF NOT EXISTS idx_feedback_hunt ON feedback(hunt_name);
CREATE INDEX IF NOT EXISTS idx_eval_costs_hunt_date ON eval_costs(hunt_name, evaluated_at);
