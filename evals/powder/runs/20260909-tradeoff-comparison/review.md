# Comparative selection: powder advisor v5

## Selection

Promote **tradeoff guidance with the buffer-day refinement** to powder prompt v5. Keep Gemini 3.8 Flash. This is the strongest overall candidate in this development comparison, not a claim of perfection or statistically established superiority.

The user's acceptance criterion is useful judgment under uncertainty: identify a worthwhile bet, explain its upside and what time/money might be lost, compare the alternative, and provide an actionable contingency. Uncertainty need not force Watch. We evaluated whole answers against the current prompt, rather than retaining the current prompt whenever a candidate contained an error.

## Matched comparison

Four synthetic cases: Japan resort alternative, unreachable Cascade powder followed by rain, Steamboat reopening, and a new White Pass reopening that may slip one day with a Saturday extension available. The first three were already development cases; this is not an unseen historical holdout. The final refinement used observed failures, so it is development evidence too.

The current baseline uses the actual frozen v4 powder template plus shared decision instructions and the actual powder extraction schema in the research prompt. Candidates use exactly the same rendered evidence sections, but concise research instructions without the schema. Evidence hashes match across all variants within each case. Changing prompt wording and removing the research schema is a bundled product comparison; this does not isolate which component causes the difference. Search is enabled for durable information, with hypothetical weather/operations explicitly stipulated. Grounding metadata is not proof of source support for a hypothetical claim.

The first round used current, tradeoff and tradeoff-with-example variants (12 assessments). A read-only reviewer initially saw only anonymized answers and evidence. Mapping: A=example, B=current, C=tradeoff. The reviewer preferred C>A>B in Japan, rain and Steamboat; A narrowly won the delayed White Pass case because it evaluated the extra-night option. The coordinator independently agreed with the overall direction.

The example answer's cancellation logic contradicted its buffer-day plan. We therefore kept C and added a short instruction to weigh buying time, explain the remaining payoff after a delay, and make the fallback consistent with delays already allowed for. Six follow-up assessments covered all four cases plus independent repeats of White Pass and Steamboat.

## What improved

- **Decisions became easier to use.** Current answers ran 645–1,231 words; selected-candidate answers were 315–350 words. They organize around a recommendation, plausible payoff, downside, strongest alternative and decision deadline, rather than filling every schema field in prose.
- **Japan:** A Rusutsu Wednesday option is weighed against waiting for Niseko Thursday. The answer supplies a morning road/operations decision point instead of burying the plan in a resort encyclopedia.
- **Rain:** All variants reject travel for the unreachable powder session followed by rain/refreeze. The concise answer retains the decisive reason.
- **Steamboat:** A two-day trip captures potential reopening value and a fresh Saturday session; Saturday-only is explicitly compared as a way to save PTO/lodging.
- **Delayed White Pass:** Both selected-candidate repeats recommend considering the larger storm with a Saturday buffer. They explain losing Thursday's skiing, adding a night/personal day, and canceling for a longer road/access failure. This is substantially more useful for this user than the current/C answers' categorical Skip without fully evaluating the buffer option.

The reviewer narrowly preferred the refined candidate overall: improvement in the delayed-reopening decision outweighed some regressions in factual restraint. The coordinator selected it on that comparative basis.

## Imperfections retained, not hidden

The model still sometimes promotes forecasts into confident surface/operating claims. Both refined Steamboat repeats incorrectly suggest Saturday will have full operations; an extra day does not ensure that. Some Japan claims overstate shelter, lift reliability and low crowds. White Pass outputs invent spending precision or imply guaranteed days of powder. Those are decision-relevant errors, not merely tone, and the prompt explicitly discourages them without eliminating them.

The decision to promote is relative: the existing prompt also makes substantial unsupported claims while offering a less usable decision. We did not require a particular tier; Recommended/Drop Everything variation is acceptable when analysis supports the choice. No new mandatory snow thresholds or deterministic grading rules were added.

## Implementation

`hunts/powder/prompt.go` is v5.0.0 with the selected guidance and existing evidence, profile, feedback and history sections. The exhaustive every-resort research checklist was removed in favor of selective decision-changing research.

`powderEvaluator` calls `TwoStepAdvice`: research sees the self-contained advisor prompt; the schema is supplied only during extraction. The same shared implementation still enables Google Search, medium research thinking, low extraction thinking, local schema validation, retries, source handling and cost accounting. Other hunts retain their existing `TwoStep` research behavior. Research and structured responses remain separately persisted by the existing flow.

The selected research prefix is the exact tested `ADVISOR + BALANCE` string. The app's dynamic evidence formatting remains intact. HTTP regression tests check schema-free grounded research and schema-based extraction, original context, source URLs and conditional-summary instructions. Existing powder tests cover retained preferences and verdict/pick mapping.

## Validation and cost

All 18 comparative research calls and 18 extraction calls ended STOP; exact prompts, evidence hashes, outputs and grounding metadata are retained. Their estimated cost was $0.7528, conservatively charging reported search queries at the app's rates. These estimates are not invoice totals.

Two initial production-path smoke runs validated transport but **are excluded from decision-quality conclusions**: the temporary fixture mistakenly put operational context in an AFD that the snow-day filter omitted. These remain as `*-production.json` to document the mistake. The corrected fixture supplies scenario context outside that filter, asserts traveler/operation text reaches the rendered prompt before calling the model, removes fabricated zero-elevation resort metadata, and saves separate `*-production-context.json` records. The input uses explicitly labeled nominal temperature/SLR, so these are synthetic integration checks, not weather replays.

Both corrected live calls through `powderEvaluator.Evaluate` passed schema validation and produced a pick with the research verdict/recommendation. White Pass was Recommended with a Thursday-loss/Friday-payoff discussion and optional Saturday extension; Steamboat was Drop Everything with staged-access risks. Their precise wording still overstates some costs, snow preservation and crowd expectations, as in the comparison. These are integration/fidelity checks, not independent evidence that the prompt is always accurate. The actual HTTP extraction receives the source list and the original context as verified by regression tests.

The corrected smoke runs cost an estimated $0.0378; the two excluded fixture runs cost $0.0469. Total for this pass including excluded runs was approximately **$0.84**. Full `go test ./...` and `go vet ./...` passed. The paid smoke-test source is retained as `production_smoke_test.go.txt`, outside the normal test suite; it requires an explicit environment opt-in if restored for another run.

All work is local. No production database, notifications, Unraid deployment or model-default change was made.

## Code review disposition

The independent code review verified v5 wiring, research-schema separation, shared search/extraction/validation/accounting, and unchanged research behavior for other hunts. It also identified the initial smoke fixture's filtered AFD, already corrected and excluded above. That fixture does not establish a new production regression: both real providers still populate calendar-day snowfall alongside session totals. We did not change AFD selection to make an artificial fixture pass. A broader decision to retain AFDs for session-only imported data would be separate scope. The harness intentionally preserves failed outputs rather than automatically issuing more paid calls; none of these comparison calls failed.
