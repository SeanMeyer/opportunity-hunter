# Powder v5.0.1 focused live rerun

Same production runner and exact synthetic fixtures; new output directory. Three two-step evaluator calls, sequential, 90-second context deadlines. No retries or grounding sources. All calls succeeded. Confirmed saved research prompts contain v5.0.1 and explicit new evidence-boundary paragraph. Research/structured verdicts unchanged: strong-accessible DROP_EVERYTHING; rain-wind-access SKIP; moderate-negative-only SKIP.

## Improved
- rain-wind-access structured summary now says "forecast 65 mph winds risking major lift holds", and closure_risk/resort_insights preserve forecast/unconfirmed qualifiers. Prior risk-to-observation extraction finding did not recur.
- moderate-negative-only alternate is conditional: local resort "if they catch any piece of this pulse". crowd_estimate remains Unknown.

## Still unsupported
- strong-accessible research invents "2:00 PM today" deadline and heavy snow onset before evening from daily totals; says Winter Park fallback "won't duplicate" Steamboat depth/fluff but still delivers "a great powder day" with no alternative forecast. Summary assumes road deterioration. This violates explicit added evidence instructions.
- strong-accessible extracted information_edge asserts "advertised totals will translate directly to retained soft powder in tree glades", stronger than plausible research payoff and inconsistent with original guidance that accumulated snowfall is not measured retained powder.
- strong-accessible research says existing road frequently closes and raises main risk to night stuck in car although scenario supplied open approach and no material access warnings. Durable winter-driving concern is fair; storm-specific severity is unsupported.
- rain-wind-access research still asserts local alternatives eliminate Steamboat freezing-rain inversion and provide reliable drive home, despite no alternative weather/road data. Research wind holds still "almost certainly" shuts particular lifts. The negative verdict remains appropriate.
- moderate-negative-only research confidently predicts hard bottoming and firm base with no underlying-base data; tells user they "will spend the afternoon" regretting it. Preference-aligned SKIP is supported without these embellishments.
- moderate-negative-only research now conditions alternate snowfall, but extraction strategy/day_by_day loses that condition when recommending a local day trip (secondary, lower-impact).
- moderate-negative-only local Friday day trip said to preserve PTO although a Friday ski day may still require PTO; schedule assumption not established.

## Interpretation
One sample per case: cannot quantify reliability or infer overall quality improved from three outputs. The extraction qualifier adjustment helped one concrete defect; broader prompt-only evidence guidance was insufficient to eliminate comparative weather/access inventions, precise deadlines, or unsupported base/retention claims. Do not report all recommendation-quality issues fixed.

Files per case: input.json, prompt.txt, http-01.json, http-02.json, result.json. Request/response bodies exclude credentials. Per-result cost is authoritative; cumulative fields reset per invocation.
Aggregate estimated USD: 0.03373575
