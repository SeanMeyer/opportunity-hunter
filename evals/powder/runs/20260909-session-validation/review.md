# Ski-session prompt validation, September 9, 2026

## Decision

Do not promote a new research prompt or switch models based on this pass. The short prompts produce useful trip shapes, but repeatedly sell uncertain powder preservation and operations as facts. Keep the small extraction change: supply retrieved grounding URLs, distinguish sources used from future research tasks, keep illustrative budgets separate from estimates, and preserve decisive conditions in recommendation and summary.

This is a completed bounded development pass, not a successful general-quality gate. No production research prompt was replaced, no model default changed, and no Unraid deployment or notification occurred.

## Method

Three new **synthetic** scenarios use explicit resort-local opening (previous 16:00 to 09:00) and during-skiing (09:00–16:00) snowfall, plus wind bearings and gusts. They are not reconstructed historical forecasts or observations. Existing parser tests establish hourly aggregation separately; these cases test interpretation of the resulting values, not the entire ingestion pipeline. No historical wind direction or hourly snowfall was invented and presented as real.

The frozen scenarios are in `fixtures.json`: Niseko/Rusutsu alternatives, an unreachable White Pass powder day followed by rain/refreeze, and a Steamboat Friday reopening versus Saturday refresh. No reference verdict is in the model input. We selected these as development stress tests, not a representative holdout sample; later variants were informed by earlier failures.

Google Search was enabled for durable terrain/pass/logistics research. Hypothetical future operations and weather remained stipulated scenario evidence; current pages could not verify them. All experiments used the actual powder output-schema snapshot in `../../session-schema.json` and the app's extraction wording, with the additions recorded in each exact extraction prompt. They are direct REST experiments, not full `powderEvaluator.Evaluate` runs.

| Variant | Research runs | Change |
| --- | ---: | --- |
| Short | 3 Flash | Concise advice plus opportunity/search hints |
| Timing hint | 3 Flash | Same prompt plus one session/preservation sentence |
| Calibrated | 5 Flash | Short plus conditional claims, strongest alternative and source support; **does not include the timing-hint sentence** |
| Thoughtful advisor | 5 Flash | Rewritten framing emphasizing known/inferred/check-before-acting distinctions |
| Calibrated Pro | 2 Pro | Same calibrated prompt, Gemini 3.1 Pro preview instead of Flash 3.8 |
| Extraction counterexamples | 6 Flash | Three human-authored analyses, original versus updated extraction; no research calls |

Total: 18 research calls and 24 extraction calls. All 42 responses ended STOP. All 24 structured outputs passed an offline recursive check of the actual schema's required fields, types, arrays and enums. This verifies transport/structure only. Estimated cost $0.8257 using the app's rates and a conservative charge for 30 reported searches; not an invoice total.

## Findings

**Niseko/Rusutsu:** Both a Wednesday Rusutsu option and a Thursday Niseko reopening opportunity are useful suggestions. The calibrated Flash leads became conditional, but later passages promised preserved snow, near-certain closures, or even “guaranteed” Rusutsu skiing. One fallback moved back to Wednesday after discussing Thursday gate failure, without a clear Wednesday decision deadline. Pro better separated opening/daytime snow and offered a practical two-day option, but its extraction dropped the explicit Rusutsu-lifts-operating condition and converted a possible Thursday reopening into “when upper lifts reopen.”

**Steamboat:** All research variants noticed that 14 inches during a closure might add value beyond the 6-inch opening window. All then overclaimed retained/untracked depth; several also guaranteed light weekday crowds, staged fresh turns throughout Friday, or particular terrain opening. Twenty inches of snowfall is arithmetic, not a measurement of twenty inches available to ski. Pro repeated the central error and called the $800 value reference a budget. DROP_EVERYTHING itself is not the failure: a supported exceptional verdict would be welcome here.

**White Pass:** SKIP was supported by unreachable Friday skiing and forecast summit-level rain/refreeze before Saturday. Answers went beyond that sufficient case, making categorical mountainwide surface claims, introducing arbitrary forecast thresholds, or extending White Pass weather to another resort without its forecast. The tone changes did not consistently remove those claims.

**Extraction:** Verdicts were preserved, but qualifiers sometimes disappeared in summary/best-day fields. The early timing-hint Japan output listed future operations/road checks as research sources. Passing actual retrieved URLs into extraction avoids requiring it to reconstruct source provenance from prose alone. The updated extraction produced Unknown total cost in the calibrated rain and budget counterexamples, and retained the closure counterexample's snowfall-versus-retained-depth distinction in the recommendation. Original extraction also handled several counterexamples correctly; this small sample supports a targeted guardrail, not a claim that extraction fidelity is solved. The updated conditional summary still has awkward timing and should not be mistaken for a perfect answer.

## Source audit

The resort's own [Rusutsu winter page](https://rusutsu.com/en/rusutsu-in-winter/) specifically describes protection from **northwest** winds. That supports investigating shelter, but does not prove lift operation in our **westerly** test scenario. Its [trail map](https://rusutsu.com/en/trail-map/) describes varied terrain, including steep options; treating Rusutsu as only mellow terrain is too simplistic. [Rusutsu's Epic partnership page](https://rusutsu.com/en/epic-pass/) provides primary pass context. These pages were inspected independently after model runs; their contents were not secretly inserted into the fixtures. This was a focused source check, not an audit of every retrieved page or every route/ticket assertion.

## What changed and what remains

`llm/gemini.go` now passes retrieved URLs into extraction and adds brief provenance, budget and conditional-summary guidance. An HTTP regression test checks the actual request contains original context, analysis, schema, grounding URLs and the new instructions. Full Go tests and vet pass. The existing research prompt/model remain in place; an unused experimental research API was removed.

The next useful experiment would separate a specific decision-changing evidence check from the initial advice and test whether a brief factual critique changes the final judgment. Repeating longer reminders in the main prompt did not earn promotion here. This would be a different experiment, not a proven recommendation to add another production call. Retain all failed outputs and avoid forcing verdicts to match reference labels.

## Independent review disposition

The focused reviewer independently identified preserved-depth promises, timing errors and qualifier loss, and advised against promotion. The separate Gemini code/method review prompted an explicit empty `[]` source list when grounding is absent, a distinct extraction-instructions heading, and stronger HTTP assertions. The live extraction counterexamples preceded the heading-only/empty-list normalization; their exact prompts remain recorded.

Grounding metadata can associate hypothetical claims with unrelated retrieved pages; it is not proof that those claims are sourced. This reinforces the limited geography audit above, not a positive source-fidelity score. Research/extraction wording changed together in some variants, so we do not attribute differences solely to extraction; the six human-authored counterexamples isolate extraction. The proposed claim that calibrated code was unreachable was rejected: the separate calibrated scripts invoke that path directly. Each paid script was run in a fresh process, so cross-import mutable globals did not accumulate across this pass. Small sample sizes and changing live search prevent a general Flash-versus-Pro ranking. The harness now rejects non-STOP extraction responses; all recorded responses were already STOP.
