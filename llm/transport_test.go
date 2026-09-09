package llm

import (
	"context"
	"encoding/json"
	"fmt"
	genai "google.golang.org/genai"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestTwoStepHTTPContract(t *testing.T) {
	var requests []map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		requests = append(requests, body)
		answer := "Skip: closed resort, price unknown."
		if len(requests) == 2 {
			answer = `{"tier":"SKIP","recommendation":"Skip: closed resort, price unknown."}`
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"candidates":[{"content":{"role":"model","parts":[{"text":%q}]},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":100,"candidatesTokenCount":20,"thoughtsTokenCount":30}}`, answer)
	}))
	defer server.Close()
	sdk, err := genai.NewClient(context.Background(), &genai.ClientConfig{APIKey: "test", Backend: genai.BackendGeminiAPI, HTTPOptions: genai.HTTPOptions{BaseURL: server.URL}})
	if err != nil {
		t.Fatal(err)
	}
	c := &Client{client: sdk, model: "gemini-3.8-flash"}
	result, err := c.TwoStep(context.Background(), "No flights", &genai.Schema{Type: genai.TypeObject, Properties: map[string]*genai.Schema{"tier": {Type: genai.TypeString}}})
	if err != nil {
		t.Fatal(err)
	}
	if result.Structured["tier"] != "SKIP" || len(requests) != 2 {
		t.Fatalf("unexpected result: %+v", result)
	}
	content := requests[0]["contents"].([]any)[0].(map[string]any)
	sentPrompt := content["parts"].([]any)[0].(map[string]any)["text"].(string)
	if result.RenderedPrompt != sentPrompt {
		t.Fatal("saved prompt differs from actual request")
	}
	config := requests[0]["generationConfig"]
	b, _ := json.Marshal(config)
	if !strings.Contains(string(b), `"thinkingLevel":"MEDIUM"`) {
		t.Fatalf("research reasoning config: %s", b)
	}
	b, _ = json.Marshal(requests[1])
	if !strings.Contains(string(b), "No flights") {
		t.Fatal("extraction lost original context")
	}
	if result.CostUSD < .0005 {
		t.Fatalf("thinking costs omitted: %f", result.CostUSD)
	}
}

func TestResponseCostIncludesThinkingAndSearch(t *testing.T) {
	resp := &genai.GenerateContentResponse{UsageMetadata: &genai.GenerateContentResponseUsageMetadata{PromptTokenCount: 1000, CandidatesTokenCount: 100, ThoughtsTokenCount: 900}, Candidates: []*genai.Candidate{{GroundingMetadata: &genai.GroundingMetadata{WebSearchQueries: []string{"resort status", "lodging"}}}}}
	// Introductory token pricing plus a conservative charge for both search requests.
	want := .00075 + .00375 + .028
	if !time.Now().Before(time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC)) {
		want = .0015 + .0075 + .028
	}
	if got := responseCost("gemini-3.8-flash", resp); math.Abs(got-want) > 1e-9 {
		t.Fatalf("cost=%f want=%f", got, want)
	}
}

func TestPriceTransition(t *testing.T) {
	for _, tc := range []struct {
		date string
		want float64
	}{{"2026-12-31", 4.50}, {"2027-01-01", 9.00}} {
		now, _ := time.Parse("2006-01-02", tc.date)
		if got := estimateCostAt("gemini-3.8-flash", 1000000, 1000000, now); got != tc.want {
			t.Errorf("%s: %f", tc.date, got)
		}
	}
}

func TestModelOverrideAndDefault(t *testing.T) {
	for _, tc := range []struct{ override, want string }{{"", "gemini-3.8-flash"}, {"  gemini-2.5-flash  ", "gemini-2.5-flash"}, {"models/gemini-2.5-flash", "gemini-2.5-flash"}} {
		c, err := NewClient(context.Background(), "test", tc.override)
		if err != nil {
			t.Fatal(err)
		}
		if c.model != tc.want {
			t.Fatalf("model = %q", c.model)
		}
		if tc.override != "" && c.thinking(genai.ThinkingLevelMedium) != nil {
			t.Fatal("legacy override received Gemini 3 thinking levels")
		}
	}
}

func TestTwoStepRejectsMissingDecision(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"candidates":[{"content":{"role":"model","parts":[{"text":"{}"}]},"finishReason":"STOP"}]}`)
	}))
	defer server.Close()
	sdk, err := genai.NewClient(context.Background(), &genai.ClientConfig{APIKey: "test", Backend: genai.BackendGeminiAPI, HTTPOptions: genai.HTTPOptions{BaseURL: server.URL}})
	if err != nil {
		t.Fatal(err)
	}
	c := &Client{client: sdk, model: DefaultModel}
	_, err = c.TwoStep(context.Background(), "Evaluate", &genai.Schema{Type: genai.TypeObject, Properties: map[string]*genai.Schema{"tier": {Type: genai.TypeString}}, Required: []string{"tier"}})
	if err == nil {
		t.Fatal("accepted missing required verdict")
	}
}
