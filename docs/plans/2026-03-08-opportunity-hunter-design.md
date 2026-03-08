# Opportunity Hunter — Unified Design (v2)

**Date**: 2026-03-08
**Status**: Design approved, pending implementation planning
**Prior version**: comedy-hunter/docs/plans/2026-03-07-opportunity-hunter-design.md

## Problem

Four projects — comedy-hunter, powder-hunter, a new performing-arts hunter, and a new movies hunter — share the same fundamental pattern: discover opportunities, evaluate them with an LLM, recommend the best ones, collect feedback, and improve over time. Rather than maintain separate codebases with duplicated infrastructure, unify them into a single pluggable system.

## Decisions

- **Fresh repo** (`opportunity-hunter`), port existing projects in incrementally
- **Go interface plugin pattern** — hunts are Go packages, not config-driven
- **Framework pipeline** — core owns the orchestration loop; hunts implement interfaces at defined hook points
- **Open-source and location-agnostic** — users configure their own location, enable the hunts they want, plug in their own venues/sources
- **Per-hunt Discord webhooks** — clean channel separation
- **Per-hunt web UI tabs** — self-contained, no cross-hunt views
- **Shared cost tracking** — seeded from DB on startup, budget cap on Schedule
- **Small required interface (6 methods) + 6 optional interfaces** — hunts opt in to what they need
- **Sequential hunt execution** — eliminates concurrency bugs; concurrent source scanning within a hunt
- **Linear state machine** — states only move forward; re-evaluation creates new records
- **Normalized picks table** — queryable, indexable, proper relational design
- **Error resilience** — log-and-continue with PipelineResult, Discord error alerts, retry with backoff

## Core Entity Model

Every hunt produces the same thing: an **Opportunity** worth acting on.

```go
type Opportunity struct {
    ID           int64
    HuntName     string     // "comedy", "performing-arts", "powder", "movies"
    SourceID     string
    Source       string
    Title        string     // comedian name, show name, storm description, movie title
    Subtitle     string     // venue name, date range, streaming service
    VenueID      *int64     // nil for venue-agnostic opportunities (streaming movies)
    StartTime    time.Time
    EndTime      *time.Time // nil for single events, set for storm windows
    PriceMin     *float64
    PriceMax     *float64
    TicketURL    string
    State        State      // discovered -> evaluated -> notified -> reminded -> expired
    Attributes   Attributes // hunt-specific extras (json.RawMessage)
    RawData      string
    DiscoveredAt time.Time
    EvaluatedAt  *time.Time
    NotifiedAt   *time.Time
    RemindedAt   *time.Time
}

// Attributes is the type alias for hunt-specific JSON data.
// Core stores and passes it through without parsing.
// Each hunt defines typed structs with Encode/Decode helpers.
type Attributes = json.RawMessage
```

`Attributes` is the escape hatch for domain-specific data. Powder stores snowfall totals, model consensus, friction tier. Performing arts stores review scores, awards. The core never parses it. Each hunt defines typed structs:

```go
// hunts/powder/attrs.go
type PowderAttrs struct {
    SnowfallIn   float64 `json:"snowfall_in"`
    Consensus    float64 `json:"consensus"`
    FrictionTier string  `json:"friction_tier"`
}

func DecodePowderAttrs(raw core.Attributes) (PowderAttrs, error) {
    var a PowderAttrs
    return a, json.Unmarshal(raw, &a)
}

func (a PowderAttrs) Encode() core.Attributes {
    b, _ := json.Marshal(a)
    return b
}
```

### State Transitions

States only move forward. The state machine is strictly linear.

```go
type State string

const (
    Discovered State = "discovered"
    Evaluated  State = "evaluated"
    Notified   State = "notified"
    Reminded   State = "reminded"
    Expired    State = "expired"
)
```

Transition methods atomically set both state and timestamp:

```go
func (o *Opportunity) MarkEvaluated(at time.Time) { o.State = Evaluated; o.EvaluatedAt = &at }
func (o *Opportunity) MarkNotified(at time.Time)  { o.State = Notified; o.NotifiedAt = &at }
func (o *Opportunity) MarkReminded(at time.Time)  { o.State = Reminded; o.RemindedAt = &at }
func (o *Opportunity) MarkExpired()               { o.State = Expired }
```

