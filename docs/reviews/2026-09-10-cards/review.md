# Card visual pass — September 10, 2026

The previous dashboard was technically readable but visually flat: prose preceded
important event facts, assessment dates competed with event dates, and multiple
expansion controls and tall footers made comparison slow.

A cold screenshot-only reviewer independently described it as an internal monitoring
dashboard. The selected refinement preserves the dark palette and consistent cards,
while giving titles/dates stronger hierarchy, a short two-line teaser, a single
explanation/details disclosure, and labeled editable feedback. Scan controls now sit
behind one disclosure; errors remain visible outside it. Mobile header space is tighter.
Assessment history and old-assessment cautions remain available in details.

The cold follow-up found the revision meaningfully clearer and requested a tighter
mobile introduction, which was applied. Adding imagery or an ornamental hero was
not selected: this pass prioritizes scanning real choices. It remains a restrained
utility, rather than claiming a distinctive editorial redesign.

Travel diagnosis: comedy and performing arts requested only walking routes; the
pipeline saved only walking results, and prompts ignored driving estimates.
Both now use walking for trips up to 30 minutes and actual driving estimates for
longer trips. Unknown driving time is explicitly unavailable. Driving mileage is
stored independently. Existing cached walking/driving results are loaded before
lookup. Prompts use the same mode selection as cards.

Validation: regression tests reproduced absent driving and impractical walking
before fixes; route boundary, distinct mileage, failed lookup, cached HTTP avoidance,
pipeline persistence and prompt checks pass. Full Go tests and go vet passed.
A read-only technical reviewer found the prompt omission; after correction it found
no remaining material issue. A product-informed Gemini baseline review also inspected
both screenshots; claims of missing known prices were not accepted because those
listings did not supply prices.

Real Routes requests succeeded for 17 relevant venues without AI or notifications.
The copied-database preview shows Red Rocks as a 21-minute drive / 15.8 miles.
Browser checks covered desktop/mobile, comedy and performing cards, four movie
cards, powder empty state, status, and expanded preferences. Feedback on copied data
saved a note/downvote, canceled a draft change without losing it, then changed to
an upvote. No production feedback was touched. Preview notification configuration
is disabled and is not evidence about live webhook settings.

Artifacts: before-desktop.png, after-desktop.png, after-mobile.png. Screenshots show
real copied content; the selected feedback visible in the mobile image is QA data.
Additional interaction/category captures and raw review output are in the task-owned
local temporary directory oh-visual-pass.

Final product-informed Gemini review inspected revised desktop/mobile screenshots
and the code. Accepted follow-ups: movie route parity, visible running/error status,
focus the feedback editor without automatically opening a touch keyboard, and keep
word-based powder verdicts above the title. Reason duplication had already been fixed
by hiding the teaser while details are open. Old CSS for the removed reason toggle
was removed. Broad stylesheet consolidation and sub-minute route rounding are outside
this pass; the latter is pre-existing and not exercised by the current venue dataset.
