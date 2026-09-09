# Search-enabled terrain probe

Two exploratory Flash 3.8 calls, with the I-70 historical forecast kept fixed. General knowledge and Google Search were allowed for terrain/exposure; present-day operations could not establish historical access. No production prompt was changed.

The optional-search run returned no grounding sources. The follow-up explicitly requested Google Search for relevant terrain and returned three reported queries and five grounding sources. Grounding support metadata associates its general terrain/exposure statements with retrieved pages. This demonstrates retrieval, not independent validation of every claim or proof of storm-specific shelter.

Queries:
- Winter Park Mary Jane wind exposure terrain
- Arapahoe Basin wind exposure terrain aspects
- Copper Mountain terrain wind exposure aspect

Returned source domains: peakrankings.com, chartersports.com, onthesnow.com, thevirtualsherpa.com, powderhounds.com.


The searched answer distinguishes exposed high-alpine terrain from tree-sheltered options and explicitly leaves a specific lee-side recommendation unresolved without wind direction. It still introduces a six-inch threshold and makes categorical surface-quality claims. General search therefore provides useful evidence but does not automatically resolve the reasoning weaknesses. No current forecast direction was inserted into the historical storm.

A separate live Open-Meteo request using the application's three model names returned non-null wind-direction arrays for GFS Seamless, ECMWF IFS and GFS HRRR. New forecasts now parse direction; old snapshots still lack it. The live request establishes provider support, not historical wind direction.

Important evaluation correction: during implementation we found that the current parser's Night bucket combines early-morning and evening hours of the same local calendar date. Earlier experiment commentary describing the entire bucket as the following overnight period was too strong. The new prompt explicitly describes the actual aggregation. Exact first-chair snow needs hourly evidence or revised period modeling; model wording alone cannot repair that missing distinction.

Full prompts, assessments, usage and grounding metadata are retained in the two adjacent probe JSON files. Grounding source pages were not independently audited in this small probe; do not treat these as validated shelter recommendations.