Re-evaluation does NOT change an opportunity's state. It creates a new `Evaluation` record and posts an update to the existing Discord thread. The opportunity stays in its current state (typically `notified`).

### Venues — shared, normalized

Venues are physical locations shared across hunts. A theater might host both comedy and performing arts; a resort is a venue with hunt-specific attributes stored on the Opportunity.

```go
type Venue struct {
    ID        int64
    Name      string   // normalized on upsert (lowercase, strip punctuation, fuzzy match)
    Address   string
    Latitude  float64
    Longitude float64
    Notes     string
}
```

**Normalization**: On upsert, venue names are normalized (lowercased, punctuation stripped) and fuzzy-matched against existing venues to prevent duplicates like "Comedy Works Downtown" vs "Comedy Works - Downtown".

**Hunt-specific venue attributes** (resort elevation, trail count, etc.) live on `Opportunity.Attributes`, not on the Venue itself. Venues stay simple.

**Deduplication** — each hunt provides `DedupeKey(item) string`. Comedy: `comedian|venue|date`. Powder: `region|window_start|window_end`.

## Hunt Interface — Small Core + Optional Capabilities

The required interface is 6 methods. Hunts opt in to additional capabilities via optional interfaces that the core checks with type assertions at startup.

```go
// Hunt is the required interface. Every hunt must implement this.
type Hunt interface {
    Name() string
    Init(ctx context.Context, lookup func(string) string) error
    Sources() []Source
    DedupeKey(raw RawItem) string
    Evaluator() Evaluator
    DefaultSchedule() Schedule
}
```

`Init` is called once at startup. Hunts validate their configuration and create API clients here. If Init returns an error, the hunt is not started and the error is reported.

```go
func (h *ComedyHunt) Init(ctx context.Context, lookup func(string) string) error {
    h.tmKey = lookup("TICKETMASTER_API_KEY")
    if h.tmKey == "" {
        return fmt.Errorf("comedy: TICKETMASTER_API_KEY required")
    }
    h.ebToken = lookup("EVENTBRITE_API_TOKEN") // optional
    h.llm = gemini.NewClient(lookup("GOOGLE_API_KEY"))
    return nil
}
```

### Optional Interfaces (6)

Hunts implement these only when they need non-default behavior. The core checks via type assertion at startup and falls back to sensible defaults.

```go
// Grouper controls how opportunities are batched before evaluation.
// Default: each opportunity evaluated individually.
type Grouper interface {
    GroupForEval(items []Opportunity) []Group
}

// ReEvaluator controls whether an opportunity should be re-evaluated.
// Default: no re-evaluation (event hunts evaluate once).
// Powder uses this for weather change detection + cooldown logic.
type ReEvaluator interface {
    ShouldReEvaluate(opp Opportunity, lastEval *Evaluation) bool
}

// Briefer controls post-eval grouping and synthesis.
// Default: one notification per evaluation, no synthesis.
// Powder uses this to bundle regions into macro-region threads with LLM briefings.
type Briefer interface {
    GroupForNotify(evals []Evaluation) []NotifyGroup
    Synthesize(ctx context.Context, group NotifyGroup, costTracker *CostTracker) (string, error)
}

// Expirer controls when opportunities should be marked as expired.
// Default: expire when StartTime is in the past.
// Movies uses this: theatrical expires ~8 weeks after release, streaming never expires.
// Powder uses this: expire when EndTime (window end) is in the past.
type Expirer interface {
    ShouldExpire(opp Opportunity) bool
}

// WebHunt provides UI customization.
// Default: generic card rendering, no feedback options.
type WebHunt interface {
    CardRenderer() CardRenderer
    FeedbackOptions() []FeedbackOption
}

// NotifyHunt provides notification formatting.
// Default: simple text message with picks.
type NotifyHunt interface {
    NotifyFormatter() NotifyFormatter
}
```

At startup, `ValidateHunt(h Hunt) error` checks interface co-dependencies and logs which optional interfaces each hunt implements.

### How comedy implements this

