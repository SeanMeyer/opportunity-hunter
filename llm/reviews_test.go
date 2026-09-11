package llm

import (
	"context"
	"github.com/seanmeyer/opportunity-hunter/core"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestReviewEvidenceRejectsUnsupportedSources(t *testing.T) {
	item := func(kind string, index float64) any {
		return map[string]any{"kind": kind, "source_index": index, "title": "Publisher — review", "subject": "Original production (2024)", "summary": "Praised the staging; criticized the pacing.", "url": "https://invented.example/"}
	}
	sources := []string{"https://www.youtube.com/watch?v=abcdefghijk", "https://critic.example/review", "javascript:alert(1)", "https://youtube.com.evil.example/watch?v=abcdefghijk"}
	entry := map[string]any{"review_evidence": []any{item("clip", 0), item("clip", 1.5), item("review", 99), item("review", 3), item("clip", 4), item("clip", 2), item("clip", 1), item("clip", 1), item("review", 2)}}
	got := ParseReviewEvidence(entry, sources)
	if len(got) != 2 || got[0].URL != sources[0] || got[1].URL != sources[1] {
		t.Fatalf("unexpected evidence: %+v", got)
	}
	if len(ParseReviewEvidence(entry, nil)) != 0 {
		t.Fatal("ungrounded evidence was retained")
	}
	roundtrip := core.ReadReviewEvidence(core.WithReviewEvidence(core.Attributes(`{"genre":"play"}`), got))
	if len(roundtrip) != 2 || roundtrip[1].Subject != "Original production (2024)" {
		t.Fatalf("lost production scope: %+v", roundtrip)
	}
}

type reviewTransport func(*http.Request) (*http.Response, error)

func (f reviewTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestReviewRedirectsNeverFetchDestinations(t *testing.T) {
	good := "https://vertexaisearch.cloud.google.com/grounding-api-redirect/token"
	sources := []string{good, good, "https://attacker.example/grounding-api-redirect/token", "https://vertexaisearch.cloud.google.com/other", "https://user@vertexaisearch.cloud.google.com/grounding-api-redirect/token"}
	calls := 0
	got := resolveReviewSourcesUsing(context.Background(), sources, reviewTransport(func(r *http.Request) (*http.Response, error) {
		calls++
		if r.URL.String() != good {
			t.Fatalf("unexpected request: %s", r.URL)
		}
		return &http.Response{StatusCode: 302, Header: http.Header{"Location": []string{"https://www.youtube.com/watch?v=abcdefghijk"}}, Body: io.NopCloser(strings.NewReader("")), Request: r}, nil
	}))
	if calls != 1 || got[1] != got[0] || got[0] != "https://www.youtube.com/watch?v=abcdefghijk" || sources[0] != good {
		t.Fatalf("calls %d, sources %+v", calls, got)
	}
	got = resolveReviewSourcesUsing(context.Background(), []string{good}, reviewTransport(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 503, Header: http.Header{}, Body: io.NopCloser(strings.NewReader("")), Request: r}, nil
	}))
	if got[0] != good {
		t.Fatal("failed resolution lost original citation")
	}
}
