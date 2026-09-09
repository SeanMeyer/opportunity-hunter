# UI cleanup — September 9, 2026

The existing dark card interface now uses one shared stylesheet, readable contrast,
consistent controls, and a compact summary with expandable narrative details.
Numeric scores and verdicts retain their existing meaning. High-score borders now
match the green score badge, and ticket actions have stronger visual emphasis.
Mobile cards stack the score above the title to leave room for long verdicts.

Feedback restores the latest saved rating and note after reload. Cancel restores
the saved selection and returns focus to the originating button. Saving feedback,
preferences, schedules, and requesting a manual run preserve the current sort and
filter. Feedback redirects back to the affected card. Preferences are removed from
keyboard navigation while collapsed, and reason toggles respond to viewport changes.

Handlers reject unknown hunts, invalid ratings, malformed opportunity IDs,
cross-hunt feedback, and malformed schedule times/days. Feedback titles come from
the stored opportunity. Unknown routes return 404, and undated cards sort last.

Validation:

- `go test ./...`, `go vet ./...`, `go build ./...`, and `git diff --check` passed.
- Regression tests cover invalid forms, trusted feedback titles, redirects,
  undated sorting, same-second feedback ordering, rendered saved state, and
  separating narrative details from card summaries.
- A temporary local server with synthetic SQLite data exercised feedback
  save/cancel, preferences and weekly schedule persistence, filter recovery,
  reason expansion, and long detail wrapping at desktop and 390px widths.
  The status page was checked at 320px. No browser console errors were observed.
- An independent read-only reviewer inspected code and desktop/mobile screenshots;
  the focused follow-up found no actionable regressions.
- The separate Gemini review did not return a result after more than seven
  minutes and was stopped. It provides no additional verification evidence.

Live scraping, AI evaluation, external ticket sites, and notification delivery were
not exercised by the browser checks. Manual callback dispatch is covered by a Go
test. The synthetic preview does not establish production data quality.

## Feedback and deduplication follow-up

Evaluations and cards now use the last saved feedback choice per opportunity,
before applying the 20-entry evaluation limit. Earlier choices remain stored;
distinct manual feedback remains eligible. Database read failures are reported
instead of silently omitting feedback from an evaluation.

Multi-date merging is now an explicit hunt capability. Comedy and performing arts
opt in; movies and powder preserve each distinct deduplication key. Existing
date lists only participate in deduplication for hunts that opt in. Source IDs
are also retained when reconstructing existing keys.

Regression checks cover repeated scans, distinct movie titles, separate powder
windows, retained comedy/performing dates, legacy powder date lists, feedback
changes before the limit, and the HTTP feedback-to-next-evaluator path. Replaying
the original probes now yields two distinct powder/movie records followed by
zero new records on an unchanged scan, and only the latest rating at evaluation.

These checks use a test evaluator and do not establish real AI recommendation
quality. Existing incorrectly merged production records have not been rewritten;
their discarded attributes cannot be reconstructed safely without source data.

Follow-up verification passed for `core`, `storage`, `pipeline`, `web`, and all
four hunt packages using `go test -skip Temporary`, plus scoped `go vet` and the
application build. The unrestricted suite encountered concurrent temporary AI
experiment tests: they attempted live calls but failed when writing already
existing output files. Those experiments were excluded from this fix's checks
and do not count as AI-quality validation. A read-only technical review found
no actionable defects in the fixes.