```go
// comedy implements: Hunt + Grouper + WebHunt + NotifyHunt
type ComedyHunt struct { ... }

func (h *ComedyHunt) Name() string                                      { return "comedy" }
func (h *ComedyHunt) Init(ctx context.Context, lookup func(string) string) error { ... }
func (h *ComedyHunt) Sources() []core.Source                            { ... }
func (h *ComedyHunt) DedupeKey(raw core.RawItem) string                 { ... }
func (h *ComedyHunt) Evaluator() core.Evaluator                         { ... }
func (h *ComedyHunt) DefaultSchedule() core.Schedule                     { ... }
func (h *ComedyHunt) GroupForEval(items []core.Opportunity) []core.Group { return groupByWeek(items) }
func (h *ComedyHunt) CardRenderer() core.CardRenderer                    { ... }
func (h *ComedyHunt) FeedbackOptions() []core.FeedbackOption             { ... }
func (h *ComedyHunt) NotifyFormatter() core.NotifyFormatter              { ... }
```

### How powder implements this

```go
// powder implements: Hunt + ReEvaluator + Briefer + WebHunt + NotifyHunt
type PowderHunt struct { ... }

func (h *PowderHunt) Name() string                                                          { return "powder" }
func (h *PowderHunt) Init(ctx context.Context, lookup func(string) string) error             { ... }
func (h *PowderHunt) Sources() []core.Source                                                 { ... }
func (h *PowderHunt) DedupeKey(raw core.RawItem) string                                     { ... }
func (h *PowderHunt) Evaluator() core.Evaluator                                              { ... }
func (h *PowderHunt) DefaultSchedule() core.Schedule                                          { ... }
func (h *PowderHunt) ShouldReEvaluate(opp core.Opportunity, last *core.Evaluation) bool      { ... }
func (h *PowderHunt) GroupForNotify(evals []core.Evaluation) []core.NotifyGroup               { ... }
func (h *PowderHunt) Synthesize(ctx context.Context, g core.NotifyGroup, ct *core.CostTracker) (string, error) { ... }
func (h *PowderHunt) CardRenderer() core.CardRenderer                                        { ... }
func (h *PowderHunt) FeedbackOptions() []core.FeedbackOption                                  { ... }
func (h *PowderHunt) NotifyFormatter() core.NotifyFormatter                                   { ... }
```

Note: powder does NOT implement `Grouper` (no pre-eval grouping) — each storm/region is evaluated individually. It implements `Briefer` to bundle regions into macro-region threads after evaluation. Budget gating is handled by `MaxMonthlySpendUSD` on its `Schedule`.

### How movies implements this

```go
// movies implements: Hunt + Expirer + WebHunt + NotifyHunt
type MoviesHunt struct { ... }

func (h *MoviesHunt) Name() string                                                  { return "movies" }
func (h *MoviesHunt) Init(ctx context.Context, lookup func(string) string) error     { ... }
func (h *MoviesHunt) Sources() []core.Source                                         { ... } // TMDB + Letterboxd + Ticketmaster
func (h *MoviesHunt) DedupeKey(raw core.RawItem) string                              { ... } // title|year or title|venue|date for screenings
func (h *MoviesHunt) Evaluator() core.Evaluator                                      { ... }
func (h *MoviesHunt) DefaultSchedule() core.Schedule                                  { ... }
func (h *MoviesHunt) ShouldExpire(opp core.Opportunity) bool                         { ... } // theatrical: 8 weeks; streaming: never
func (h *MoviesHunt) CardRenderer() core.CardRenderer                                { ... }
func (h *MoviesHunt) FeedbackOptions() []core.FeedbackOption                          { ... }
func (h *MoviesHunt) NotifyFormatter() core.NotifyFormatter                           { ... }
```

Note: movies does NOT implement `Grouper` (each movie evaluated individually), `ReEvaluator`, or `Briefer`. It is the simplest hunt in terms of pipeline behavior — it exercises the framework defaults. Its complexity is in the feedback loop and mixed venue/non-venue opportunities.

Movies has three sources:
- **TMDB** — new theatrical and streaming releases (no venue, VenueID=nil)
- **Letterboxd** — trending/popular for discovery signal (no venue, VenueID=nil)
- **Ticketmaster/Eventbrite** — local special screenings, arthouse events, film festivals (with venue)

