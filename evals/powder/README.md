# Powder evaluation set

Five development cases: four historical situations and one explicitly hypothetical follow-up for refining advice together. This is a draft calibration set, not a claim that four correct labels establish model quality.

Read `cases/` for evidence and reference notes. `archive/` preserves original outputs for comparison, **not model input**. The original research contains conclusions and unsupported claims; the case inputs instead identify the operational claims explicitly as unverified. Weather forecasts remain available in their saved form.

The common profile is Denver, Ikon, expert, flexible weekdays, and 15 PTO days. It restores this preference from the older Powder Hunter database:

> I want fresh, untouched powder, wherever that exists. Trees, bowls, steeps, flatter terrain, whatever. I have a bit of a preference for steeper terrain when the powder gets truly deep, and less steep/cruisier terrain on more mild amount of powder.

The newer database lacks that preference. Restoring it here is an explicit benchmark assumption, not an edit to either live database. An expert skier is not necessarily looking exclusively for expert terrain. Remote-work flexibility also does not mean skiing and working simultaneously.

## 1. Roaring Fork: snow after closing

Source: Opportunity Hunter evaluation 262, April 18, 2026.

**Evidence:** About 14.7 inches forecast over April 23–29. Saved research reports all five regional resorts closed by April 19. Those closure reports are archived claims, not independently verified source snapshots.

**My provisional judgment: SKIP under the reported season-ending schedule.** First distinguish a seasonal shutdown from a temporary closure and investigate any credible reopening overlapping a reachable ski day. Closed today does not mean closed for the whole opportunity. Expert resort skiing does not establish a backcountry alternative.

**What I want the answer to say:** The described trip does not work under the reported seasonal schedules. Explain whether any supported bonus reopening or practical nearby alternative changes that assessment. Do not invent a reopening to make the trip work, but do not discard evidence of one either.

Old label: ON_THE_RADAR, despite saying not to go. First replay: SKIP. The first replay included the old assessment, so it is not directly comparable to a future run using these separated inputs.

## 2. I-70: small refresh, consequential wind

Source: Opportunity Hunter evaluation 221, April 14, 2026.

**Evidence:** At A-Basin, Friday has 0.9 inches daytime and 2.6 overnight; Saturday has 0.2 daytime and 1.1 overnight. Saturday's wind column is 52 mph. Saved research reports some resorts closed and limited terrain at those still open, including closed Pallavicini and Mary Jane areas.

**Your confirmed judgment: WATCH until wind/access improves.** This is not a blanket rejection of lighter snowfall or cruisier skiing.

**What I want the answer to say:** A short local day could become worthwhile if usable terrain and wind conditions line up. Explain the snow available for the proposed session: Friday night's refresh is relevant to Saturday morning; Saturday night's snow is not. Consider wind and access at the actual proposed resort, without declaring a certain lift closure from a forecast alone. State what update would make the trip worth recommending.

Old label: WORTH_A_LOOK. First replay: WATCH, but it underplayed wind, blurred snowfall periods, and assumed limited steep terrain was too disqualifying. Matching the verdict does not make that reasoning satisfactory.

## 3. Little Cottonwood: potentially worthwhile trip, stale inputs

Source: Opportunity Hunter evaluation 100, April 1, 2026, afternoon in Denver.

**Evidence:** The older forecast table shows small totals. Newer archived research reports 6 inches overnight at Snowbird and substantially more arriving April 1–3; Alta's reported projection is about 10, 9, then 1 inch over those days. It reports both resorts open with partial terrain, cooling weather, and possible canyon-control delays. The detection window and forecast freshness conflict with the newer research. No booked travel, actual quote, or confirmed reservation is supplied.

**My provisional judgment: RECOMMENDED, with a reachable Thursday/Friday plan and explicit conditions.** WATCH is defensible if the conflicting evidence or access cannot be resolved. This is our promising-trip case, not a requirement that the model be enthusiastic regardless of evidence.

