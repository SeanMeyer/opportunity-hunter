package llm

import (
	"context"
	"encoding/json"
	"fmt"
	genai "google.golang.org/genai"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestTwoStepExtractionReceivesRetrievedSourcesAndBudgetDistinction(t *testing.T) {
	var requests []map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		requests = append(requests, body)
		answer := "Recommended only if access confirms; $800 is a value reference."
		if len(requests) == 2 {
			answer = `{"tier":"RECOMMENDED"}`
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"candidates":[{"content":{"role":"model","parts":[{"text":%q}]},"finishReason":"STOP","groundingMetadata":{"groundingChunks":[{"web":{"uri":"https://example.com/status","title":"Operations"}}]}}]}`, answer)
	}))
	defer server.Close()
	sdk, err := genai.NewClient(context.Background(), &genai.ClientConfig{APIKey: "test", Backend: genai.BackendGeminiAPI, HTTPOptions: genai.HTTPOptions{BaseURL: server.URL}})
	if err != nil {
		t.Fatal(err)
	}
	c := &Client{client: sdk, model: DefaultModel}
	prompt := "Give independent advice. Respect no flights."
	schema := &genai.Schema{Type: genai.TypeObject, Properties: map[string]*genai.Schema{"tier": {Type: genai.TypeString}}, Required: []string{"tier"}}
	result, err := c.TwoStepAdvice(context.Background(), prompt, schema)
	if err != nil {
		t.Fatal(err)
	}
	if len(requests) != 2 || result.RenderedPrompt != prompt || result.Structured["tier"] != "RECOMMENDED" {
		t.Fatalf("unexpected result: %+v", result)
	}
	first, _ := json.Marshal(requests[0])
	second, _ := json.Marshal(requests[1])
	if strings.Contains(string(first), "responseSchema") || strings.Contains(string(first), "Output contract") || !strings.Contains(string(first), "googleSearch") {
		t.Fatalf("research request: %s", first)
	}
	for _, want := range []string{"responseSchema", "No flights", "https://example.com/status", "illustrative budgets", "trip cost estimates", "suggested future checks", "Preserve decisive conditions in both recommendation and summary"} {
		if !strings.Contains(strings.ToLower(string(second)), strings.ToLower(want)) {
			t.Errorf("extraction missing %q", want)
		}
	}
}

func TestExtractionWithoutGroundingGetsEmptySourceList(t *testing.T) {
	var extraction string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		content := body["contents"].([]any)[0].(map[string]any)
		text := content["parts"].([]any)[0].(map[string]any)["text"].(string)
		if strings.Contains(text, "## Analysis") {
			extraction = text
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"candidates":[{"content":{"role":"model","parts":[{"text":"{}"}]},"finishReason":"STOP"}]}`)
	}))
	defer server.Close()
	sdk, err := genai.NewClient(context.Background(), &genai.ClientConfig{APIKey: "test", Backend: genai.BackendGeminiAPI, HTTPOptions: genai.HTTPOptions{BaseURL: server.URL}})
	if err != nil {
		t.Fatal(err)
	}
	c := &Client{client: sdk, model: DefaultModel}
	if _, err := c.TwoStep(context.Background(), "Evaluate", &genai.Schema{Type: genai.TypeObject}); err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(extraction, "Retrieved source URLs:\n[]") {
		t.Fatal("missing explicit empty source list")
	}
}