The evaluator is heavily feedback-driven. User ratings of past movies (both scanned and manually added) inform future recommendations. The LLM learns taste patterns from the feedback history.

## Pipeline

The pipeline is a framework that owns the orchestration loop. Hunts provide behavior at defined hook points via the required and optional interfaces. The core runs hunts sequentially; sources within a hunt scan concurrently.

```go
type Pipeline struct {
    db          *storage.DB
    hunts       []Hunt
    notifier    *notify.Discord
    errNotifier *notify.Discord  // error alert channel
    costTracker *CostTracker
    dryRun      bool
}
```

### Orchestration flow

```
for each enabled hunt (sequentially):
    1. Scan        — fetch from hunt.Sources() concurrently, dedupe, store new opportunities
    2. Collect     — gather opportunities needing evaluation (state=discovered)
    3. Gate        — check schedule.MaxMonthlySpendUSD against costTracker
    4. ReEval      — if ReEvaluator, check ShouldReEvaluate for previously-evaluated items
    5. Group       — if Grouper, group for eval; otherwise evaluate individually
    6. Evaluate    — call hunt.Evaluator().Evaluate() per group (with retry)
    7. Store       — persist evaluations + picks + update opportunity states
    8. Brief       — if Briefer, group evaluations for notification + synthesize (retry once, skip group on failure)
    9. Notify      — format + send via Discord (with retry; handles threads, updates, new posts)
   10. Remind      — send reminders for upcoming opportunities
   11. Expire      — mark past opportunities as expired
```

Steps 4 is a no-op for event hunts (comedy, performing arts). Step 8 is a no-op for simple hunts. The pipeline reads the hunt's capabilities once at startup and skips inapplicable steps.

### Error handling

The pipeline uses **log-and-continue**: one failing evaluation doesn't stop other evaluations for that hunt, and one failing hunt doesn't stop other hunts.

All external calls (LLM, Discord, source APIs) are wrapped in a retry helper: 3 attempts with exponential backoff (0s, 1s, 5s).

```go
func withRetry[T any](ctx context.Context, name string,
    fn func() (T, error)) (T, error) {
    var lastErr error
    for i, delay := range []time.Duration{0, 1 * time.Second, 5 * time.Second} {
        time.Sleep(delay)
        result, err := fn()
        if err == nil { return result, nil }
        lastErr = err
        slog.Warn("retry", "attempt", i+1, "op", name, "err", err)
    }
    var zero T
    return zero, lastErr
}
```

After the run completes, the pipeline returns a `PipelineResult`:

```go
type PipelineResult struct {
    HuntResults []HuntResult
}

type HuntResult struct {
    HuntName  string
    Scanned   int
    Evaluated int
    Notified  int
    Errors    []StepError
}

type StepError struct {
    Step    string // "scan", "evaluate", "notify", "brief", etc.
    Err     error
    Context string // which item/group failed
}
```

If the result contains errors, a summary is posted to the error Discord webhook and the last-run status is displayed in the web UI. Consecutive failures for the same hunt+step trigger escalated alerts (e.g., "3rd consecutive scan failure for comedy").

### Evaluation

```go
type Evaluator interface {
    Evaluate(ctx context.Context, ec EvalContext) (*Evaluation, error)
}

type EvalContext struct {
    Opportunities []Opportunity        // the items being evaluated (1 for powder, N for comedy)
    Venues        map[int64]Venue      // venue lookup
    Preferences   string               // user preferences text
    Feedback      []FeedbackEntry      // recent feedback for prompt context
    CostTracker   *CostTracker         // shared cost tracking
    PriorEval     *Evaluation          // previous evaluation for this group (nil if first)
}
```

Evaluators close over hunt-specific state. The `EvalContext` carries what ALL evaluators need; hunt-specific data (walking info, weather) is fetched by the evaluator itself via dependencies injected through the hunt's `Evaluator()` factory method.

```go
func (h *ComedyHunt) Evaluator() core.Evaluator {
    return &comedyEvaluator{
        llm:      h.llm,
        distance: h.distanceClient,
    }
}
```

A `Validate() error` method on `EvalContext` checks: `len(Opportunities) > 0`, every `VenueID` exists in `Venues`, and `CostTracker != nil`.

