# Focused Cascades prompt/model comparison

## Recommendation

Keep Gemini 3.8 Flash as the primary model. The most promising configuration in this small experiment is a short prompt with a few examples of useful alternatives. Use a compact, source-traceable evidence view for efficiency, with the full record retained. Do not promote this directly to production: factual and itinerary mistakes remain in both models.

The experiment supports a narrower conclusion than “less is always better.” A 75-word prompt often produced categorical rejection; a 140-word version adding examples elicited more useful conditional plans from Flash with either evidence presentation. Reducing the input alone did not reliably improve advice. The examples seem useful, but this is one selected development case and most conditions have only one sample.

## Setup

The [reference advice](../../cascades-reference.md) was written before generation and withheld from both models. It allows different verdicts/destinations when justified; it does not grade by Watch alone.

- Models: `gemini-3.8-flash` and `gemini-3.1-pro-preview`, verified available through the authenticated model catalog.
- Prompts: 75-word minimal request; the same request plus approximately 65 words of hints about a reopening, different day/resort or shorter trip, attainable sessions, and flexible preferences/prices.
- Evidence: original archived record (~109,355 characters) versus a focused March 12–15 view (~12,547 characters). The focused view converts units and retains each model's daytime/nighttime snow, gusts, temperature range and SLR. It omits later dates and other weather fields, so this tests selection plus formatting, not lossless compression. Model disagreement is retained.
- Eight initial model/prompt/evidence combinations, plus one independent repeat of hints/compact per model: ten calls total. Medium thinking, no output schema, no search, no extraction. Both models received exactly the same serialized evidence within each presentation, verified by hash.
- Raw prompts, outputs, usage, elapsed times, finish reasons and hashes are retained in adjacent JSON files. Every response completed with STOP and nonempty text. This verifies execution, not advice quality.

## Observations

| Configuration | Flash | Pro |
| --- | --- | --- |
| Minimal / full | Skip; invented cost threshold and adverse snow conclusions | Watch; short weekend plan, but overstates terrain and snow certainty |
| Minimal / compact | Skip; old cost estimate treated as barrier | Skip; cost/terrain preferences treated as requirements |
| Hints / full | Watch; Saturday arrival alternative and operations checks | Skip with conditional Watch; misreads Thursday afternoon as evening and assumes Friday is missed |
| Hints / compact | Watch; actionable Saturday plan but invented price cutoffs | Watch; White Pass reopening idea, but optimistic preserved snow and incomplete cost accounting |
| Hints / compact repeat | Watch; distinguishes Friday day/night snow and notes reopening alternative; itinerary inconsistency | Watch leaning Skip; terrain and cost rigidity persist |

This is a human qualitative review, not a blinded or independent benchmark. Model and prompt labels were visible. Repeated hints/compact outputs are directionally similar but still contain material errors. No broad model ranking follows.

## What actually improved

Flash's hints/full output proposes flying Friday evening for Saturday skiing, checking roads and terrain before committing, and considering delayed terrain openings. Its hints/compact repeat explicitly distinguishes approximately 9–10 inches Friday daytime from 3.5–10 inches Friday night at Crystal. These are more decision-relevant than the minimal prompt's claim that last-minute travel will exceed a supposed budget threshold.

The hints helped Flash search for an alternative plan. They did not simply make it recommend everything: both samples retain access checks and a Watch verdict. The evidence supports examining Saturday, not asserting that it must outperform a confirmed Friday reopening.

Pro sometimes offers a useful White Pass reopening plan, but the comparison gives no clear reason to pay more for this case. Its full/hints answer calls Thursday evening even though the evaluation is at 19:38 UTC, approximately 13:38 in Denver, and uses that mistake to dismiss Friday travel.

## Remaining failures

1. **Preference hardening:** Flash invents $1,000/airfare cutoffs and treats specific steep terrain as necessary. Pro also treats $800 as a limit. Neither matches the user's actual flexibility.
2. **Forecast versus experience:** Descriptions of untouched snow, deep resets and snow support are more confident than the forecast/operations evidence permits. A possible terrain release is an angle to investigate, not a promised outcome.
3. **Itinerary arithmetic:** Flash's repeat calls its plan one night, then proposes Friday arrival and Sunday departure. Optional skiing and number of nights must agree.
4. **Cost accounting:** Pro's compact/hints components total $985 before omitted items, while describing the trip as close to the $800 ideal; required White Pass lift access is not costed. Old estimates are not quotes.
5. **Operational assumptions:** Selecting Crystal because White Pass is closed does not establish that Crystal's access is better. Specific terrain prerequisites and precise booking deadlines are introduced without enough justification.

These are reasons to keep testing the advice, not to encode this case's preferred verdict into the prompt. The strongest next bounded check is whether the short hint prompt transfers to the confirmed-reopening follow-up and the I-70 case, with human attention to assumptions and a concrete session itinerary. Also test whether the extraction pass preserves the useful insight. Neither transfer nor extraction is established by these ten calls.

## Cost and latency

For hints/compact, the two Flash runs averaged approximately **$0.0121 and 10.7 seconds**, versus **$0.0367 and 21.6 seconds** for the two Pro runs. Tiny-sample measurements, not latency guarantees. Total estimated cost of all ten calls: **$0.4488**.

Estimates count input plus candidate/thinking output tokens using standard rates ($0.75/$3.75 per million for Flash's current introductory period; $2/$12 for Pro below 200k input tokens). No search charges apply. Rates were checked against [Google's pricing documentation](https://ai.google.dev/gemini-api/docs/pricing); these are estimates, not billing records.

Reproduce with `run_cascades_experiment.py --repeat 1` using the ignored root `.env`. Existing outputs are preserved. Change the output directory for a fresh complete run. No server state or production prompt/model configuration was changed.
