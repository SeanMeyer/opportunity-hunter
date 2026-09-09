# Product Quality Validation Plan

Goal: exercise recommendation quality and actual usage across all four hunts,
fix reproducible defects, and preserve repeatable evidence. The user approved
the five areas in the conversation: recommendations, feedback sensitivity,
complete journeys, failure/recovery, and a monitored real-source trial.

Architecture: parallel investigators use isolated synthetic data and public
fixtures; the coordinator owns production changes and Git integration. Keep
normal tests offline. Live tests are explicitly invoked with bounded call counts,
deadlines, recorded inputs/outputs/model/cost, and no external notifications.

## Workstreams

- [x] Confirm previous work is on main, fetch origin, run baseline tests, push.
- [x] Run up to 12 production evaluator calls across comedy/movies/performing,
  including paired feedback cases, hard constraints and weak/unknown options.
- [x] Run up to 6 production powder evaluations for access, timing, weather
  uncertainty and preference sensitivity; inspect extraction fidelity.
- [x] Reproduce partial failures, persisted state, notification recovery and
  overlapping/restarted runs using SQLite and local test doubles.
- [x] Browse realistic cards across hunts at desktop/mobile sizes; exercise
  filters, feedback, preferences, schedule and manual-run/status interactions.
- [x] Add regression tests before each confirmed fix, implement one writer at
  a time, and rerun affected tests. Review source and rendered changes.
- [x] Run full offline tests, vet, build and a tracked-file credential scan;
  commit and push verified fixes and evidence to main.
- [x] Establish a bounded isolated real-source trial across scheduled cycles,
  with captured notifications and explicit completion/stop conditions. Record
  missing credentials or provider access as coverage gaps, not product success.

## Limits and interpretation

Initial live evaluator allowance: at most 18 two-step evaluations, estimated
model cost capped at $1 per live workstream. Additional calls require a concrete
failed case or a bounded end-to-end verification need. Secrets stay in ignored
local configuration and are excluded from prompts and committed artifacts.
Subjective judgments are observations; hard constraints and state invariants
are assertions. No small benchmark establishes universal recommendation quality.

The delayed trial is established, not complete: two immediate cycles passed;
two additional six-hour cycles are scheduled. See docs/product-quality-validation.md
for findings, exact evidence, and coverage limitations.