### Evaluation Output

```go
type Evaluation struct {
    ID               int64
    HuntName         string
    GroupKey         string
    EvaluatedAt      time.Time
    SkippedReasoning string     // why evaluation was skipped (cooldown, no change, etc.)
    RawLLMResponse   string
    RenderedPrompt   string
    CostUSD          float64
}

type Pick struct {
    ID            int64
    EvaluationID  int64
    OpportunityID int64
    Score         float64  // normalized 0-1 internally
    DisplayScore  string   // hunt decides: "8/10", "DROP EVERYTHING", etc.
    Reason        string
    Urgency       string
    Attributes    Attributes // sell_out_risk, snow_quality, change_class, etc.
}
```

Picks are stored in a normalized table with foreign keys to evaluations and opportunities.

## Notifications — Thread-Aware

The notification system supports both simple posts and threaded conversations.

```go
type NotifyFormatter interface {
    FormatPicks(ctx NotifyContext) []NotifyAction
    FormatReminder(opp Opportunity, pick Pick, reminderType ReminderType) []NotifyAction
}

type NotifyContext struct {
    Evaluations   []Evaluation
    Picks         []Pick
    Opportunities []Opportunity
    Synthesis     string              // from Briefer, empty if not implemented
}

type NotifyAction struct {
    Type       ActionType          // CreateThread, PostToThread, PostMessage
    ThreadName string              // for CreateThread: name of the new thread
    ThreadRef  string              // for PostToThread: references an earlier CreateThread by ThreadName
    Message    NotifyMessage
    Ping       bool                // @here notification
}

type ActionType string

const (
    CreateThread ActionType = "create_thread"  // opens a new Discord thread
    PostToThread ActionType = "post_to_thread"  // replies in an existing thread
    PostMessage  ActionType = "post_message"    // standalone message (no thread)
)

type NotifyMessage struct {
    Content string
    Embeds  []Embed
}
```

A `ValidateActions(actions []NotifyAction) error` function checks: every `PostToThread.ThreadRef` matches a preceding `CreateThread.ThreadName`, `CreateThread` actions have non-empty `ThreadName`, `PostMessage` actions have no thread fields set.

### How comedy uses this

Returns a single `PostMessage` action per evaluation — one embed with the week's picks.

### How powder uses this

Returns a sequence of actions:
1. `CreateThread` — briefing embed (synthesized from Briefer) with thread name like "PNW Cascades — Jan 15-17"
2. `PostToThread` x N — one detail embed per region, posted to the thread created in step 1
3. On re-evaluation: `PostToThread` with update embeds + conditional `Ping: true` on tier escalation

The core Discord client processes actions in order, tracking thread IDs from `CreateThread` to route subsequent `PostToThread` actions.

### Thread State

The core stores thread IDs so updates go to the right place:

```sql
notification_threads (hunt_name, group_key, thread_id, created_at,
                      UNIQUE(hunt_name, group_key))
```

When the formatter returns `PostToThread` for an existing group_key, the notifier looks up the thread ID. If the thread doesn't exist yet (first notification), it creates one. The UNIQUE constraint prevents duplicate thread creation under any race condition.

## Web UI

Shared template shell with tabs per hunt. One HTML template renders all hunts via `CardData`:

```go
type ScoreTier string

const (
    ScoreHigh   ScoreTier = "high"
    ScoreMedium ScoreTier = "medium"
    ScoreLow    ScoreTier = "low"
    ScoreNone   ScoreTier = "none"
)

type CardData struct {
    Title       string
    Subtitle    string
    Score       string
    ScoreTier   ScoreTier   // core constant, template maps to CSS
    Reason      string
    Urgency     string
    Fields      []CardField // hunt-specific detail rows
    ActionURL   string
    ActionLabel string
}

type CardField struct {
    Icon  string
    Label string
    Value string
}
```

Comedy adds fields for walking distance, price. Performing arts adds reviews, awards. Powder adds snowfall, conditions, drive time. The template iterates `Fields` without knowing what they mean.

The web UI also displays:
- **Last run status** from `PipelineResult` — when it last ran, how many items processed, any errors
- **Per-hunt error history** — consecutive failures highlighted

### Feedback