**What I want the answer to say:** This could justify a trip. Identify what snow remains available after arrival from Denver; do not sell Wednesday morning skiing to someone still in Denver Wednesday afternoon. Explain which reports are newer, identify the unresolved conflict, and check canyon access and pass/parking details. Unknown airfare does not automatically mean Skip, but it cannot support an unconditional purchase recommendation.

Old label: WORTH_A_LOOK. No new replay yet.

## 4. South Cascades: enormous upside, unjustified guarantees

Source: older Powder Hunter evaluation 207, March 12, 2026, Thursday afternoon in Denver.

**Evidence:** Saved models put Crystal Mountain's Friday snow around 13–20 inches and White Pass around 19–23 inches, with important daytime/overnight splits. Thursday wind forecasts are strong; several Friday projections ease substantially. Archived research reports White Pass closed Thursday with an anticipated Friday reopening, and contradictory Crystal terrain status. Travel, road conditions, and reopening are not confirmed. The old $1,700–2,400 estimate describes three nights while its itinerary departs Saturday.

**My provisional judgment: WATCH with urgent checks and exceptional upside.** A strong recommendation may be justified if access, arrival timing and acceptable cost can be established. Large totals alone do not prove DROP_EVERYTHING, and uncertainty alone should not erase the potential.

**What I want the answer to say:** This might be worth a rushed trip. Compare reachable Friday and Saturday sessions, separating snow already down from snow still arriving. State which opening/access checks matter. Never promise all expert terrain will open, all powder will stay untouched, or lodging is inside closure gates without evidence. Cost assumptions and number of nights must match the proposed trip.

**User calibration:** Roughly $800 for a short trip would be worthwhile for exceptionally deep days with road and resort access confirmed. $1,700 sounds expensive, but the old three-night estimate is not a reason to dismiss cheaper one- or two-night plans. $800 is a value reference, not a hard cap or a verified quote.

Old label: DROP_EVERYTHING, accompanied by unsupported guarantees. No new replay yet.

## 5. Cascades follow-up: access confirmed, reopening Friday

This is an **explicitly hypothetical variation** of case 4, not another historical event. Keep the same forecast; supply a planned Friday 09:00 reopening, confirmed road approach, feasible Thursday arrival, and an approximately $800 one-night trip including required lift access. Patrol releases and later weather remain conditional. These assumptions are evaluation inputs, not claims about actual historical announcements, fares or road conditions.

**My provisional judgment: RECOMMENDED; potentially DROP_EVERYTHING if the whole experience justifies it.** The model should change its advice when the decisive objections change. Evaluate whether Friday's first reopening turns offer an advantage, or whether additional Friday-night snow makes Saturday worth an extra night despite potential weekend crowds. It should not promise empty slopes, instantaneous full-terrain access, or all forecast snow at Friday first chair.

This pair tests responsiveness to new evidence. Exact tier remains less important than a useful supported plan and honest uncertainty.

## What counts as useful extra insight

We want more than a weather recap with warnings. A strong answer might identify an attainable reopening window, an earlier terrain release, a better arrival day, a credible sheltered option, or a cheaper short itinerary that changes the value of the opportunity. It should connect evidence to a concrete recommendation and name the decisive missing fact when necessary.

These are examples, not mandatory fields. A clear Skip can be the most useful answer. Invented secret stashes, guaranteed reopening powder, unsupported crowd predictions and fake price precision receive no credit for sounding clever. In particular, wind shelter needs terrain, direction and local operating evidence; it is not a synonym for avalanche safety. Missing evidence should produce a targeted check or a tentative alternative, not a claim of a safe zone.

Case 2's Watch remains grounded in wind/access, while preserving the possibility of a supported sheltered lift, a different resort, or another session. Case 4 should explore a short practical trip and the reopening opportunity before deciding whether uncertainty is decisive.

