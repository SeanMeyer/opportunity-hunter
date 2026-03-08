# Opportunity Hunter Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Build a unified framework for discovering, evaluating, and recommending opportunities — starting with core infrastructure, then implementing three hunts (performing arts, comedy, powder).

**Architecture:** Framework pipeline with Go interface plugin pattern. Core owns orchestration; hunts implement required interface (6 methods) plus optional interfaces. Sequential hunt execution, concurrent source scanning. SQLite persistence, Gemini LLM evaluation, Discord notifications, web UI.

**Tech Stack:** Go 1.24+, modernc.org/sqlite, google.golang.org/genai (Gemini), gocolly/colly (scraping), stdlib net/http, slog, embed

**Design doc:** `docs/plans/2026-03-08-opportunity-hunter-design.md`

**Reference codebases:**
- `/Users/sean.meyer/projects/comedy-hunter` — comedy patterns, sources, evaluation, web UI
- `/Users/sean.meyer/projects/powder-hunter` — weather pipeline, threading, briefing, re-evaluation, budget gating

---

## Phase 1: Core Framework

Build the foundation that all hunts depend on. No hunt-specific code yet — just types, interfaces, storage, pipeline skeleton, and test infrastructure.

### Task 1: Project scaffolding

**Files:**
- Create: `go.mod`, `Makefile`, `.env.example`, `.gitignore`, `CLAUDE.md`

**Step 1: Initialize Go module**

```bash
cd /Users/sean.meyer/projects/opportunity-hunter
go mod init github.com/seanmeyer/opportunity-hunter
```

**Step 2: Create Makefile**

Port from comedy-hunter's Makefile. Targets: `build`, `test`, `lint`, `run`.

```makefile
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")

build:
	CGO_ENABLED=0 go build -ldflags="-X main.version=$(VERSION)" -o opportunity-hunter ./cmd/opportunity-hunter/

test:
	go test ./...

lint:
	go vet ./...

run:
	go run ./cmd/opportunity-hunter/ run

.PHONY: build test lint run
```

**Step 3: Create .env.example**

Copy the Configuration section from the design doc verbatim.

**Step 4: Create .gitignore**

```
opportunity-hunter
*.db
*.db-wal
*.db-shm
.env
!docs/
```

Note: Update the existing `.gitignore` (currently just `!docs/`) with these entries.

**Step 5: Create CLAUDE.md**

Adapt from comedy-hunter's CLAUDE.md. Include project structure, key commands, and local testing instructions.

**Step 6: Commit**

```bash
git add -A
git commit -m "chore: project scaffolding with go.mod, Makefile, .env.example"
```

---

### Task 2: Core types — Opportunity, State, Venue, Attributes

**Files:**
- Create: `core/opportunity.go`
- Test: `core/opportunity_test.go`

**Step 1: Write tests for state transitions**

Test that `MarkEvaluated` sets both state and timestamp atomically. Test that all transition methods work. Test that `Validate()` catches invalid states (empty HuntName, EndTime before StartTime).

```go
func TestOpportunity_MarkEvaluated(t *testing.T) {
    opp := core.Opportunity{HuntName: "test", State: core.Discovered}
    now := time.Now()
    opp.MarkEvaluated(now)
    if opp.State != core.Evaluated { t.Fatalf("want Evaluated, got %s", opp.State) }
    if opp.EvaluatedAt == nil || !opp.EvaluatedAt.Equal(now) { t.Fatal("EvaluatedAt not set") }
}
```