Each hunt declares its options via `WebHunt`:

```go
type FeedbackOption struct {
    Value string  // stored in DB
    Label string  // displayed to user
}
```

- Comedy: `loved`, `not_for_me`
- Performing arts: `loved`, `not_for_me`, `already_seen`
- Powder: `went_great`, `went_ok`, `skipped`
- Movies: `loved`, `good`, `meh`, `not_for_me`

Core renders buttons, handles POST, stores `(opportunity_id, title, rating, note)`. The `opportunity_id` is nullable — movies supports rating films that weren't scanned (manual feedback via the web UI's "Rate a movie" form). Hunts that don't implement `WebHunt` get no feedback buttons and a generic card renderer.

## Storage

One SQLite database. Core tables are universal; hunt-specific data lives in `attributes` JSON columns with `CHECK(json_valid(attributes))` constraints.

```sql
-- Venues are shared across hunts, normalized on upsert
venues (id, name, address, latitude, longitude, notes,
        UNIQUE(name, address))

opportunities (id, hunt_name, source_id, source, title, subtitle, venue_id,
               start_time, end_time, price_min, price_max, ticket_url,
               state, attributes JSON CHECK(json_valid(attributes)),
               raw_data, discovered_at, evaluated_at,
               notified_at, reminded_at)

-- Evaluations (picks are in a separate table)
evaluations (id, hunt_name, group_key, evaluated_at,
             skipped_reasoning, raw_llm_response, rendered_prompt, cost_usd)

-- Normalized picks table
picks (id, evaluation_id, opportunity_id,
       score, display_score, reason, urgency,
       attributes JSON CHECK(json_valid(attributes)),
       FOREIGN KEY (evaluation_id) REFERENCES evaluations(id),
       FOREIGN KEY (opportunity_id) REFERENCES opportunities(id))

feedback (id, opportunity_id, hunt_name, title, rating, note, created_at)
-- opportunity_id is nullable: movies supports rating films not scanned by the system
-- title is always set: used directly in eval prompts without joining to opportunities

preferences (id, hunt_name, preferences_text)

-- Distance cache supports multiple travel modes (walking, driving)
distance_cache (venue_id, home_address, mode, minutes, distance_mi, created_at,
                UNIQUE(venue_id, home_address, mode))

-- Thread tracking for Discord conversations
notification_threads (hunt_name, group_key, thread_id, created_at,
                      UNIQUE(hunt_name, group_key))

eval_costs (id, hunt_name, evaluated_at, cost_usd, model, success)
```

## Cost Tracking

CostTracker is seeded from the `eval_costs` table on startup so budget gating survives restarts. Fields are unexported; access is through methods only. No mutex needed since hunts run sequentially.

```go
type CostTracker struct {
    total  float64
    byHunt map[string]float64
}

func NewCostTracker(db *storage.DB) (*CostTracker, error) {
    monthly, err := db.MonthlySpend(time.Now())
    if err != nil { return nil, err }
    return &CostTracker{
        total:  monthly.Total,
        byHunt: monthly.ByHunt,
    }, nil
}

func (ct *CostTracker) Add(hunt string, cost float64) { ... }
func (ct *CostTracker) Total() float64                { ... }
func (ct *CostTracker) ForHunt(hunt string) float64   { ... }
```

Budget gating is a field on `Schedule`, not a separate interface:

```go
type Schedule struct {
    ScanInterval       time.Duration   // how often to scan for new opportunities
    EvalInterval       time.Duration   // how often to run evaluation (0 = after every scan)
    RemindBefore       []time.Duration // send reminders this far before start_time
    MaxMonthlySpendUSD *float64        // nil = no limit
}
```

## Schedules

- Comedy: scan every 12h, eval weekly, remind 1 day before
- Performing arts: scan every 12h, eval weekly, remind 1 week + 1 day before
- Powder: scan every 12h, eval after every scan (hunt-side gating via ReEvaluator), remind 2 days before, $10/month budget cap
- Movies: scan every 24h, eval weekly, remind 1 day before (theatrical only, streaming has no reminders)

## Configuration

