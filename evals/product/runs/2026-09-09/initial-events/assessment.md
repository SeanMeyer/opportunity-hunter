# Live recommendation benchmark — 2026-09-09

Production revision supplied: main bae5f23. Harness imports real hunt evaluators and unchanged production prompts. No tracked files modified. Fixtures are synthetic; these results do not claim real event availability. Model gemini-3.8-flash (production configuration), 12 evaluator calls / 24 HTTP requests, $0.2556735 estimated production accounting cost, 5.89–75.19 seconds per evaluator call. All completed within 90-second deadlines; sequential concurrency 1. No sources or notifications invoked. Movies/performing Init require source credential presence, so non-secret disabled sentinels were supplied; only Google credential was actually used. Home/distance credentials disabled; built-in movie theater catalog remains part of production evaluator.

## Coverage and observed behavior

Each hunt received seven candidates: favorite, alternative taste, over-budget, over-distance, late, past, missing facts. Four calls per hunt: baseline, changed feedback, identical feedback repeat, and all-bad subset. Feedback candidates/constraints were held fixed. All results parsed, passed production schema validation, and contained valid opportunity IDs and 1–10 scores. No duplicate IDs. Each all-bad subset returned empty picks.

| Hunt | Baseline | Feedback | Feedback repeat |
|---|---|---|---|
| comedy | Nate 10, Taylor 8 | Taylor 9 only | Taylor 10 only |
| movies | Arrival 9, Alien 8 | Alien 9 only | Alien 9 only |
| performing arts | Hadestown 9, Swan Lake 8 | Swan Lake 9 only | Swan Lake 9 only |

Comedy and performing outputs explicitly reject the supplied price ($150 > $50), walking (95 > 20 minutes), start time (11 PM outside 6–9 PM), and past date violations. Movie constraint results are confounded: the production prompt drops actual prices and dates, and the fixture titles contain cues such as Premium, Late, and Past. Correctly rejecting those movie titles therefore does not demonstrate actual constraint comprehension. Feedback rank changes were stable in this small repeat sample; they are evidence, not a broad quality guarantee.

## Confirmed logic defects / minimal fixes

1. Movie prompt drops supplied prices and screening dates/times. Repro: movies-baseline-input.json has PriceMin/PriceMax=35 for Arrival, =150 for Arrival Premium, and distinct StartTime values; movies-baseline-result.json RenderedPrompt contains none of these. The model cannot distinguish a $35 from $150 version of the same title/venue based on price fields, nor time eligibility. It nonetheless claims Premium exceeds budget and Late/Past violate time constraints, inferred from names. Fix hunts/movies/prompt.go to include known PriceMin/PriceMax, StartTime/ShowDates for screenings, explicit unknowns, and full date/year/timezone. Add a prompt test using identical title/venue with changed price/time facts, ensuring facts actually reach the prompt.

2. Comedy and performing render zero time as 'Mon Jan 1, 12:00 AM'. Repro: Mystery fixture StartTime is zero; all comedy/performing outputs treat its time as a midnight constraint violation. Unknown became fabricated specific scheduling evidence. Fix those prompt builders to use StartTime.IsZero and render Unknown (and preserve years/timezone for known dates). Test zero date explicitly. Their dates also omit year; this benchmark supplied current date in preferences, so no claim made that production correctly handles year boundaries.

## Quality issues and proposed measured changes

- Comedy baseline asserts 'will sell out very fast'; performing baseline claims high demand and limited affordable seating; performing feedback says 'buy tickets now — limited run'. Fixtures contain no demand/inventory/run-length evidence. These claims appear in research itself, not introduced solely by extraction. Comedy's mandatory low/medium/high enum cannot express unknown demand. Consider permitting unknown and requiring evidence or explicitly labeled estimate for urgency, with a no-evidence default to checking availability rather than inventing scarcity. Retest before claiming resolved.
- Performing recommendations use famous-work prestige despite company/cast explicitly unknown. General facts about Hadestown awards or Swan Lake are not evidence of this production's caliber. The favorite-work ranking is reasonable, but uncertainty about the production should survive into the short recommendation.
- Movies baseline and feedback variants suggest Alamo Sloans Lake at 3.0 mi / $8 Terror Tuesday despite the user's max-20-minute walk/no-driving constraint. The fixed catalog and urgency instruction encourage off-fixture alternatives. The text sometimes says 'future outings', so this is a reasoning/action inconsistency, not an invented pick ID. Prefer actual screening venue in urgency; allow alternative theaters only if feasible and label availability/pricing unverified. Built-in catalog prices were not independently verified by this benchmark.

## Artifacts

main.go: runnable scratch harness, no embedded secrets; invoke from repo module with scratch dir and .env path arguments. *-input.json: exact evaluator inputs. *-result.json: model, time, latency, cost, exact rendered research prompt, research prose, structured output, mapped picks. http-01.json through http-24.json: exact request bodies for both passes and response bodies including usage/grounding metadata; headers/query omitted, secret strings scrubbed. checks.json: ID/score checks and compact result table. summary.json: aggregate count/model/cost. This report is assessment.md.
