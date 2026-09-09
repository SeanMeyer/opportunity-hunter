# AI evaluation

## Wind context

New forecasts retain wind direction from Open-Meteo and NWS alongside speed and gusts. The powder weather table shows source/model/fetch time and day/night direction plus gusts. Bearings describe where wind comes **from**, clockwise from true north; they do not establish terrain shelter or lift access.

Direction is an equal-hour circular mean excluding hours with known nonpositive wind speed, so 350° and 10° summarize as north. A resultant length below 0.5 is shown as Variable rather than a misleading mean for substantially opposing/changing winds. Missing, null or invalid bearings remain Unknown; a real 0° is north. Older saved forecast JSON remains readable and displays Unknown for absent direction. New values persist inside the existing forecast snapshot JSON, without a database migration.

The legacy Night bucket combines 00:00–06:00 and 18:00–24:00 on the row's local date. It is **not** the following morning. Wind summaries retain those explicitly labeled periods. The AI snowfall table now uses separate ski-session calculations instead of the ambiguous Day/Night snow buckets.

## Snow available for a ski session

For each ski date, **Opening snow** totals the previous local day's 16:00 close through that date's 09:00 opening. **During skiing** totals 09:00–16:00. These are clearly labeled default hours, not verified resort operating schedules. Calendar-day totals remain available for storm detection; they are not added to the opening total.

Both providers calculate these new fields from hourly snowfall rather than recombining the old aggregates. Open-Meteo precipitation timestamps mark the end of the preceding accumulation hour; NWS intervals start at `validTime`. NWS temperature, wind and cloud values are held through their validity intervals, not divided by interval duration. Daylight-saving boundaries use the resort timezone when supplied by the provider.

Every hour of a period must be available for a complete total. Missing/null inputs or a truncated first forecast day produce Unknown, while a fully covered dry period produces zero. Older snapshots lack the new `SkiSession` field and remain readable, but their opening totals display Unknown: the necessary hourly timing cannot be recovered from a mixed Night bucket. New snapshots persist the session totals and assumed hours without a database migration.

These totals are snowfall estimates, not guarantees of preserved or untouched snow. Actual opening schedules, prolonged closures and delayed terrain releases remain separate evidence for the advisor to assess.

Direction parsing tests cover null versus north, wraparound, opposing bearings, model-specific arrays, NWS multi-hour intervals/local time, and JSON round trips. Direction means are unweighted by wind speed and are summaries of available hours, not storm-specific exposure maps.

All hunts use `GEMINI_MODEL` (default `gemini-3.8-flash`). Gemini 3 research uses medium thinking; extraction and powder briefing use low. Older model overrides retain the provider's default reasoning configuration. The current Go SDK supports the request fields used here.

The first call researches with Google Search and makes the actual recommendation. It receives the opportunity, preferences, feedback and applicable history. Powder v5 uses a concise tradeoff advisor: plausible payoff, downside, alternatives, buffer-day cost and consistent fallback decisions. It can recommend a calculated risk without requiring every uncertainty to disappear. Powder's output contract is supplied only to the later extraction pass; other hunts retain their existing research contract context. Explicit user constraints still apply.

The second call extracts the assessment into JSON. It receives both original context and research, must preserve the judgment and uncertainty, and can use "Unknown", "Not applicable", or empty arrays for unsupported details. It must not invent prices, availability, sources, or IDs. The client validates the returned structure locally and rejects empty or interrupted responses. JSON validity does not establish factual accuracy.

Extraction also receives the research call's retrieved source URLs. Its instructions distinguish actual sources from suggested future checks, illustrative value references from travel estimates, and possible reopenings from confirmed operations, including in recommendation and summary fields. These are guardrails, not guarantees of fidelity. The [initial session validation](../evals/powder/runs/20260909-session-validation/review.md) found unresolved errors. The subsequent [matched comparative pass](../evals/powder/runs/20260909-tradeoff-comparison/review.md) selected v5 for stronger overall human decision support than the current prompt, while documenting its remaining overconfidence.

Research and extracted decisions are saved separately in SQLite, and the exact research prompt is retained for inspection. Existing databases gain an empty `structured_response` column on startup without changing existing records. Old powder picks remain readable and supply history when older research was only prose. History is looked up by opportunity, so overlapping windows in the same region do not replace one another's judgments.

## Powder verdicts

| Verdict | Meaning | Notification behavior |
| --- | --- | --- |
| Drop Everything | Exceptional opportunity; make this happen | Full alert, eligible for a ping |
| Recommended | Yes, this looks good for this person | Normal alert, no ping |
| Watch | Promising but not ready to recommend | Informational alert, no ping |
| Skip | Not worth pursuing or unsuitable | Visible in the UI; no fresh alert or reminder. A downgrade still sends a note. |

Cards lead with the overall recommendation and sort by verdict. Snow, costs, resort details, and risks remain supporting information. Old `WORTH_A_LOOK` and `ON_THE_RADAR` values display as Recommended and Watch. Existing filters continue to work. No stored tiers are rewritten. Alerts can post into a saved Discord thread, and exceptional alerts insert a single mention through the notifier.

Snowfall detection thresholds remain the initial cost-control filter; the AI cannot evaluate storms that do not pass it. No new numerical scoring formula has been added. Comedy, movies, and performing arts keep their 1–10 scales and can return no recommendations; all share the improved research and feedback handling.

## Costs and verification

Estimates include candidate and thinking tokens and conservatively charge every reported search, without subtracting the account-wide free search allowance. Gemini 3.8 Flash uses $0.75/$3.75 per million input/output tokens through 2026, then $1.50/$7.50 beginning January 1, 2027. Search is estimated at $14 per 1,000 reported queries. Gemini 2.5 Flash/Pro token prices are also recognized; unlisted overrides log a warning and use fallback rates. These are application estimates, not invoice totals or a hard billing cap. See [Google pricing](https://ai.google.dev/gemini-api/docs/pricing).

Run `go test ./...` and `go vet ./...` for local checks. HTTP fixtures verify the model contract, reasoning configuration, extraction context, missing output rejection, usage calculations, and posting to saved Discord threads. Other tests cover context retention, feedback polarity, four verdicts, skip/downgrade alerts, filtering/sorting, persistence, and independent history for overlapping storm windows. Successful briefing costs are persisted as well as evaluation costs, so they survive tracker reconstruction after restart.

Before judging live model quality, compare a small set of historical opportunities with the old and new model/prompts: an exceptional local day, an ordinary worthwhile trip, an uncertain long-range storm, and impressive snow with closed access or prohibitive travel. Check factual support, judgment, uncertainty, extraction fidelity, latency, and cost. Local tests do not measure model quality. Use an isolated database and no notification webhooks for this comparison; the `eval` command otherwise may send real notifications.
