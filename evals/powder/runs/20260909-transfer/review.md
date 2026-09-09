# Short-with-hints transfer check

Four Flash 3.8 assessments, medium thinking: two independent samples each on the hypothetical confirmed-reopening case and historical I-70 case. The 140-word short-with-hints prompt was unchanged from the Cascades comparison. Human reference answers and expected tiers were withheld. Identical prompts within each pair were verified by hash; all four completed with STOP and nonempty output. No search, extraction, notifications or production changes.

## Search and knowledge: correction to the earlier framing

The production research pass already enables Google Search (`llm/gemini.go`). These historical experiments deliberately disable it and instruct the model to use supplied evidence only. Consequently they do not measure the full advisor's ability to retrieve terrain information or use general resort knowledge. Failure to propose a wind-sheltered alternative in this setup is not proof that the model cannot find one in production.

General knowledge is an appropriate source of hypotheses: a different resort, aspect, treed sector, or reopening window may be worth investigating. An answer need not cite every ordinary piece of geographical knowledge. The distinction is between proposing a plausible option and confidently asserting storm-specific shelter, safe terrain, an open lift or a reopening schedule. Search and current/local information can help resolve those details.

Code inspection found speed/gust fields but no structured wind-direction field in the weather types or forecast parsing. Forecast discussion prose may mention direction, but this is not reliable structured input. [Open-Meteo supports hourly wind direction](https://open-meteo.com/en/docs). Recommended future input change: retain forecast direction with time and model/location/height context alongside speed and gusts. Do not infer resort-wide shelter from a single 10 m gridpoint or average compass bearings arithmetically across north. Let the model investigate terrain/operations as needed rather than build a large fixed rulebook of resort shelter claims.

A separate research test should allow general geographical knowledge and search, capture sources and retrieval timestamps, then assess whether those sources actually support the suggested alternative. Historical operational truth requires archived as-of evidence; today's search cannot silently substitute for it. This transfer test does not answer research-quality questions.

## Confirmed reopening

Both samples: Recommended. Both identify a Thursday arrival, Friday White Pass reopening, one-night trip around the supplied hypothetical $800 estimate, reduced Friday winds and conditional patrol releases. They distinguish a temporary closure from a season-ending shutdown. This is useful human decision support and the intended direction.

Limitations: the fixture explicitly supplies feasible travel, cost and planned access; the model did not discover them. Both samples overstate how much snow a closure preserves. One uses Friday's weekday timing as a crowd advantage without adequately comparing Saturday's additional Friday-night snowfall. Suggested pivots to Crystal require separate route and operations checks; open access to one resort does not establish access to another. Forecast winds alone do not prove lift viability.

The reference can accept Recommended or a well-supported exceptional verdict. Exact tier is secondary to a realistic plan, time-aware snow interpretation, and appropriately conditional claims.

## I-70

Both samples: Watch with a possible short local outing. Both preserve the option to ski modest accumulations, recognize wind-hold risk, and identify resort operations as a relevant check. One explicitly offers cruisier terrain. This is more useful than the earlier categorical rejection, and aligns directionally with the user's stated judgment.

Limitations: one calls A-Basin's Friday overnight snow 3–4 inches though the supplied table gives 2.6 overnight plus 0.9 daytime. Another cites 4.8 inches across Friday/Saturday while proposing Saturday morning; that total includes Saturday night's 1.1 inches, unavailable to the morning session. Both introduce arbitrary 5–7+ inch upgrade triggers or mandatory steep-terrain conditions. One assumes a drive and ski session fits before remote work without establishing work hours. Wind concern should remain tied to the chosen session and operating terrain, not automatically every nearby resort.

Different grades can be reasonable. An analysis proposing a credible sheltered alternative could support a conditional recommendation even when the original reference says Watch. A matching Watch with faulty snow arithmetic is not a quality pass.

## Assessment and next step

The short prompt with a few hints transferred usefully to these cases in two samples each. This is encouraging development evidence, not a general reliability claim: both cases had already been examined earlier in the development process, and the reopening is hypothetical. Avoid tuning exact verdicts or adding a growing checklist.

Keep the concise advisor prompt and evaluate researched, source-backed alternatives next. The priority is preserving useful judgment while making session timing and factual assumptions clear to the reader. Validate extraction separately before promoting this to production. Retain these imperfect outputs for comparison rather than replacing them with manually cleaned answers.

Total estimated API cost: $0.0567.