**Step 2: Run tests — expect FAIL (types don't exist)**

```bash
go test ./core/ -v
```

**Step 3: Implement core/opportunity.go**

Types from design doc: `Attributes`, `State` constants, `Opportunity` struct with `Mark*` methods, `Venue` struct. Include `Validate() error` on Opportunity.

**Step 4: Run tests — expect PASS**

**Step 5: Commit**

```bash
git commit -m "feat: core opportunity, state, venue types with transition methods"
```

---

### Task 3: Core types — Evaluation, Pick, EvalContext

**Files:**
- Create: `core/evaluation.go`
- Test: `core/evaluation_test.go`

**Step 1: Write tests**

Test `EvalContext.Validate()` — checks non-empty opportunities, all venue IDs present in map, non-nil CostTracker. Test `Evaluation` with picks vs skipped mutual exclusivity if we add that validation.

**Step 2: Run tests — FAIL**

**Step 3: Implement core/evaluation.go**

Types: `Evaluator` interface, `EvalContext` with `Validate()`, `Evaluation`, `Pick`, `FeedbackEntry`, `Group`, `NotifyGroup`.

**Step 4: Run tests — PASS**

**Step 5: Commit**

```bash
git commit -m "feat: core evaluation, pick, eval context types"
```

---

### Task 4: Core types — Hunt interface + optional interfaces + ValidateHunt

**Files:**
- Create: `core/hunt.go`
- Test: `core/hunt_test.go`

**Step 1: Write tests for ValidateHunt**

Test that ValidateHunt returns no error for a minimal Hunt. Test that it logs which optional interfaces are implemented. (ValidateHunt currently has no co-dependency checks since we merged PostEvalGrouper+Synthesizer into Briefer, but it should still exist for future checks and logging.)

**Step 2: Run tests — FAIL**

**Step 3: Implement core/hunt.go**

Types: `Hunt` interface (6 methods), `Schedule`, `Grouper`, `ReEvaluator`, `Briefer`, `WebHunt`, `NotifyHunt`. Function: `ValidateHunt(h Hunt) error`.

**Step 4: Run tests — PASS**

**Step 5: Commit**

```bash
git commit -m "feat: core hunt interface with 5 optional interfaces and validation"
```

---

### Task 5: Core types — Source, Notification, Web, Cost, Pipeline result

**Files:**
- Create: `core/source.go`, `core/notify.go`, `core/web.go`, `core/cost.go`, `core/pipeline.go`, `core/retry.go`
- Test: `core/notify_test.go`, `core/cost_test.go`, `core/retry_test.go`

**Step 1: Write tests**

- `ValidateActions`: test that PostToThread with missing ThreadRef fails, valid sequences pass
- `CostTracker`: test Add, Total, ForHunt
- `withRetry`: test that it retries on error and succeeds on eventual success, stops after max attempts

**Step 2: Run tests — FAIL**

**Step 3: Implement all files**

- `core/source.go`: `Source` interface, `ScanRegion`, `RawItem`
- `core/notify.go`: `NotifyFormatter`, `NotifyAction`, `NotifyMessage`, `ActionType` constants, `ValidateActions`
- `core/web.go`: `CardRenderer`, `CardData`, `ScoreTier` constants, `FeedbackOption`, `CardField`
- `core/cost.go`: `CostTracker` with unexported fields, `NewCostTracker` (takes initial total + byHunt, NOT db — that coupling belongs in main), `Add`, `Total`, `ForHunt`
- `core/pipeline.go`: `PipelineResult`, `HuntResult`, `StepError`
- `core/retry.go`: `withRetry[T any]` generic helper

**Step 4: Run tests — PASS**

**Step 5: Commit**

```bash
git commit -m "feat: core source, notify, web, cost, pipeline result, retry types"
```

---

### Task 6: SQLite storage layer — schema + DB setup

**Files:**
- Create: `storage/sqlite.go`, `storage/schema.sql`
- Test: `storage/sqlite_test.go`

**Step 1: Write tests**

Test that `Open()` creates a DB, applies schema, and tables exist. Test that opening twice is idempotent.

**Step 2: Run tests — FAIL**

**Step 3: Implement storage**

- `storage/schema.sql`: Full schema from design doc. All tables with constraints, indexes, CHECK(json_valid) on attributes columns. Use `CREATE TABLE IF NOT EXISTS` for idempotency.
- `storage/sqlite.go`: `DB` struct wrapping `*sql.DB`. `Open(path)` with WAL mode, busy timeout, foreign keys, max 1 connection. Embed schema.sql via `//go:embed`.

Add dependency: `go get modernc.org/sqlite`

**Step 4: Run tests — PASS**

**Step 5: Commit**

```bash
git commit -m "feat: sqlite storage layer with embedded schema"
```

---

### Task 7: Storage — venues CRUD with normalization

**Files:**
- Create: `storage/venues.go`
- Test: `storage/venues_test.go`

**Step 1: Write tests**

- `TestUpsertVenue_Insert`: new venue is inserted
- `TestUpsertVenue_Update`: existing venue (by normalized name) is updated with new coordinates
- `TestUpsertVenue_Normalization`: "Comedy Works - Downtown" matches "Comedy Works Downtown"
- `TestGetVenue`: by ID
- `TestGetVenueByName`: normalized lookup

**Step 2: Run tests — FAIL**

**Step 3: Implement venues.go**

`normalizeVenueName(name) string` — lowercase, strip common punctuation (hyphens, extra spaces), trim. `UpsertVenue(ctx, Venue) (int64, error)` — normalize name, look up by normalized name, insert or update. `GetVenue`, `GetVenueByName`.

**Step 4: Run tests — PASS**

**Step 5: Commit**

```bash
git commit -m "feat: venue storage with name normalization and upsert"
```

---

### Task 8: Storage — opportunities CRUD

**Files:**
- Create: `storage/opportunities.go`
- Test: `storage/opportunities_test.go`

**Step 1: Write tests**

- Insert opportunity, retrieve by ID
- `GetByState(huntName, state)` returns only matching
- `OpportunityExists(huntName, dedupeKey)` returns true for duplicates
- `UpdateState(id, state)` transitions correctly
- State scoping by hunt_name works (comedy sees only comedy)

**Step 2: Run tests — FAIL**

**Step 3: Implement**

`InsertOpportunity`, `GetOpportunity`, `GetByState`, `OpportunityExists`, `UpdateState`, `GetUpcoming`. All queries scoped by `hunt_name`.

**Step 4: Run tests — PASS**

**Step 5: Commit**

```bash
git commit -m "feat: opportunity storage with hunt-scoped queries"
```

---

### Task 9: Storage — evaluations + picks CRUD

**Files:**
- Create: `storage/evaluations.go`, `storage/picks.go`
- Test: `storage/evaluations_test.go`

**Step 1: Write tests**

- Save evaluation, retrieve by ID
- Save evaluation with picks, retrieve picks by evaluation ID
- `GetLatestEvaluation(huntName, groupKey)` returns most recent
- Picks have proper FK references to evaluation and opportunity
- `GetPicksForOpportunity(opportunityID)` returns all picks across evaluations

**Step 2: Run tests — FAIL**

**Step 3: Implement**

`SaveEvaluation` (returns ID), `SavePick`, `SaveEvaluationWithPicks` (transaction), `GetEvaluation`, `GetLatestEvaluation`, `GetPicksForEvaluation`, `GetPicksForOpportunity`.

**Step 4: Run tests — PASS**

**Step 5: Commit**

```bash
git commit -m "feat: evaluation and normalized picks storage"
```

---

### Task 10: Storage — feedback, preferences, distance cache, threads, costs

**Files:**
- Create: `storage/feedback.go`, `storage/preferences.go`, `storage/distance.go`, `storage/threads.go`, `storage/costs.go`
- Test: `storage/feedback_test.go`, `storage/costs_test.go`

**Step 1: Write tests for feedback and costs**

- Save feedback, retrieve by opportunity ID
- `GetRecentFeedback(huntName, limit)` returns most recent
- `RecordCost` and `MonthlySpend` return correct totals
- `MonthlySpendByHunt` breaks down per hunt

**Step 2: Run tests — FAIL**

**Step 3: Implement all storage files**

- `feedback.go`: `SaveFeedback`, `GetFeedbackForOpportunity`, `GetRecentFeedback`
- `preferences.go`: `SavePreferences(huntName, text)`, `GetPreferences(huntName)`
- `distance.go`: `GetDistance(venueID, homeAddress, mode)`, `SaveDistance`, `GetDistanceBatch`
- `threads.go`: `GetThread(huntName, groupKey)`, `SaveThread` (with ON CONFLICT IGNORE)
- `costs.go`: `RecordCost`, `MonthlySpend`, `MonthlySpendByHunt`

**Step 4: Run tests — PASS**

**Step 5: Commit**

```bash
git commit -m "feat: feedback, preferences, distance, thread, and cost storage"
```

---

### Task 11: Test infrastructure — testutil + fake hunt

**Files:**
- Create: `testutil/db.go`, `testutil/evaluator.go`, `testutil/notifier.go`, `hunts/fake/hunt.go`

**Step 1: Implement testutil/db.go**

```go
func NewTestDB(t *testing.T) *storage.DB {
    t.Helper()
    db, err := storage.Open(filepath.Join(t.TempDir(), "test.db"))
    if err != nil { t.Fatal(err) }
    t.Cleanup(func() { db.Close() })
    return db
}
```

**Step 2: Implement testutil/evaluator.go**

`FakeEvaluator` that returns configurable picks or errors. Records calls for assertion.

**Step 3: Implement testutil/notifier.go**

`FakeNotifier` that records all actions sent. Implements the Discord client interface.

**Step 4: Implement hunts/fake/hunt.go**

`FakeHunt` implementing all 6 required + all 5 optional interfaces. Configurable via functional options. Records all method calls.

**Step 5: Write a smoke test**

Create `hunts/fake/hunt_test.go` — verify FakeHunt satisfies all interfaces at compile time:

```go
var _ core.Hunt = (*FakeHunt)(nil)
var _ core.Grouper = (*FakeHunt)(nil)
var _ core.ReEvaluator = (*FakeHunt)(nil)
var _ core.Briefer = (*FakeHunt)(nil)
var _ core.WebHunt = (*FakeHunt)(nil)
var _ core.NotifyHunt = (*FakeHunt)(nil)
```

**Step 6: Commit**

```bash
git commit -m "feat: test infrastructure — testutil helpers and fake hunt"
```

---

### Task 12: Discord notification client (thread-aware)

**Files:**
- Create: `notify/discord.go`, `notify/notifier.go`
- Test: `notify/discord_test.go`

**Step 1: Write tests**

Test `ExecuteActions` with a fake HTTP server:
- `PostMessage` sends POST to webhook URL
- `CreateThread` sends POST with `thread_name` field, returns thread ID
- `PostToThread` sends POST to `webhook?thread_id=<id>`
- Retry on 429 with Retry-After header
- Retry on 5xx, no retry on 4xx

**Step 2: Run tests — FAIL**

**Step 3: Implement**

Port from comedy-hunter's `notify/` and powder-hunter's `discord/`. Key method: `ExecuteActions(ctx, actions []core.NotifyAction) error` — processes actions in order, tracks thread IDs from CreateThread to resolve ThreadRef in PostToThread.

Also add `PostError(ctx, message string) error` for error alerts to the error webhook.

**Step 4: Run tests — PASS**

**Step 5: Commit**

```bash
git commit -m "feat: thread-aware Discord notification client with retry"
```

---

### Task 13: LLM client (Gemini two-step)

**Files:**
- Create: `llm/gemini.go`, `llm/cost.go`
- Test: `llm/gemini_test.go`

**Step 1: Write tests**

Test the retry logic and response parsing. Use a fake Gemini client or test the parsing functions in isolation (don't hit real API in tests).

**Step 2: Run tests — FAIL**

**Step 3: Implement**

Port the two-step pattern from comedy-hunter's `evaluation/evaluator.go`:
- Step 1: Research with Google Search grounding
- Step 2: Structured JSON extraction with schema
- `generateWithRetry` with exponential backoff

Define a `Client` interface so hunts can use different models/configs:

```go
type Client struct {
    client *genai.Client
    model  string
}

func NewClient(ctx context.Context, apiKey string) (*Client, error)
func (c *Client) Generate(ctx context.Context, prompt string, config *genai.GenerateContentConfig) (string, error)
func (c *Client) TwoStep(ctx context.Context, prompt string, schema *genai.Schema) (research string, structured map[string]any, sources []string, err error)
```

Add dependency: `go get google.golang.org/genai`

**Step 4: Run tests — PASS**

**Step 5: Commit**

```bash
git commit -m "feat: Gemini two-step LLM client with retry"
```

---

### Task 14: Distance client (walking/driving)

**Files:**
- Create: `distance/distance.go`
- Test: `distance/distance_test.go`

**Step 1: Write tests**

Test response parsing with a fake HTTP server. Test mode parameter (walking vs driving).

**Step 2: Run tests — FAIL**

**Step 3: Implement**

Port from comedy-hunter's `distance/distance.go`. Add `mode` parameter to support both walking and driving. Returns `Result{Minutes, DistanceMi}`.

**Step 4: Run tests — PASS**

**Step 5: Commit**

```bash
git commit -m "feat: distance client supporting walking and driving modes"
```

---

### Task 15: Config parsing

**Files:**
- Create: `config/config.go`
- Test: `config/config_test.go`

**Step 1: Write tests**

Test `FromEnv` with a fake lookup function. Test defaults. Test that per-hunt enable flags work. Test that missing required vars return errors.

**Step 2: Run tests — FAIL**

**Step 3: Implement**

```go
type Config struct {
    DBPath      string
    GoogleAPIKey string
    HomeLatitude  float64
    HomeLongitude float64
    HomeAddress   string
    WebPort       int
    DryRun        bool
    ErrorDiscordWebhookURL string
    EnabledHunts  map[string]bool          // "comedy" -> true
    HuntWebhooks  map[string]string        // "comedy" -> webhook URL
}

func FromEnv(lookup func(string) string) (Config, error)
```

Parse `HUNT_<NAME>_ENABLED` and `<NAME>_DISCORD_WEBHOOK_URL` patterns.

**Step 4: Run tests — PASS**

**Step 5: Commit**

```bash
git commit -m "feat: config parsing with per-hunt enable flags"
```

---

### Task 16: Web UI shell (shared layout + tabs)

**Files:**
- Create: `web/web.go`, `web/templates/layout.html`, `web/templates/cards.html`, `web/templates/preferences.html`, `web/templates/feedback.html`, `web/templates/status.html`
- Test: `web/web_test.go`

**Step 1: Write tests**

- `GET /` returns 200 with hunt tabs
- `GET /?hunt=comedy` filters to comedy tab
- `POST /preferences` saves and redirects
- `POST /feedback` saves and redirects
- Pipeline status section shows last run info

**Step 2: Run tests — FAIL**

**Step 3: Implement**

Port from comedy-hunter's `web/web.go`. Key changes:
- Tab navigation per hunt (query param `?hunt=comedy`)
- Generic card rendering using `CardData` from hunt's `CardRenderer`
- Feedback buttons from hunt's `FeedbackOptions`
- Pipeline status section showing `PipelineResult`
- Re-evaluate button per hunt

The `Server` struct takes a list of hunts and renders tabs dynamically. Uses `embed.FS` for templates.

**Step 4: Run tests — PASS**

**Step 5: Commit**

```bash
git commit -m "feat: web UI shell with hunt tabs, cards, feedback, and status"
```

---

### Task 17: Pipeline framework orchestration

**Files:**
- Create: `pipeline/pipeline.go`
- Test: `pipeline/pipeline_test.go`

This is the heart of the framework. Use FakeHunt + testutil for integration tests.

**Step 1: Write tests**

- `TestPipeline_ScanStoresOpportunities`: FakeHunt with fake source → opportunities appear in DB
- `TestPipeline_EvaluateCallsEvaluator`: discovered opportunities → evaluator called → picks stored
- `TestPipeline_GrouperBatchesOpportunities`: hunt with Grouper → evaluator called per group
- `TestPipeline_NotifyCallsFormatter`: evaluated opportunities → NotifyFormatter called → actions sent
- `TestPipeline_ReEvaluatorChecksOld`: hunt with ReEvaluator → calls ShouldReEvaluate for notified opps
- `TestPipeline_BrieferGroupsAndSynthesizes`: hunt with Briefer → groups evaluations, calls Synthesize
- `TestPipeline_BudgetGating`: schedule with MaxMonthlySpendUSD → skips when exceeded
- `TestPipeline_ErrorsCollectedInResult`: failing evaluator → error in PipelineResult, other groups still processed
- `TestPipeline_RetryOnTransientError`: evaluator fails once then succeeds → retry works

**Step 2: Run tests — FAIL**

**Step 3: Implement pipeline/pipeline.go**

```go
type Pipeline struct {
    db          *storage.DB
    notifier    *notify.Client
    errNotifier *notify.Client
    costTracker *core.CostTracker
    dryRun      bool
    lastResult  *core.PipelineResult // for web UI status
}

func New(db *storage.DB, costTracker *core.CostTracker) *Pipeline

func (p *Pipeline) Run(ctx context.Context, hunt core.Hunt) core.HuntResult
func (p *Pipeline) RunAll(ctx context.Context, hunts []core.Hunt) core.PipelineResult
```

The `Run` method implements the 11-step orchestration from the design:
1. **Scan**: call each source concurrently (WaitGroup), dedupe via `hunt.DedupeKey`, upsert venues, insert opportunities
2. **Collect**: `db.GetByState(huntName, Discovered)`
3. **Gate**: check `schedule.MaxMonthlySpendUSD` against `costTracker.ForHunt()`
4. **ReEval**: if `ReEvaluator`, get notified/evaluated opps, filter by `ShouldReEvaluate`
5. **Group**: if `Grouper`, call `GroupForEval`; else one group per opportunity
6. **Evaluate**: `withRetry` → `hunt.Evaluator().Evaluate(ctx, evalContext)` per group
7. **Store**: `db.SaveEvaluationWithPicks`, `opportunity.MarkEvaluated`, `costTracker.Add`
8. **Brief**: if `Briefer`, call `GroupForNotify` then `Synthesize` (retry once, skip on failure)
9. **Notify**: get formatter (or default), call `FormatPicks`, validate actions, `notifier.ExecuteActions`
10. **Remind**: get upcoming opps, call `FormatReminder`, send
11. **Expire**: mark past opps as expired

Each step wraps errors into `StepError` and continues.

**Step 4: Run tests — PASS**

**Step 5: Commit**

```bash
git commit -m "feat: pipeline framework with 11-step orchestration"
```

---

### Task 18: CLI entrypoint

**Files:**
- Create: `cmd/opportunity-hunter/main.go`

**Step 1: Implement main.go**

Subcommands: `run` (daemon), `scan` (one-shot), `eval` (one-shot), `web` (dev), `version`.

**Daemon loop** (same pattern as comedy-hunter):
1. Parse config
2. Open DB
3. Init hunts (only enabled ones), call `Init(ctx, os.Getenv)` on each, skip on error
4. Validate hunts
5. Create CostTracker seeded from DB
6. Create Pipeline
7. Start web server in goroutine
8. Scan ticker (per-hunt intervals) + eval ticker
9. Graceful shutdown on SIGTERM/SIGINT

Register hunts in a simple slice. All three hunts are imported but only enabled ones are initialized.

**Step 2: Verify it compiles**

```bash
go build ./cmd/opportunity-hunter/
```

**Step 3: Commit**

```bash
git commit -m "feat: CLI entrypoint with daemon, scan, eval, web commands"
```

---

## Phase 2: Performing Arts Hunt

New hunt — validates the framework interfaces without porting baggage. This is intentionally the first hunt because it exercises the core without legacy assumptions.

### Task 19: Performing arts — hunt skeleton + sources

**Files:**
- Create: `hunts/performing/hunt.go`, `hunts/performing/attrs.go`, `hunts/performing/sources/ticketmaster.go`
- Test: `hunts/performing/hunt_test.go`

**Step 1: Write tests**

- Compile-time interface checks: `var _ core.Hunt = (*PerformingHunt)(nil)`, etc.
- `Init` validates config
- `DedupeKey` produces expected keys
- `DefaultSchedule` returns correct values

**Step 2: Run tests — FAIL**

**Step 3: Implement**

`PerformingHunt` implements `Hunt + Grouper + WebHunt + NotifyHunt`.

- `Name()` → `"performing-arts"`
- `Init()` → validates Ticketmaster API key (performing arts uses Ticketmaster to find theater/ballet/opera)
- `Sources()` → Ticketmaster source configured for performing arts categories (classificationName: "Arts & Theatre")
- `DedupeKey(raw)` → `title|venue|date`
- `DefaultSchedule()` → scan 12h, eval weekly, remind 7d + 1d before

`PerformingAttrs`:
```go
type PerformingAttrs struct {
    Genre      string  `json:"genre"`       // ballet, opera, theater, musical
    ReviewScore *float64 `json:"review_score"` // aggregated if available
    Awards     []string `json:"awards"`
}
```

Ticketmaster source: port from comedy-hunter's `sources/ticketmaster/`, change classification filter to Arts & Theatre.

**Step 4: Run tests — PASS**

**Step 5: Commit**

```bash
git commit -m "feat: performing arts hunt skeleton with Ticketmaster source"
```

---

### Task 20: Performing arts — evaluator + prompt

**Files:**
- Create: `hunts/performing/evaluator.go`, `hunts/performing/prompt.go`
- Test: `hunts/performing/evaluator_test.go`

**Step 1: Write tests**

Test prompt building with sample opportunities. Test that the evaluator returns picks with correct fields. Use a fake LLM client.

**Step 2: Run tests — FAIL**

**Step 3: Implement**

Evaluator closes over the LLM client. Prompt structure similar to comedy but for performing arts:
- System instruction: you are a performing arts recommender
- Scoring calibration adapted for theater/ballet/opera
- User preferences section
- Past feedback section
- Shows section with genre, venue, dates, price
- Consolidation: group by show title + venue

**Step 4: Run tests — PASS**

**Step 5: Commit**

```bash
git commit -m "feat: performing arts evaluator and prompt builder"
```

---

### Task 21: Performing arts — cards, notify, feedback

**Files:**
- Create: `hunts/performing/cards.go`, `hunts/performing/notify.go`
- Test: `hunts/performing/cards_test.go`

**Step 1: Write tests**

Test `RenderCard` produces correct CardData with genre, price fields. Test `FormatPicks` returns PostMessage actions.

**Step 2: Run tests — FAIL**

**Step 3: Implement**

- `CardRenderer`: Fields include genre, price, venue, dates
- `FeedbackOptions`: `loved`, `not_for_me`, `already_seen`
- `NotifyFormatter`: Returns `PostMessage` actions (same pattern as comedy — no threads)
- `GroupForEval`: group by week (same pattern as comedy)

**Step 4: Run tests — PASS**

**Step 5: Commit**

```bash
git commit -m "feat: performing arts cards, notifications, and feedback"
```

---

### Task 22: End-to-end integration test with performing arts

**Files:**
- Test: `pipeline/integration_test.go`

**Step 1: Write integration test**

Full pipeline run with performing arts hunt using:
- Fake Ticketmaster source returning 5 canned shows
- Fake LLM client returning canned picks
- Real SQLite (in-memory via testutil)
- Fake Discord notifier

Assert: opportunities stored, evaluation created, picks stored, notification actions generated, pipeline result has correct counts.

**Step 2: Run test — expect PASS**

If failures, fix issues in core/pipeline/storage.

**Step 3: Commit**

```bash
git commit -m "test: end-to-end integration test with performing arts hunt"
```

---

## Phase 3: Comedy Hunt (Port)

Port comedy-hunter into the framework. This proves Grouper works and validates the migration path.

### Task 23: Comedy — hunt skeleton + sources

**Files:**
- Create: `hunts/comedy/hunt.go`, `hunts/comedy/attrs.go`, `hunts/comedy/sources/ticketmaster.go`, `hunts/comedy/sources/eventbrite.go`, `hunts/comedy/sources/comedyworks.go`
- Test: `hunts/comedy/hunt_test.go`

**Step 1: Write tests**

Interface checks. `DedupeKey` matches comedy-hunter's pattern (`comedian|venue|date`).

**Step 2: Run tests — FAIL**

**Step 3: Implement**

`ComedyHunt` implements `Hunt + Grouper + WebHunt + NotifyHunt`.

- Port all three sources from comedy-hunter (`sources/ticketmaster/`, `sources/eventbrite/`, `sources/comedyworks/`)
- Map `RawShow` → `core.RawItem` (Comedian → Title, VenueName → VenueName, etc.)
- `GroupForEval`: port `groupByWeek` from `pipeline/pipeline.go`
- `ComedyAttrs`: `SellOutRisk string`

Add dependency: `go get github.com/gocolly/colly/v2` (for Comedy Works scraper)

**Step 4: Run tests — PASS**

**Step 5: Commit**

```bash
git commit -m "feat: comedy hunt skeleton with Ticketmaster, Eventbrite, Comedy Works sources"
```

---

### Task 24: Comedy — evaluator + prompt

**Files:**
- Create: `hunts/comedy/evaluator.go`, `hunts/comedy/prompt.go`
- Test: `hunts/comedy/evaluator_test.go`

**Step 1: Write tests**

Test prompt building. Test show consolidation (grouping by comedian+venue). Test that evaluator closes over distance client and fetches walking info during evaluation.

**Step 2: Run tests — FAIL**

**Step 3: Implement**

Port from comedy-hunter's `evaluation/`:
- `comedyEvaluator` struct holds LLM client + distance client (closure pattern from design decision 7)
- `BuildPrompt` port from `evaluation/prompt.go` — same prompt structure, same consolidation logic
- Map `domain.ShowPick` → `core.Pick` (Score int → Score float64 normalized, SellOutRisk → Pick.Attributes)
- Gemini schema adapted for the core Pick format

Key mapping: comedy-hunter's `Show.Comedian` → `Opportunity.Title`. Update prompt template to use `Title` instead of `Comedian`.

**Step 4: Run tests — PASS**

**Step 5: Commit**

```bash
git commit -m "feat: comedy evaluator and prompt builder ported from comedy-hunter"
```

---

### Task 25: Comedy — cards, notify, feedback

**Files:**
- Create: `hunts/comedy/cards.go`, `hunts/comedy/notify.go`
- Test: `hunts/comedy/cards_test.go`

**Step 1: Write tests**

Test CardRenderer produces fields for walking distance, price, sell-out risk. Test NotifyFormatter produces PostMessage with formatted picks.

**Step 2: Run tests — FAIL**

**Step 3: Implement**

- `CardRenderer`: Port web UI rendering from comedy-hunter. Fields: walking distance, price range, sell-out risk.
- `FeedbackOptions`: `loved`, `not_for_me`
- `NotifyFormatter`: Port from comedy-hunter's `notify/`. Returns PostMessage actions with formatted pick embeds.

**Step 4: Run tests — PASS**

**Step 5: Commit**

```bash
git commit -m "feat: comedy cards, notifications, and feedback"
```

---

### Task 26: Comedy integration test + interface revision checkpoint

**Files:**
- Test: `pipeline/comedy_integration_test.go`

**Step 1: Write integration test**

Full pipeline run with comedy hunt. Fake sources, fake LLM, real DB, fake Discord. Assert weekly grouping works, picks stored in normalized table, notifications sent.

**Step 2: Run test**

**Step 3: Interface revision checkpoint**

Per the design doc: "Expect at least one interface revision after step 3 before tackling step 4."

Review what didn't fit cleanly during the comedy port. Common issues:
- Did the comedian name mapping to Title work smoothly in prompts?
- Did weekly grouping via Grouper produce correct Group.Key values?
- Did the EvalContext have everything the comedy evaluator needed?
- Did the NotifyFormatter produce valid actions?

Fix any interface issues before moving to powder.

**Step 4: Commit**

```bash
git commit -m "test: comedy integration test and interface revision"
```

---

## Phase 4: Powder Hunt (Port)

Most complex hunt. Proves ReEvaluator + Briefer work. Weather sources, re-evaluation gating, macro-region grouping, threaded Discord notifications.

### Task 27: Powder — domain types and catalog

**Files:**
- Create: `hunts/powder/attrs.go`, `hunts/powder/catalog/regions.go`, `hunts/powder/catalog/resorts.go`
- Test: `hunts/powder/attrs_test.go`

**Step 1: Implement**

Port powder-hunter's domain types that are hunt-specific (not in core):
- `PowderAttrs`: snowfall, consensus, friction tier, change class, weather snapshot
- Region catalog: static data for ski regions (from powder-hunter's seed data)
- Resort catalog: static data for resorts per region

These are hunt-internal types, not core types.

**Step 2: Commit**

```bash
git commit -m "feat: powder domain types and region/resort catalog"
```

---

### Task 28: Powder — weather sources

**Files:**
- Create: `hunts/powder/sources/openmeteo.go`, `hunts/powder/sources/nws.go`
- Test: `hunts/powder/sources/openmeteo_test.go`

**Step 1: Write tests**

Test response parsing with canned API responses. Test that weather data maps to `core.RawItem` correctly.

**Step 2: Run tests — FAIL**

**Step 3: Implement**

Port from powder-hunter's weather sources. Key adaptation: weather sources return `core.RawItem` where:
- `Title` = region name + storm window description
- `Subtitle` = date range
- `VenueName` = region name (regions are "venues" for powder)
- `Attributes` = `PowderAttrs` with snowfall, consensus, etc.
- `StartTime`/`EndTime` = storm window bounds

Detection logic (threshold checking, window merging) stays inside the powder hunt package.

**Step 4: Run tests — PASS**

**Step 5: Commit**

```bash
git commit -m "feat: powder weather sources (Open-Meteo, NWS)"
```

---

### Task 29: Powder — hunt skeleton with ReEvaluator

**Files:**
- Create: `hunts/powder/hunt.go`, `hunts/powder/detection.go`
- Test: `hunts/powder/hunt_test.go`, `hunts/powder/detection_test.go`

**Step 1: Write tests**

- Interface checks: Hunt + ReEvaluator + Briefer + WebHunt + NotifyHunt
- `ShouldReEvaluate`: test weather change detection, cooldown by tier, budget check
- `DedupeKey`: `region|window_start|window_end`

**Step 2: Run tests — FAIL**

**Step 3: Implement**

Port gating logic from powder-hunter's `pipeline/gating.go` and `domain/weather_compare.go`:
- `ShouldReEvaluate` checks: weather changed (snowfall delta > thresholds) → re-eval; cooldown not elapsed → skip; unchanged weather → skip
- The hunt holds the cost tracker reference to check budget in ShouldReEvaluate

`PowderHunt` struct:
```go
type PowderHunt struct {
    weatherSvc  *weather.Service
    llm         *llm.Client
    regions     []Region
    costTracker *core.CostTracker
}
```

**Step 4: Run tests — PASS**

**Step 5: Commit**

```bash
git commit -m "feat: powder hunt with ReEvaluator (weather change + cooldown gating)"
```

---

### Task 30: Powder — evaluator + prompt

**Files:**
- Create: `hunts/powder/evaluator.go`, `hunts/powder/prompt.go`
- Test: `hunts/powder/evaluator_test.go`

**Step 1: Write tests**

Test prompt rendering with weather data. Test that evaluator handles re-evaluation (PriorEval in EvalContext). Test change class comparison.

**Step 2: Run tests — FAIL**

**Step 3: Implement**

Port from powder-hunter's `evaluation/`. The evaluator:
1. Fetches weather data via closed-over weather service
2. Builds prompt with forecasts, region info, resorts, user profile
3. Calls Gemini two-step (research + structured extraction)
4. If PriorEval exists, compares to classify change (new/material/minor/downgrade)
5. Maps result to `core.Evaluation` + `core.Pick` with `PowderAttrs` in Attributes

Powder produces one Pick per opportunity (one storm = one opportunity = one pick). The Pick's `DisplayScore` uses tier names ("DROP EVERYTHING", "WORTH A LOOK", "ON THE RADAR").

**Step 4: Run tests — PASS**

**Step 5: Commit**

```bash
git commit -m "feat: powder evaluator with weather prompts and change classification"
```

---

### Task 31: Powder — Briefer (grouping + synthesis)

**Files:**
- Create: `hunts/powder/grouping.go`, `hunts/powder/briefing.go`
- Test: `hunts/powder/grouping_test.go`, `hunts/powder/briefing_test.go`

**Step 1: Write tests**

- `GroupForNotify`: test grouping by macro-region + friction tier, splitting by window overlap
- `Synthesize`: test with fake LLM client, verify briefing text is returned

**Step 2: Run tests — FAIL**

**Step 3: Implement**

Port from powder-hunter's `domain/grouping.go` and `evaluation/` briefing:
- `GroupForNotify`: buckets evaluations by (StormGroup + FrictionTier from PowderAttrs), splits non-overlapping windows, sorts members by tier
- `Synthesize`: calls LLM with cross-region comparison prompt, returns 2-4 sentence briefing

**Step 4: Run tests — PASS**

**Step 5: Commit**

```bash
git commit -m "feat: powder Briefer with macro-region grouping and LLM synthesis"
```

---

### Task 32: Powder — cards, notify (threaded), feedback

**Files:**
- Create: `hunts/powder/cards.go`, `hunts/powder/notify.go`
- Test: `hunts/powder/cards_test.go`, `hunts/powder/notify_test.go`

**Step 1: Write tests**

- `CardRenderer`: fields for snowfall, conditions, tier, drive time
- `NotifyFormatter.FormatPicks`: returns CreateThread + PostToThread sequence
- `NotifyFormatter.FormatPicks` with re-evaluation: returns PostToThread update + conditional Ping

**Step 2: Run tests — FAIL**

**Step 3: Implement**

- `CardRenderer`: Fields for snowfall, snow quality, crowd estimate, best day, drive time, tier
- `FeedbackOptions`: `went_great`, `went_ok`, `skipped`
- `NotifyFormatter`:
  - New evaluation: `CreateThread` (briefing from Synthesis) + `PostToThread` x N (per region detail)
  - Re-evaluation: `PostToThread` (update embed) + `Ping: true` on tier escalation (material change)
  - Thread names: "PNW Cascades — Jan 15-17" format

**Step 4: Run tests — PASS**

**Step 5: Commit**

```bash
git commit -m "feat: powder cards, threaded notifications, and feedback"
```

---

### Task 33: Powder integration test

**Files:**
- Test: `pipeline/powder_integration_test.go`

**Step 1: Write integration test**

Full pipeline run with powder hunt:
- Fake weather sources returning storm data for 3 regions in same macro-region
- Fake LLM returning evaluations
- Real DB, fake Discord
- Assert: opportunities stored with PowderAttrs, evaluations created, Briefer groups regions, synthesis called, CreateThread + PostToThread actions generated

**Step 2: Write re-evaluation test**

Run pipeline twice. Second run: weather changed for one region. Assert:
- `ShouldReEvaluate` returns true for changed region
- New evaluation created (old one preserved)
- PostToThread update action generated (not new thread)
- Opportunity state unchanged (stays notified)

**Step 3: Run tests**

**Step 4: Commit**

```bash
git commit -m "test: powder integration tests including re-evaluation"
```

---

## Phase 5: Integration + Polish

Wire everything together, add Docker, error alerting, and final testing.

### Task 34: Wire all hunts into main.go

**Files:**
- Modify: `cmd/opportunity-hunter/main.go`

**Step 1: Register all three hunts**

```go
allHunts := []core.Hunt{
    &comedy.ComedyHunt{},
    &performing.PerformingHunt{},
    &powder.PowderHunt{},
}
```

Filter to enabled hunts, call Init on each, validate, start pipeline.

**Step 2: Test manually**

```bash
# Scan only (Comedy Works scraper doesn't need API keys)
go run ./cmd/opportunity-hunter/ scan
```

**Step 3: Commit**

```bash
git commit -m "feat: wire all three hunts into CLI entrypoint"
```

---

### Task 35: Error alerting (Discord + web UI)

**Files:**
- Modify: `pipeline/pipeline.go`, `web/web.go`, `web/templates/status.html`

**Step 1: Implement error Discord posting**

After `RunAll` completes, if `PipelineResult.HasErrors()`, format and post summary to error webhook. Track consecutive failures per hunt+step for escalation.

**Step 2: Implement web UI status**

Store last `PipelineResult` on the web server. Render in `status.html` template: last run time, per-hunt counts, errors with context.

**Step 3: Test**

Run pipeline with a hunt that has a failing source. Verify error appears in PipelineResult and renders in web UI.

**Step 4: Commit**

```bash
git commit -m "feat: error alerting to Discord and pipeline status in web UI"
```

---

### Task 36: Reminder system

**Files:**
- Create: `remind/remind.go`
- Test: `remind/remind_test.go`

**Step 1: Write tests**

Test that reminders are sent for opportunities within RemindBefore windows. Test that reminded opportunities are marked as Reminded.

**Step 2: Run tests — FAIL**

**Step 3: Implement**

Port from comedy-hunter's `remind/`. Generalized: reads `Schedule.RemindBefore` durations, finds opportunities in those windows, calls hunt's NotifyFormatter.FormatReminder.

**Step 4: Run tests — PASS**

**Step 5: Commit**

```bash
git commit -m "feat: reminder system using hunt-defined RemindBefore windows"
```

---

### Task 37: Docker + docker-compose

**Files:**
- Create: `Dockerfile`, `docker-compose.yml`

**Step 1: Implement**

Port from comedy-hunter. Multi-stage Alpine build. Volume mount for SQLite DB and .env file.

```dockerfile
FROM golang:1.24-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o opportunity-hunter ./cmd/opportunity-hunter/

FROM alpine:latest
RUN apk add --no-cache ca-certificates tzdata
COPY --from=builder /app/opportunity-hunter /usr/local/bin/
ENTRYPOINT ["opportunity-hunter"]
CMD ["run"]
```

**Step 2: Test build**

```bash
docker build -t opportunity-hunter .
```

**Step 3: Commit**

```bash
git commit -m "feat: Dockerfile and docker-compose for deployment"
```

---

### Task 38: CONTRIBUTING.md

**Files:**
- Create: `CONTRIBUTING.md`

**Step 1: Write contributor guide**

Sections:
- How to add a new hunt (implement 6 required methods, register in main.go)
- Optional interfaces and when to use them
- How to write and test sources
- How to test with FakeHunt and testutil
- Environment setup
- Code style (Go conventions)

**Step 2: Commit**

```bash
git commit -m "docs: CONTRIBUTING.md with hunt authoring guide"
```

---

### Task 39: Final integration test — all three hunts

**Files:**
- Test: `pipeline/full_integration_test.go`

**Step 1: Write test**

Run pipeline with all three hunts enabled. Fake sources, fake LLM, real DB, fake Discord. Assert:
- Each hunt scans, evaluates, and notifies independently
- Shared venues work (same theater used by comedy and performing arts)
- Sequential execution (no data races)
- PipelineResult has results for all three hunts
- Distance cache shared across hunts

**Step 2: Run all tests**

```bash
make test
```

**Step 3: Fix any failures**

**Step 4: Commit**

```bash
git commit -m "test: full integration test with all three hunts"
```

---

## Summary

| Phase | Tasks | What it proves |
|-------|-------|---------------|
| 1: Core Framework | 1-18 | Types, storage, pipeline, web, notify, test infra all work |
| 2: Performing Arts | 19-22 | Framework interfaces are usable for a new hunt |
| 3: Comedy Port | 23-26 | Grouper works, comedy-hunter migrates cleanly |
| 4: Powder Port | 27-33 | ReEvaluator + Briefer work, threading works, re-eval works |
| 5: Integration | 34-39 | All hunts run together, error alerting, Docker, docs |

**Total: 39 tasks.** Each task is a focused unit of work with TDD cycle and a commit.
