# Sourced recommendation context

Adds optional context to comedy, movie and performing-arts picks. Comedy prioritizes a real YouTube performance; movies include critical reception; performing arts distinguishes original-production awards from particular touring reviews. These remain separate from the personal match score.

Evidence is researched in the existing grounded Gemini evaluation, saved in pick attributes, and rendered without page-load network requests. Extraction chooses 1-based indices from retrieved sources; code supplies the URLs. Only HTTPS video URLs on the exact YouTube/youtu.be hosts qualify as clips, including Shorts. Unsupported/unknown evidence stays absent. Google grounding redirects are resolved on the exact Google endpoint, with no destination fetching, four concurrent requests, deduplication, a 12-second total deadline and 4KB response drains. URL membership establishes retrieval, not semantic correctness of an AI paraphrase.

One featured item is visible; other notes appear inside the existing details disclosure. Mobile descriptions show two lines until expanded. Full source titles and production scope remain visible. Links open in a separate tab. Old pick attributes remain compatible.

## Actual AI checks

Ran isolated evaluations for Nate Bargatze, The Grand Budapest Hotel (2014), and Hamilton touring. These are synthetic listings with real researched works, not bookable event assertions. Raw research, extraction, final picks and approximate costs are in the adjacent JSON files. No production votes, notifications or recommendation rows were changed.

- Comedy: a 5:11 Tonight Show stand-up clip, a sourced Paste critic rating through Rotten Tomatoes, and a past-special award. Opened the YouTube video in Chrome and confirmed playback; checked the cited Rotten Tomatoes and Chortle pages.
- Movies: three sourced review summaries, including one with reservations. RogerEbert.com rejected the independent fetch with 403, so that page was not independently text-verified.
- Performing: the first research run supplied no usable sources; extraction correctly returned empty evidence. After explicitly requiring Google Search and deferring source indices to extraction, the rerun returned original Broadway Tony Awards and separate 2025/2026 touring reviews. Checked the official Tony winners page.
- Four live runs cost approximately $0.204 total. This is sample coverage, not a guarantee of clip availability or broad summary accuracy.

## Visual review

Independent reviewer received only A/B desktop/mobile screenshots, a one-sentence product description and a request for a reasoned preference (either/neither allowed), visible strengths/regressions, at most three improvements, and no score. A is the same fixture without evidence; B adds evidence. Reviewer preferred B on desktop for concrete character and a useful next action, A on mobile for compact comparison. Its absent-price/venue and duplicate-date concerns were fixture limitations; actual cards already group dates and show known planning details. We retained the evidence panel and shortened mobile descriptions, with full text on expansion. final-mobile.png shows that refinement. Both movie and performing mobile layouts were inspected; expanded movie descriptions were not clamped and the page had no horizontal overflow.

## Technical review and verification

Independent read-only code review found missing Shorts support, now fixed. Gemini review identified the nil-schema guard and redirect connection reuse/duplicate-resolution concerns, now fixed. A focused follow-up found no concrete concurrency fault. Other Gemini claims were rejected or already covered: comedy already saves the actual rendered prompt; int source indices are not emitted by the JSON transport; null arrays intentionally fail the contract (empty arrays are valid); empty attributes already persist as {}. There was no demonstrated null-array production failure.

`go test ./...` and `go vet ./...` pass. Tests cover unsafe and unsupported sources, missing grounding, clip URL forms, deduplication, preserved source positions, no destination fetches, failed redirect fallback, saved attribute compatibility, all three hunts' database-to-template rendering, escaping, and featured-clip ordering. Windows race testing is unavailable with CGO disabled; no race-test success is claimed.

## Rollout

This change affects new evaluations. It does not mass-research existing cards or alter their assessment dates. A separate intentional backfill is needed to populate existing recommendations. No new service credentials are required. Discord configuration is unchanged.

Deployed as v0.3.9 on September 11 at 22:47 UTC; see ../../deployments/2026-09-09-unraid.md. A real startup evaluation populated the Chris Fleming Live card with a clip and review context. Existing cards were not bulk-backfilled.
