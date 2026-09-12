# Saved downvote placement

Saved downvotes are partitioned into a collapsed Not for me section below the main recommendations. Latest feedback determines placement. Existing score/venue filters apply before grouping; each group retains the requested sort order. Existing card markup is shared by both sections so details, links, notes, feedback controls, and filter-preserving forms remain identical.

Saving a downvote redirects to the collapsed section and shows a placement notice. Draft changes and Cancel do not move cards. Saving Good pick restores the card to the main list and redirects to its anchor. No AI scores, evaluations, notifications, or expiry behavior changed.

Verification:
- Full go test ./... and go vet ./... passed.
- Integration test exercises POST downvote and GET placement, one copy of each card, collapsed default, all-downvoted filtered results, restoration with a newer upvote, preserved sort, and unchanged AI score.
- Real browser interactions used an enriched production database copy, not live feedback: cancel left Ralph Barbosa in place; save down moved him from the five matching Downtown 8+ cards into the collapsed section; save up restored five main cards. Venue, score filter, and date sort survived.
- Desktop collapsed section and mobile restore screenshots inspected. The 390px viewport had no horizontal overflow. The stable draft upvote state was captured after checking aria-pressed; saved rating and final placement were separately verified.
- Focused read-only technical review found no actionable regressions. Independent Gemini review also confirmed grouping and state behavior. Its notice-copy observation was addressed by using Saved rather than Moved for repeated note edits. The top notice can be outside the viewport after redirecting to the bottom section; the visible section heading provides the destination cue, and its expanded hint explains restoration. Legacy hunt feedback-option names were already separate from the binary card controls and were not changed.
- This is a small extension of the previously reviewed card UI; visual coverage is direct render/interaction inspection, not a new design comparison.
