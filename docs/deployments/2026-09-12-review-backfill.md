# Production review evidence backfill — September 12, 2026

Applied at 19:10 UTC to the existing Unraid v0.3.9 application without restarting it.

| Hunt | Searched/applied | With evidence | With clips |
| --- | ---: | ---: | ---: |
| Comedy | 59 | 55 | 51 |
| Movies | 4 | 4 | 0 |
| Performing arts | 28 | 25 | 3 |
| Total | 91 | 84 | 54 |

Estimated Gemini cost: $5.2575005. Three concurrent grounded research requests; no failed requests. Existing Chris Fleming evidence was skipped. Seven empty searches have completion audits so another apply does not repeat them. These results describe the researched artists, films, or identified past productions; a past cast review is not a review of the upcoming cast.

## Safety and verification

Research ran against a consistent snapshot in a separate temporary container. Apply ran with network disabled and no API or notification credentials. Temporary research environment file was removed after completion.

Retained private operator directory on tower:
`/mnt/user/appdata/opportunity-hunter-backups/reviews-20260912T185507Z`

Immediate pre-apply backup: `before-apply.db`; research snapshot: `source.db`; per-pick checkpoints: `results/`; applied helper: `backfill-reviews-v4`. The source for rebuilding the helper is `cmd/backfill-reviews`.

All 91 results first applied to a disposable production copy. Fingerprints verified every original table except allowed pick attributes and newly inserted audit/cost rows. Every original pick attribute field was also compared semantically and preserved, including previously populated review evidence. Existing evaluation costs were unchanged.

The same checks passed against production after applying: 91 audit rows, 91 new cost rows, unchanged original judgments, opportunity dates/states, feedback and delivery records, and SQLite integrity check OK. A second apply on both copy and production reported zero candidates and changed no costs or data.

Full `go test ./...` and `go vet ./...` passed. Targeted tests cover expired/already enriched selection, stale pick and changed identity guards, idempotence, failed-attempt cost retention, research date attribution, and indented/HTML-escaped checkpoint compatibility.

Live comedy (Mark Normand), movies, and performing arts (Jersey Boys) cards were inspected using browser screenshots. All three pages and System Status returned HTTP 200; app running with zero restarts. Latest scheduled runs showed OK. No new scans or Discord notifications were triggered by this backfill.

## Independent review dispositions

A focused read-only agent review and independent Gemini CLI review found failure-cost loss, incomplete identity guards, checkpoint JSON encoding mismatches, date attribution, and incomplete apply reporting. These were corrected and tested. Final focused review found no blocker. Per-pick atomic application is intentional for resumption; missing checkpoints from newer recommendations or limited research are explicitly reported rather than silently treated as complete.

Research links use the existing grounded-source validation. A source-content spot check supported the JoBlo movie blurb; several other direct page opens were blocked. This was not a manual verification of every external link or every AI paraphrase.

Pre-existing visual/data issues observed during spot checks, outside this metadata-only operation: some symphony titles contain replacement question marks, and historical run history retains prior powder notification errors. Neither was introduced or changed by the backfill.