```env
# Core
DB_PATH=/data/opportunity-hunter.db
GOOGLE_API_KEY=...
HOME_LATITUDE=39.75
HOME_LONGITUDE=-104.99
HOME_ADDRESS="123 Main St, Denver, CO"
WEB_PORT=8080
DRY_RUN=false

# Error alerts
ERROR_DISCORD_WEBHOOK_URL=...

# Per-hunt toggles
HUNT_COMEDY_ENABLED=true
HUNT_PERFORMING_ENABLED=true
HUNT_POWDER_ENABLED=true
HUNT_MOVIES_ENABLED=true

# Per-hunt Discord webhooks
COMEDY_DISCORD_WEBHOOK_URL=...
PERFORMING_DISCORD_WEBHOOK_URL=...
POWDER_DISCORD_WEBHOOK_URL=...
MOVIES_DISCORD_WEBHOOK_URL=...

# Source API keys (hunts validate in Init)
TICKETMASTER_API_KEY=...
EVENTBRITE_API_TOKEN=...
TMDB_API_KEY=...
```

Hunts validate their config requirements in `Init()`. If a required key is missing and the hunt is enabled, Init returns an error with a clear message and the hunt is not started.

## Supporting Types

```go
// RawItem is the normalized output from any Source, before deduplication.
type RawItem struct {
    SourceID       string
    Source         string
    Title          string
    Subtitle       string
    VenueName      string
    VenueAddress   string
    VenueLatitude  float64
    VenueLongitude float64
    StartTime      string   // RFC3339
    EndTime        string   // RFC3339, empty for single events
    PriceMin       *float64
    PriceMax       *float64
    TicketURL      string
    RawJSON        string
    Attributes     Attributes // source-specific extras
}

// Source fetches raw items from an external provider.
type Source interface {
    Name() string
    Scan(ctx context.Context, region ScanRegion) ([]RawItem, error)
}

type ScanRegion struct {
    Latitude  float64
    Longitude float64
    RadiusMi  int
}

// Group is a batch of opportunities evaluated together.
type Group struct {
    Key           string
    Opportunities []Opportunity
    Venues        map[int64]Venue
}

// NotifyGroup is a batch of evaluations notified together (post-eval grouping).
type NotifyGroup struct {
    Key         string
    Evaluations []Evaluation
}

// FeedbackEntry is a single piece of user feedback, used in eval prompts.
type FeedbackEntry struct {
    OpportunityTitle string
    Rating           string
    Note             string
}

// CardRenderer converts picks into display cards for the web UI.
type CardRenderer interface {
    RenderCard(opp Opportunity, pick Pick, venue Venue) CardData
}
```

## Testing

### Test infrastructure

```
hunts/fake/
    hunt.go           // FakeHunt implementing all interfaces with configurable behavior

testutil/
    db.go             // NewTestDB() *storage.DB (in-memory SQLite)
    evaluator.go      // FakeEvaluator returning canned picks
    notifier.go       // FakeNotifier recording sent messages
```

**FakeHunt** implements all required and optional interfaces with configurable behavior:
- Configurable sources that return canned RawItems
- Configurable evaluator that returns canned picks or errors
- Records all calls for assertion

**Testing patterns**:
- Pipeline integration tests use FakeHunt + in-memory SQLite
- Hunt implementations test their evaluator/formatter logic in isolation
- Source implementations test against recorded API responses
- No mocking frameworks; use fakes and test doubles

## Project Structure

