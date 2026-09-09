# Powder live production quality validation

Production path: PowderHunt.Init -> Evaluator().Evaluate -> buildPrompt using ScanSnapshot -> llm.Client.TwoStepAdvice. No replacement research prompt or schema. Runner imports repository Go packages and wraps default HTTP transport to record only bodies, never credential headers/URL. No repository writes or notifications/database operations.

All six evaluations succeeded against gemini-3.8-flash; each had two HTTP requests, no retries and no grounding sources. Each had a 90-second context deadline and ran sequentially. Aggregate production-estimated cost $0.06158025 (individual result files authoritative). Completed latencies 6.5-10.6 seconds. Second runner invocation reset cumulative field, so sum result Evaluation.CostUSD across six files rather than using last cumulative field.

Synthetic January 2027 scenarios, explicitly labeled simulations. All used rich forecasts with assumed 09:00-16:00 ski-session snow, resort metadata carrying hypothetical operations and dated reference-time method, and profile/preferences. No claims that current September resorts offer skiing.

## Outcomes
- closed: SKIP
- strong-accessible: DROP_EVERYTHING
- rain-wind-access: SKIP
- moderate-positive: RECOMMENDED
- moderate-negative-constraint: SKIP
- moderate-negative-only: SKIP

Feedback-only pair uses identical snapshot and travel preferences; only feedback rating/note changes. It responds correctly to preferences. Separate hard constraint case correctly refuses unreachable ski session.

## Ranked findings

1. P2: Unsupported alternative operations and forecast advantages.
   closed/result.json research: "you will actually get on snow" at Winter Park/Mary Jane or Copper, and "open Front Range terrain" although no alternative operations were supplied and no search occurred.
   strong-accessible/result.json research: "neither is modeled to take the brunt of this 17-inch bullseye" for Winter Park/Copper; no alternative forecast exists.
   moderate-positive/result.json research: Winter Park "will likely lack Steamboat's cold, ultra-light crystal density and low-angle tree shelter"; no alternate weather exists.
   rain-wind-access/result.json research: local alternatives have more reliable road access and avoid localized freezing rain; no alternate roads/weather exists.
   These assertions affect comparative choice, despite correct main verdicts. Recommended fix direction: keep unsupported alternatives explicitly conditional and avoid inventing comparative weather/access.

2. P2: Extraction turns forecast wind risk into actual holds in summary.
   rain-wind-access/input.json explicitly says 65 mph gusts are forecast, not confirmed holds, and operations are unconfirmed.
   Research says widespread wind holds "nearly certain" (already overconfident inference).
   Structured summary: "Skip this trip due to road closures, impending freezing rain and refreeze, and 65 mph wind holds on a zero-flexibility schedule."
   Structured information_edge also says "65 mph wind holds". Recommendation and closure_risk preserve threat/risk, so fidelity is inconsistent across fields.
   Recommended fix direction: strengthen general extraction fidelity of forecast/risk versus observed/confirmed facts, including all rendered summary fields.

3. P2/P3: Feedback changes invented base/track-out claims, not just preference weighting.
   Identical moderate-positive research promises "untouched dendrites over a soft base" and "exceptional float".
   moderate-negative-only research says skier will "consistently punch straight through to the firm, tracked underlying base", and off-piste will be skied off/bumpy by midday. Neither input supplies base condition or crowd observations. The negative summary states 8 inches "will not float an expert skier".
   Correct positive RECOMMENDED / negative SKIP can be defended from tastes alone. Invented snow mechanics are unnecessary and make personalized reasoning less trustworthy.

4. P3: Unsupplied Saturday weather and overly precise timing.
   moderate-positive research calls Saturday "without fresh accumulation"; only Friday DailyForecast is present.
   strong-accessible research says snowfall tapers Friday afternoon although day aggregate and session totals don't locate hourly taper.
   Arbitrary 15:00/17:00 deadlines are useful suggested decisions but implied sunset/heavy-snow onset and road deterioration are unsupported.

## Passed checks
- Closed resort and impossible travel availability never recommended.
- Rain/refreeze, wind and closed road dominate large snowfall appropriately.
- Main tiers preserved through extraction.
- Positive vs negative feedback changes judgment in expected direction.
- Opening snow vs during-ski snow remains distinguished in most detailed fields.
- Unverified lodging stays labeled estimated/unverified; no fabricated verified quotes.
- No fake research source URLs.
- Unknown fields populated usefully.

## Limitations
Only six single runs; no repeatability estimate. One resort, simulated data rather than live forecast collection. No expired opportunity state case: closure and hard timing exercised instead. Rain fixture intentionally combines computed snowfall metrics with separate explicit rain narrative; no independent physical forecast reconstruction. Some generated crystal/calm-wind statements follow supplied computed ride-quality text, including defaults in synthetic HalfDay fields; do not interpret those as proven production weather bugs. Single synthetic model is formatted as high-confidence consensus by existing ComputeConsensus, so confidence claims need separate production-data scrutiny. No future resort operations can be verified from present-day pages. No external fact checks needed for strongest findings, which are unsupported by their recorded inputs.

Each case directory contains input.json, prompt.txt, http-01.json (exact research request/response and usage), http-02.json (exact extraction request/response and schema/usage), and result.json (production evaluation and picks).
