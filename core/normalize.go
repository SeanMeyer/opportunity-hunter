package core

import (
	"regexp"
	"strings"
)

// venueSeatingPrefixes are patterns Ticketmaster prepends for premium seating.
// e.g., "Paramount Theatre Club Seating - Nikki Glaser" → "Nikki Glaser"
var venueSeatingPrefixes = []string{
	"Paramount Theatre Club Seating",
	"Club Seating",
}

var (
	separatorRe = regexp.MustCompile(`\s*[-:–—]\s*`)
	spacesRe    = regexp.MustCompile(`\s+`)
)

// NormalizeTitle standardizes event titles for deduplication.
// It strips venue seating prefixes, normalizes case and punctuation,
// and removes tour name suffixes to group variants of the same event.
func NormalizeTitle(title string) string {
	t := title

	// Strip venue seating prefixes.
	lower := strings.ToLower(t)
	for _, prefix := range venueSeatingPrefixes {
		lp := strings.ToLower(prefix)
		if strings.HasPrefix(lower, lp) {
			rest := t[len(prefix):]
			// Strip separator after prefix: " - ", ": ", " -", etc.
			rest = strings.TrimLeft(rest, " -:–—")
			rest = strings.TrimSpace(rest)
			if rest != "" {
				t = rest
				lower = strings.ToLower(t)
			}
		}
	}

	// Normalize to lowercase.
	t = strings.ToLower(t)

	// Normalize & → and.
	t = strings.ReplaceAll(t, "&", "and")

	// Collapse whitespace.
	t = spacesRe.ReplaceAllString(t, " ")

	t = strings.TrimSpace(t)

	return t
}

// tourSuffixRe matches common tour name patterns after a separator.
// e.g., "the stunning tour", "the relatable tour", "live 2026", "brand new tour!"
var tourSuffixRe = regexp.MustCompile(`(?i)\btour\b|(?i)\blive\b`)

// NormalizeTitleForDedup returns a dedup-friendly version that also strips
// tour name suffixes so "Nikki Glaser: The Stunning Tour" and "Nikki Glaser"
// dedup together.
func NormalizeTitleForDedup(title string) string {
	t := NormalizeTitle(title)

	// Strip suffix after colon: "nikki glaser: the stunning tour" → "nikki glaser"
	if idx := strings.Index(t, ":"); idx > 0 {
		candidate := strings.TrimSpace(t[:idx])
		if len(candidate) >= 3 {
			t = candidate
		}
	}

	// Strip suffix after " - " if it looks like a tour name.
	// "trey kennedy - the relatable tour" → "trey kennedy"
	// But NOT "small town murder - live" (keep compound names without tour words).
	if idx := strings.Index(t, " - "); idx > 0 {
		suffix := t[idx+3:]
		if tourSuffixRe.MatchString(suffix) {
			candidate := strings.TrimSpace(t[:idx])
			if len(candidate) >= 3 {
				t = candidate
			}
		}
	}

	return t
}