```
opportunity-hunter/
├── cmd/opportunity-hunter/main.go
├── core/                     # Universal types & interfaces
│   ├── opportunity.go        # Opportunity, State, Attributes, RawItem
│   ├── hunt.go               # Hunt + all optional interfaces + ValidateHunt
│   ├── evaluation.go         # Evaluator, EvalContext, Evaluation, Pick
│   ├── notify.go             # NotifyFormatter, NotifyAction, NotifyMessage, ValidateActions
│   ├── web.go                # CardRenderer, CardData, ScoreTier, FeedbackOption
│   ├── source.go             # Source, ScanRegion
│   ├── cost.go               # CostTracker
│   ├── pipeline.go           # PipelineResult, HuntResult, StepError
│   └── retry.go              # withRetry helper
├── pipeline/pipeline.go      # Framework orchestration
├── storage/                  # SQLite layer (scoped by hunt_name)
│   ├── sqlite.go
│   ├── schema.sql
│   ├── opportunities.go
│   ├── evaluations.go
│   ├── picks.go
│   ├── venues.go
│   ├── feedback.go
│   ├── preferences.go
│   ├── distance.go
│   ├── threads.go
│   └── costs.go
├── web/                      # Shared UI shell
│   ├── web.go
│   └── templates/
│       ├── layout.html
│       ├── cards.html
│       ├── preferences.html
│       ├── feedback.html
│       └── status.html       # pipeline run status
├── notify/                   # Discord client (thread-aware + error alerts)
│   ├── discord.go
│   └── notifier.go
├── llm/                      # Gemini two-step client
│   ├── gemini.go
│   └── cost.go
├── distance/distance.go      # Walking/driving cache (mode-aware)
├── config/config.go
├── hunts/
│   ├── fake/                 # Test double
│   │   └── hunt.go
│   ├── comedy/               # Comedy hunt implementation
│   │   ├── hunt.go           # implements Hunt + Grouper + WebHunt + NotifyHunt
│   │   ├── evaluator.go
│   │   ├── prompt.go
│   │   ├── attrs.go          # ComedyAttrs typed struct
│   │   ├── cards.go
│   │   ├── notify.go
│   │   └── sources/
│   ├── performing/           # Performing arts hunt
│   │   ├── hunt.go           # implements Hunt + Grouper + WebHunt + NotifyHunt
│   │   ├── evaluator.go
│   │   ├── prompt.go
│   │   ├── attrs.go
│   │   ├── cards.go
│   │   ├── notify.go
│   │   └── sources/
│   ├── movies/               # Movies hunt
│   │   ├── hunt.go           # implements Hunt + Expirer + WebHunt + NotifyHunt
│   │   ├── evaluator.go
│   │   ├── prompt.go
│   │   ├── attrs.go          # MovieAttrs typed struct
│   │   ├── cards.go
│   │   ├── notify.go
│   │   └── sources/
│   │       ├── tmdb.go       # TMDB API (theatrical + streaming releases)
│   │       ├── letterboxd.go # Letterboxd scraper (trending/popular)
│   │       └── ticketmaster.go # Local special screenings
│   └── powder/               # Powder hunt
│       ├── hunt.go           # implements Hunt + ReEvaluator + Briefer + WebHunt + NotifyHunt
│       ├── evaluator.go
│       ├── prompt.go
│       ├── attrs.go          # PowderAttrs typed struct
│       ├── grouping.go       # Briefer: macro-region + friction
│       ├── briefing.go       # Briefer: LLM briefing across regions
│       ├── detection.go      # ReEvaluator: weather change + cooldown
│       ├── cards.go
│       ├── notify.go         # thread-based with update support
│       ├── catalog/
│       └── sources/
├── testutil/                 # Shared test infrastructure
│   ├── db.go
│   ├── evaluator.go
│   └── notifier.go
├── Makefile
├── Dockerfile
├── docker-compose.yml
├── CONTRIBUTING.md
└── .env.example
```

## Migration Path

1. Build fresh repo with `core/` types + `pipeline/` + `storage/` + `web/` shell + `testutil/`
2. Implement performing arts hunt (new, validates interfaces without porting baggage)
3. Port comedy-hunter in (closest to core model, proves Grouper works)
4. Implement movies hunt (new, proves Expirer + nullable VenueID + manual feedback)
5. Port powder-hunter in (most complex, proves ReEvaluator + Briefer)
6. Retire old repos

Expect at least one interface revision after step 3 before tackling step 4-5.

## Open-Source Model

Contributors add a package under `hunts/`, implement the `Hunt` interface (and any optional interfaces they need), and add one import line in `main.go`. Users enable/disable hunts via env vars and provide whatever API keys the hunt needs.

A minimal hunt implements 6 methods (Name, Init, Sources, DedupeKey, Evaluator, DefaultSchedule). A complex hunt like powder implements up to 12, opting in to re-evaluation, briefing, custom cards, and threaded notifications. A medium hunt like movies implements 9, adding custom expiration, cards, and notifications.

See `CONTRIBUTING.md` for a walkthrough of adding a new hunt.
