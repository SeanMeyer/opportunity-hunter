# Venue filter

Venue dropdown combines with the existing rating filter and sort order. Its options come from current, unfiltered cards. URL state survives refresh and form submissions; changing hunt starts that hunt's default view. Selecting All venues removes only venue restriction; Clear filters in the empty state clears both filters.

Uses the existing address-checked Comedy Works Downtown alias, preserving South as a separate venue. Other venue keys retain their normalized address, and duplicate names display addresses to disambiguate source records. No database mutations or changes to recommendation scoring, scans, or notification preferences.

Web integration tests verify combined filtering, downtown alias membership, exclusion of South, redirects, and recovery for a venue with no current cards. Full Go tests and vet passed. A focused read-only reviewer found no actionable defect; an independent Gemini review is running.

Browser interaction verification on an enriched production copy: Downtown selected 14 cards, adding 8+ selected 5, date ordering put Ralph Barbosa before Mark Normand; changing to Red Rocks selected 1 and returning preserved both score and date. Desktop and 390x844 mobile screenshots attached. The mobile venue control occupies a full-width row and the page has no horizontal overflow. This is a small extension of the previously reviewed controls; visual review used direct affected-render inspection. Local preview enabled comedy and powder only, with no scan or notification execution.

Screenshots: desktop.png and mobile.png.
