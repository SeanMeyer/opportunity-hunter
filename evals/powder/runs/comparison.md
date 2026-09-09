# First insight-refinement experiments

September 9, 2026. Gemini 3.8 Flash, medium thinking. Fifteen saved assessments: five cases under each of three prompt variants. These are judgment-only experiments; no web search, schema extraction, server changes or notifications. No production prompts were changed.

## What changed in the evaluation design

The user clarified that an approximately $800 short trip can be worthwhile for exceptionally deep Cascades days with road/resort access confirmed. This is a value reference, not a ceiling or fare quote. The old $1,700–2,400 three-night estimate must not prevent investigating a shorter practical trip.

Closures require interpretation: a season-ending closure can rule out the described visit, whereas a temporary storm closure may create a first-reopening opportunity. Wind may justify searching for another resort, sector or time, but missing direction/terrain/operating evidence cannot establish a safe wind shadow. Insight should connect evidence to an attainable plan, not invent an advantage.

The fifth case is an explicitly hypothetical follow-up to the historical Cascades case, supplying a planned Friday reopening, open road approach and feasible approximately $800 one-night trip. It tests response to updated evidence; it is not a historical discovery.

## Variants and observations

- **Baseline:** Adapted existing powder judgment prompt with the current output schema provided as context. Search replaced by frozen evidence.
- **Insight guidance:** Same baseline plus general guidance about reopening, shelter, trip length, personal preferences and useful insight.
- **Advice first:** Shorter standalone judgment prompt, roughly 350–500 words requested, no output schema in the judgment pass. Changed both instructions and output framing, so this does not isolate the effect of schema removal.

All three variants used identical serialized evidence per case, verified by SHA-256. Reference judgments, case titles and archived model answers were excluded. Hand-curated evidence is still imperfect and includes unverified operational claims. One sample per variant/case is insufficient to establish repeatability or causation.

| Case | Baseline | Added insight guidance | Advice first |
| --- | --- | --- | --- |
| closed-roaring-fork | SKIP | SKIP | SKIP |
| windy-local-i70 | SKIP | SKIP | SKIP |
| active-little-cottonwood | SKIP | SKIP | WATCH |
| huge-cascades-uncertain-access | SKIP | SKIP | WATCH |
| cascades-confirmed-reopening | RECOMMENDED | RECOMMENDED | RECOMMENDED |

These labels are observations, not a pass/fail score. Only I-70 has a user-confirmed target verdict (Watch until wind/access improves); other reference judgments remain provisional.

## Human assessment of the actual advice

### Cost and preference interpretation

Baseline and added-guidance outputs repeatedly converted $800 into a hard threshold despite explicit contrary input. The uncertain Cascades candidate said an $800 trip was virtually impossible without actual quotes. Both versions also over-weighted closed steep terrain as if the subscriber required it. General instructions to be insightful did not fix these problems.

Advice-first outputs were less dismissive of practical short trips. However, they still sometimes describe an old estimate as a quote and focus on hitting a target price. More factual restraint is needed; a tier improvement does not establish correct cost reasoning.

### Temporal closures and reopening value

All variants distinguished the confirmed reopening enough to recommend the hypothetical White Pass trip. That demonstrates response to a changed input, not sophisticated insight by itself: the fixture deliberately supplies the crucial access and trip feasibility facts.

The advice-first reopening answer still says untracked powder is virtually guaranteed and treats closure-period snowfall as preserved depth. Neither follows from the evidence. Wind, settlement, patrol activity and terrain releases remain relevant. The first candidate dismisses Saturday on crowd and budget grounds without properly weighing Friday night's additional snow. This is precisely the timing/value comparison the user wants.

### Wind and snow-surface inference

I-70 remained Skip under every variant, despite the user-confirmed Watch reference. The advice-first answer asserts forecast warmth guarantees a hard crust across the mountains, followed by categorical scouring claims. Forecast warmth/wind supports a concern, not exact slope conditions everywhere. It offers warmer spring skiing as an alternative but does not adequately investigate evidence for a viable sheltered powder option or explain what local evidence is missing. No credible safe-zone claim can be made from the supplied regional tables alone.

The closing-terrain case also produces unnecessary confident snow-quality conclusions after the operational exclusion. Correct Skip is not a license for unsupported embellishment.

### Reachable plans and remaining snowfall

The shorter Little Cottonwood answer proposes reachable Thursday/Friday sessions and recognizes a forecast-freshness conflict, improving on outright rejection. Its later-day alternative should remain conditional on operational checks, not assume patrol will have cleared terrain by then.

The shorter uncertain-Cascades answer proposes Friday/Saturday skiing from Denver, then says to wait until Friday morning to book. That requires explicit arrival-time reconciliation; the answer does not establish that the proposed Friday session is reachable. It also calls Crystal the clear primary target without fully exploring whether White Pass's reopening could offer a better opportunity.

The hypothetical reopening answer separates Friday daytime and Friday night snow more usefully, but still overstates total preserved snow and does not thoroughly compare spending another night with the additional snow available Saturday. Named operating-update times and precise crowd expectations are not established by the fixture.

## Decision and next iteration

Keep advice-first as an experimental direction, not a production replacement yet. The first addition of more general instructions was insufficient. The shorter approach made two uncertain travel opportunities more actionable, but correctness and useful insight remain inconsistent.

The next useful comparison should examine a compact decision-focused brief with: the best reachable ski session, one supported advantage, the decisive unknown, and how resolving it changes the plan. These are communication goals, not mandatory answers or a numerical storm formula. Investigate whether weather context can more clearly expose forecast timing and distinguish measured conditions from derived labels. Give the model access to the missing terrain/direction/operations evidence when testing genuine wind-shelter research; an offline fixture cannot measure its ability to retrieve that evidence.

Do not tune I-70 to Watch simply by teaching that case's expected answer. Demand the stated preference be respected, hypotheses remain conditional, and alternatives be weighed. Retain original and failed outputs. Add unseen historical cases after this development set improves.

## Artifacts and reproducibility

Full prompts, input/prompt hashes, raw assessments, model and estimated cost are saved in each run JSON. `insight_experiment_temp_test.go.txt` is the preserved experimental harness: copy it to `hunts/powder/insight_experiment_temp_test.go` and run the named test with Go, using the ignored root `.env`. Round two uses `POWDER_INSIGHT_ROUND=2`. Change output directory names for another run; the harness refuses to overwrite results. It is a temporary test harness, not an installed automated suite, and incurs API costs when explicitly run.

The API runs passed transport/nonempty-output checks only; that is not a quality pass. The two earlier full assessment/extraction replays remain a different experiment because they included old research conclusions and different profile information. These fifteen new runs do not validate JSON extraction or live search quality.

Total estimated cost of these fifteen assessments: **$0.4693**.