## Refinement process

1. Record our judgments and the reasons behind them. Only case 2 currently has an explicit user-confirmed label; the others remain my proposals.
2. Freeze these inputs. Use the historical as-of date and no present-day search. Do not pass reference notes or archived model outputs to the model.
3. Establish a new baseline using the current prompt and restored preference. This separates the missing-profile problem from later prompt improvements. Preserve the earlier two runs as a different experiment.
4. Refine how the prompt asks for evidence use: reachable ski sessions, snowfall timing, relevant wind/access, preference fit, and cost assumptions. Keep the overall judgment open; do not encode case-specific dates, magic snow thresholds, or desired answers.
5. Save assessment and extracted output separately. Check that extraction preserves both verdict and uncertainty. Compare human reviews of evidence fidelity, preference fit, actionable advice, and uncertainty—not just label matches.
6. Retain unsuccessful outputs. Once this set improves, try unseen historical storms before concluding that quality has improved generally. This set has no user-confirmed exceptional case yet; do not force one case into each tier to make the test look balanced.

`manifest.json` defines the case list, run-record requirements, and a simple human review rubric. A first judgment-only experiment compares an adapted current prompt against the same prompt plus `insight-guidance-candidate.txt`. Both use identical evidence and restored preferences, no search, and no reference judgments or archived answers. This is an experiment, not a production prompt change or full-pipeline evaluation. Results are stored under `runs/20260909-insight-v1/`; the temporary harness is retained as `runs/insight_experiment_temp_test.go.txt` for reproducibility. No server data or notifications were changed.

## First experiment results

See [comparison and failures](runs/comparison.md). Fifteen assessments compare three prompt variants across these five cases. A shorter advice-first prompt improved how two uncertain trips were handled, but material reasoning errors remain. No candidate has been promoted to production.

## Focused prompt/model comparison

[Ten-run Cascades experiment](runs/20260909-cascades-matrix/review.md): Flash remains the preferred candidate for further testing. Short prompts with a few alternative-plan examples produced more useful conditional advice than a bare request; input compression saved cost but did not reliably improve quality. Both Flash and Pro still made material errors. The reference answer was withheld, and production behavior was not changed.

## Transfer check and knowledge/search distinction

[Four-run transfer results](runs/20260909-transfer/review.md): the unchanged short-with-hints prompt produced useful conditional I-70 advice and reopening recommendations. Residual snow-timing and certainty errors remain. Exact tier agreement is secondary to useful analysis. These frozen tests restrict outside knowledge and disable search; they do not establish what the production search-enabled advisor can discover about terrain or shelter.

## Session and extraction validation

[Final bounded pass](runs/20260909-session-validation/review.md): 18 search-enabled assessments on three explicitly synthetic new scenarios, plus six extraction-only checks. Flash and Pro still overclaimed preserved powder and operations despite shorter/calibrated prompts. No research candidate was promoted. A small extraction change now supplies retrieved source URLs and reinforces provenance, budget distinctions and conditional summaries. The report separates structural validity from research quality and records remaining failures.

## Comparative selection of v5

[Matched current-versus-candidate comparison](runs/20260909-tradeoff-comparison/review.md): after adopting the user's criterion of best overall useful advice rather than perfection, 18 assessments compared the actual current prompt with tradeoff guidance, an example and a buffer-day refinement. The selected v5 prompt improves decisions and contingencies, with documented remaining errors. It is now wired into the local app; schema extraction follows independent prose research. Raw comparisons and production-path synthetic checks are retained.

## Preference sensitivity

[Controlled v5 check](runs/20260909-preferences/review.md): changing only preferences across eight assessments shifts the recommendation between open glades, a possible untouched-powder reopening, and freshly groomed pistes. Structured extraction retains those choices. This supports meaningful preference influence without a fixed numerical weight; nuanced/conflicting preferences and fallback consistency remain limitations.
