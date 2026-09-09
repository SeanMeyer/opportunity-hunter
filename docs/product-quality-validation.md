# Product quality validation — September 9, 2026

This pass tested production evaluators with synthetic cases, real public sources,
SQLite failure/restart scenarios, and the rendered UI. It does not establish
universal recommendation quality or validate every upstream provider.

## Live evaluation results

26 two-step evaluations used `gemini-3.8-flash` (52 HTTP requests, approximately
$0.42687 estimated model cost). Exact inputs, prompts, responses, costs and
assessments are in [the evidence directory](../evals/product/runs/2026-09-09/).
No external notifications were sent. The provided credential remains ignored
local configuration; the captured artifacts passed a credential-pattern scan.

- Initial 12 event evaluations: feedback changed the preferred item in comedy,
  movies and performing arts; repeated cases preserved ranking. All three
  no-good cases returned empty picks. Movie constraint tests initially had
  descriptive titles, so they were not accepted as proof of constraint handling.
- Initial six powder evaluations: closed/inaccessible cases were skipped, strong
  accessible snow recommended, and negative feedback reduced interest in the same
  moderate conditions. Several weather/base/alternative claims lacked evidence.
- Three focused event reruns after fixes used neutral movie titles. Supplied
  prices, dates, late hours and travel limits correctly excluded unsuitable
  options. Unknown dates remained unknown. Performing arts distinguished work
  acclaim from an unknown production. A final comedy run confirmed `unknown`
  sellout risk instead of inventing a low-risk rating.
- Three powder reruns and one final v5.0.2 run showed improved uncertainty
  preservation. The final case stopped inventing alternative forecasts, exact
  deadlines and retained-powder guarantees. Some prose still overstated conditions
  (no wind slab/crusting, an “easy” drive). This remains a quality limitation;
  the final improvement was measured on one case, not the entire scenario set.

## Fixes and verification

- Durable notification intents commit with evaluations, picks and opportunity
  state. Failed saves cannot notify. Delivery recovery does not require another
  AI evaluation; zero-action decisions finish without endless retry. Each
  acknowledged action and its resolved thread IDs are checkpointed, so a later
  detail-post failure does not recreate a successful thread. Dry-run mode retains
  pending deliveries for later real delivery.
- Incurred successful-evaluation cost is recorded before result persistence.
  Scan storage failures are surfaced and canceled runs finalize with a bounded
  uncanceled context. Startup, scheduled and manual runs share a guard; manual
  work is canceled and awaited during daemon shutdown.
- Comedy Works now parses current h2 listings, uses the Landmark venue filter,
  and distinguishes the same performer/date at different venues. A real scan
  changed from zero opportunities before the fix to 82 afterward.
- A complete weather-provider outage is an error rather than an empty successful
  scan. Healthy partial source items remain processable alongside errors.
- Event prompts retain listing price/date/time facts. Date-only values do not
  claim midnight showtimes. Movie release dates are distinguished from screenings;
  theater catalog prices and straight-line distances are conditional context.
  Movie cards show known ticket prices and omit empty release fields.
- Feedback polarity and notes influence subsequent evaluations; latest feedback
  per opportunity is selected before the history limit. This is prompt context,
  not model retraining. Existing results are not retroactively reevaluated.
- Deduplication regression coverage includes restart/source IDs, cross-source
  variants and merged dates; movie titles and powder windows retain hunt-specific
  identity rules rather than inheriting event merging.

Browser checks used actual renderers with clearly synthetic cards. Desktop and
390px mobile views covered all four hunts, thumbs up/down, note editing, cancel,
save/reload, sort preservation, filtering/empty recovery, preferences, schedule
save and the status page. Independent screenshot review found no definite
clipping or overlap; minor remaining polish opportunities are mobile header
height and subdued metadata. HTTP tests cover manual-run and invalid-input paths.

## Remaining coverage boundaries

Ticketmaster and TMDB credentials are absent, so real authenticated event/movie
source ingestion was not exercised. Discord behavior was tested with local
capture/failure doubles, not a real webhook. External acceptance immediately
before a database acknowledgment failure can still lead to duplicate delivery;
exactly-once delivery is not claimed. Older failed evaluations have no delivery
ledger and are not automatically backfilled.

Public-source monitoring uses an isolated database, no AI calls and no external
messages. Immediate and delayed cycles are recorded separately; scheduled future
cycles must not be counted as completed validation. September weather without a
detected storm does not validate real winter storm recommendations.

Two immediate real-source cycles completed with the same isolated database:
82 comedy opportunities inserted initially, then zero duplicates on reopen.
Both weather services returned HTTP 200 in both cycles; no storm was detected.
The `Opportunity Hunter source trial` heartbeat runs every six hours for two
additional cycles and pauses at four total. Local history is
`%TEMP%/oh-quality-source-fixed-20260909/cycles.json`. Run the bounded helper with
`go run ./evals/product/source-cycle OUTPUT_DIRECTORY`; it stops at four records.

Final checks: `go test ./... -count=1`, `go vet ./...`,
`go build ./cmd/opportunity-hunter`, and `git diff --check` passed.

Independent local code and screenshot reviews were performed. The external
Antigravity/Gemini second-opinion attempt was unavailable earlier in the session;
it is not counted as successful review coverage.
