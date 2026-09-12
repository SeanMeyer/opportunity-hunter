# Review evidence backfill

One-time enrichment of currently visible comedy, movie and performing-arts picks. Existing evidence and completed backfill audits (including empty results) are skipped. Selection uses the same states, canonical pick ordering and expiration policy as the web cards.

1. Take a consistent SQLite snapshot and a fresh production backup. Keep snapshot, checkpoint directory and binary in a private operator-owned directory.
2. Build `go build -o backfill-reviews ./cmd/backfill-reviews`.
3. Plan on the snapshot: `backfill-reviews -mode plan -db source.db -output results`.
4. Set GOOGLE_API_KEY and optionally GEMINI_MODEL through the environment. Run `backfill-reviews -mode research -db source.db -output results -workers 3 -max-cost 10`.
5. Inspect results and apply to a disposable database copy: `backfill-reviews -mode apply -db verify.db -output results`. Compare all existing judgments, feedback and delivery tables before/after and verify a second apply is a no-op.
6. Run the same apply command with the production database path. It needs no API key or network access. The existing app can remain running. Check resulting live cards and System Status.

Research is independent of user taste scoring and notification delivery. It never calls the pipeline. An apply transaction modifies only pick attributes, `review_backfills` audit rows, and `eval_costs`. Existing evaluations, dates, scores, reasons, votes and delivery checkpoints are retained. Audit rows contain the original record and full grounded research/extraction. Successful research costs enter the existing monthly cost display once, dated to checkpoint completion. Prior failed-attempt costs are attributed to that successful completion date; cross-month failure attribution is approximate.

Each completed result is saved atomically to PICK_ID.json. Resume research using the same directory; saved results do not trigger another API call. Failed attempts are kept as failed-PICK_ID-TIMESTAMP.json, included in the scheduling threshold and carried into the eventual successful apply cost. If an attempt never succeeds or a stale record cannot be applied, its cost remains recorded in the checkpoint files rather than the production monthly total. Provider retries/errors may incur costs not reported by the API; totals are estimates. The threshold counts accumulated successful and failed checkpoint costs across resumes. It stops new scheduling, not requests already in flight.

Apply rechecks current eligibility and compares opportunity identity and pick attributes. The transaction also rejects a newer pick, changed title/subtitle/source ID/attributes, superseded opportunity, changed state, or previous completion. Mismatched checkpoint files cause the command to stop for operator inspection, rather than silently reusing their evidence. Interrupted runs can resume without duplicate metadata updates or costs. Apply is atomic per pick, not across the batch. Missing checkpoints or concurrent changes are reported with a nonzero exit; newly discovered picks and picks beyond a previous research limit can legitimately cause that result. Inspect the plan before resuming.

An empty supported result is a completed search, not a promise no evidence exists anywhere. This command does not fetch arbitrary review pages or guarantee every AI paraphrase is correct. Links are selected from the existing grounded-search pipeline, with the same exact-host YouTube validation and bounded Google redirect resolution as normal evaluations.
