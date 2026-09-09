# AI evaluation

All hunts use `GEMINI_MODEL` (default `gemini-3.8-flash`). Gemini 3 research uses medium thinking; extraction and powder briefing use low. Older model overrides retain the provider's default reasoning configuration. The current Go SDK supports the request fields used here.

The first call researches with Google Search and makes the actual recommendation. It receives the opportunity, preferences, feedback, applicable history, and an output contract as context. Its response is prose: the contract does not constrain decoding on this pass. Scoring guidance is calibration, with explicit permission to consider other relevant factors, weigh tradeoffs, and recommend nothing. Explicit user constraints still apply.

The second call extracts the assessment into JSON. It receives both original context and research, must preserve the judgment and uncertainty, and can use "Unknown", "Not applicable", or empty arrays for unsupported details. It must not invent prices, availability, sources, or IDs. The client validates the returned structure locally and rejects empty or interrupted responses. JSON validity does not establish factual accuracy.

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
