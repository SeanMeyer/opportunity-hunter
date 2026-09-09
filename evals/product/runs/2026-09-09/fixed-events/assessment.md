# Focused post-fix live evaluation — 2026-09-09

3 evaluator calls, 6 HTTP requests, model gemini-3.8-flash, total estimated $0.05086125. Latencies: comedy 14.8s, movies 8.9s, performing arts 11.5s. All calls succeeded, within 90-second deadlines; sequential. Production prompts unchanged by harness. Source credential sentinels only; Google key read securely from existing .env; no sources, notifications, or repository edits.

## Results

Movies now receives neutral film titles, preventing constraint status leaking through fixture names. Both research and extracted output correctly preserve the actual fixture data: Arrival and Alien ($35, Sep 18 2026 at 7 PM, 10-minute walk) recommended 10/8; Blade Runner ($150) rejected for $50 budget; Moon (95-minute walk) rejected for 20-minute limit; Dune (11 PM) rejected for 6–9 PM window; Solaris (Aug 29) rejected as past; Mystery Film rejected for unknown schedule/price. Urgency checks availability at the actual synthetic venue, with no off-constraint catalog theater suggestions. No claims that missing constraints were met.

Comedy recommends Nate 10 / Taylor 8 while rejecting all hard failures and treating the zero schedule as unknown, not midnight. Urgency explicitly says no scarcity data and to check listing details. Residual issue: sell_out_risk=low still means missing evidence, because schema admits only low/medium/high. Research explicitly says 'In the absence of evidence, sell-out risk is treated as low'. Add unknown enum and corresponding prompt description if accurate uncertainty is desired.

Performing recommends Hadestown 8 / Swan Lake 7. Both research and short structured reasons distinguish the famous work from the unknown company/staging and preserve that uncertainty. Urgency asks to confirm availability and production details; no unsupported limited-run or high-demand claim. All constraint failures rejected and unknown dates remained unknown.

Across all 3 responses, required schema accepted by production validation and mapped IDs/scores valid. A single focused sample per hunt supports these specific observations, not broad deterministic reliability. Feedback was not re-tested in this focused run (earlier 12-call benchmark covered feedback and repeats).

## Artifacts

main.go runnable harness; *-input.json exact evaluator inputs; *-result.json exact prompts, research, structured outputs and mapped picks with timestamps/cost/latency; http-01.json through http-06.json exact request bodies for both passes and response usage; summary.json aggregate totals. Headers/query not saved; secrets scrubbed. No copied credentials in harness or artifacts.
