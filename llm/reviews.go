package llm

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/seanmeyer/opportunity-hunter/core"
	genai "google.golang.org/genai"
)

const ReviewEvidencePrompt = `
## Help the user judge for themselves
Use Google Search now to retrieve review/award pages and comedy performance videos for the recommended picks.
Research up to three useful pieces of review evidence, using actual retrieved pages, not remembered acclaim.
In the research prose identify the page and URL; source_index is assigned only by the later extraction pass
from its retrieved-source list. Do not invent indices or output a pretend source catalog during research.
Comedy: prioritize one short stand-up performance on YouTube, ideally from the comedian's official channel,
a broadcaster or the special's distributor. An actual bit, not an interview, trailer or compilation of other
comedians. Describe the style so the user can sample it; do not claim it is representative or popular without
evidence. A review of a past special can supplement it. Never invent a video URL or video ID.
Movies: concise critical reception, including reservations, from named reviews. Identify the film and year.
Performing arts: reviews of the relevant production or sourced awards. Explicitly identify original Broadway,
touring production, local cast, or the artist's previous work; do not transfer their reviews to this engagement.
For each item record the page title/publisher, the precise work reviewed (with year when available), and one
short paraphrase (at most 40 words). No copied review quotations. If including a rating, name its source,
scale, and whether critics or audiences; don't invent or combine ratings. Awards must name category and year.
Don't mistake promotional praise for independent criticism. Omit unavailable evidence; an empty array is valid.
These notes supplement your taste recommendation; acclaim does not override the user's preferences.
`

// WithReviewEvidence adds optional editorial context to an evaluator's pick contract.
func WithReviewEvidence(schema *genai.Schema) *genai.Schema {
	schema.Properties["picks"].Items.Properties["review_evidence"] = &genai.Schema{
		Type:        genai.TypeArray,
		Description: "Up to three sourced clips, reviews or awards, or empty if unsupported. Use only retrieved sources.",
		Items: &genai.Schema{Type: genai.TypeObject, Properties: map[string]*genai.Schema{
			"kind":         {Type: genai.TypeString, Enum: []string{"clip", "review", "award"}},
			"title":        {Type: genai.TypeString, Description: "Concise source/publisher and article or video title"},
			"subject":      {Type: genai.TypeString, Description: "Exact film, special, artist or production this concerns, including year/context"},
			"summary":      {Type: genai.TypeString, Description: "At most 40 words of sourced paraphrase, preserving reservations and caveats"},
			"source_index": {Type: genai.TypeInteger, Description: "1-based index in Retrieved source URLs. Clips require an actual YouTube video URL there. Never use an unrelated source."},
		}, Required: []string{"kind", "title", "subject", "summary", "source_index"}},
	}
	return schema
}

// ParseReviewEvidence only emits URLs supplied by search grounding, never model-authored URLs.
func ParseReviewEvidence(entry map[string]any, sources []string) []core.ReviewEvidence {
	items, _ := entry["review_evidence"].([]any)
	var result []core.ReviewEvidence
	seen := map[string]bool{}
	for _, item := range items {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		n, ok := m["source_index"].(float64)
		if !ok || n < 1 || n > float64(len(sources)) || n != float64(int(n)) {
			continue
		}
		get := func(k string) string { s, _ := m[k].(string); return strings.TrimSpace(s) }
		e := core.ReviewEvidence{Kind: get("kind"), Title: get("title"), Subject: get("subject"), Summary: get("summary"), URL: sources[int(n)-1]}
		if !e.Valid() || seen[e.URL] || len([]rune(e.Summary)) > 600 || len([]rune(e.Title)) > 180 || len([]rune(e.Subject)) > 180 {
			continue
		}
		if e.Kind == "clip" {
			for _, existing := range result {
				if existing.Kind == "clip" {
					e.Kind = ""
					break
				}
			}
			if e.Kind == "" {
				continue
			}
		}
		seen[e.URL] = true
		result = append(result, e)
		if len(result) == 3 {
			break
		}
	}
	return result
}

// Resolve only Google's grounding redirect endpoint, without fetching destinations.
// This exposes the real YouTube URL to extraction; failures retain the citation URL.
func resolveReviewSources(ctx context.Context, sources []string) []string {
	return resolveReviewSourcesUsing(ctx, sources, http.DefaultTransport)
}

func resolveReviewSourcesUsing(ctx context.Context, sources []string, transport http.RoundTripper) []string {
	client := &http.Client{Transport: transport, Timeout: 4 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	resolved := append([]string(nil), sources...)
	ctx, cancel := context.WithTimeout(ctx, 12*time.Second)
	defer cancel()
	pending := map[string][]int{}
	for i, raw := range sources {
		u, err := url.Parse(raw)
		if err == nil && u.Scheme == "https" && u.User == nil && u.Host == "vertexaisearch.cloud.google.com" && strings.HasPrefix(u.Path, "/grounding-api-redirect/") {
			pending[raw] = append(pending[raw], i)
		}
	}
	var wg sync.WaitGroup
	slots := make(chan struct{}, 4)
	for raw, indices := range pending {
		wg.Add(1)
		go func(raw string, indices []int) {
			defer wg.Done()
			select {
			case slots <- struct{}{}:
			case <-ctx.Done():
				return
			}
			defer func() { <-slots }()
			req, err := http.NewRequestWithContext(ctx, "GET", raw, nil)
			if err != nil {
				return
			}
			resp, err := client.Do(req)
			if err != nil {
				return
			}
			_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
			resp.Body.Close()
			if resp.StatusCode < 300 || resp.StatusCode >= 400 {
				return
			}
			dest, err := resp.Location()
			if err == nil && dest.Scheme == "https" && dest.Hostname() != "" && dest.User == nil {
				for _, i := range indices {
					resolved[i] = dest.String()
				}
			}
		}(raw, indices)
	}
	wg.Wait()
	return resolved
}
