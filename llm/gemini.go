package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"strings"
	"time"

	genai "google.golang.org/genai"
)

const (
	retryMaxAttempts = 4
	retryBaseDelay   = 3 * time.Second
	DefaultModel     = "gemini-2.5-flash"
)

// Client wraps the Gemini API client.
type Client struct {
	client *genai.Client
	model  string
}

// NewClient creates a new Gemini LLM client.
func NewClient(ctx context.Context, apiKey string) (*Client, error) {
	c, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  apiKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return nil, fmt.Errorf("create genai client: %w", err)
	}
	return &Client{client: c, model: DefaultModel}, nil
}

// TwoStepResult holds the output of a two-step evaluation.
type TwoStepResult struct {
	Research   string            // research text from grounded search
	Structured map[string]any   // parsed JSON from structured extraction
	Sources    []string          // grounding sources from research
	RawJSON    string            // raw JSON string from structured step
	CostUSD    float64           // estimated cost of both API calls
}

// TwoStep performs the two-step evaluation pattern:
// 1. Research with Google Search grounding
// 2. Structured JSON extraction with schema
func (c *Client) TwoStep(ctx context.Context, prompt string, schema *genai.Schema) (TwoStepResult, error) {
	var result TwoStepResult

	// Step 1: Grounded research.
	researchConfig := &genai.GenerateContentConfig{
		Tools: []*genai.Tool{
			{GoogleSearch: &genai.GoogleSearch{}},
		},
	}
	researchContents := []*genai.Content{
		genai.NewContentFromText(prompt, genai.RoleUser),
	}
	researchResp, err := c.generateWithRetry(ctx, researchContents, researchConfig, "research")
	if err != nil {
		return result, fmt.Errorf("research step: %w", err)
	}
	result.Research = researchResp.Text()
	result.Sources = extractSources(researchResp)
	result.CostUSD += responseCost(c.model, researchResp)

	// Step 2: Structured extraction.
	structurePrompt := fmt.Sprintf(`You are a JSON extraction assistant. Parse the analysis below into the required JSON schema exactly. Preserve all specific details.

## Analysis

%s`, result.Research)

	structureConfig := &genai.GenerateContentConfig{
		ResponseMIMEType: "application/json",
		ResponseSchema:   schema,
	}
	structureContents := []*genai.Content{
		genai.NewContentFromText(structurePrompt, genai.RoleUser),
	}
	structureResp, err := c.generateWithRetry(ctx, structureContents, structureConfig, "structure")
	if err != nil {
		return result, fmt.Errorf("structure step: %w", err)
	}

	result.RawJSON = structureResp.Text()
	result.CostUSD += responseCost(c.model, structureResp)
	if err := json.Unmarshal([]byte(result.RawJSON), &result.Structured); err != nil {
		return result, fmt.Errorf("parse structured response: %w", err)
	}

	return result, nil
}

// GenerateResult holds the output of a single generation call.
type GenerateResult struct {
	Text    string
	CostUSD float64
}

// Generate performs a single generation call.
func (c *Client) Generate(ctx context.Context, prompt string, config *genai.GenerateContentConfig) (GenerateResult, error) {
	contents := []*genai.Content{
		genai.NewContentFromText(prompt, genai.RoleUser),
	}
	resp, err := c.generateWithRetry(ctx, contents, config, "generate")
	if err != nil {
		return GenerateResult{}, err
	}
	return GenerateResult{
		Text:    resp.Text(),
		CostUSD: responseCost(c.model, resp),
	}, nil
}

func (c *Client) generateWithRetry(ctx context.Context, contents []*genai.Content, config *genai.GenerateContentConfig, step string) (*genai.GenerateContentResponse, error) {
	var lastErr error
	for attempt := range retryMaxAttempts {
		if attempt > 0 {
			delay := retryBaseDelay * time.Duration(math.Pow(2, float64(attempt-1)))
			slog.Warn("llm retry", "step", step, "attempt", attempt+1, "delay", delay, "err", lastErr)
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(delay):
			}
		}

		resp, err := c.client.Models.GenerateContent(ctx, c.model, contents, config)
		if err != nil {
			lastErr = err
			continue
		}
		return resp, nil
	}
	return nil, fmt.Errorf("llm %s: all %d attempts failed: %w", step, retryMaxAttempts, lastErr)
}

func extractSources(resp *genai.GenerateContentResponse) []string {
	var sources []string
	if resp == nil || resp.Candidates == nil {
		return sources
	}
	for _, cand := range resp.Candidates {
		if cand.GroundingMetadata != nil {
			for _, chunk := range cand.GroundingMetadata.GroundingChunks {
				if chunk.Web != nil && chunk.Web.URI != "" {
					sources = append(sources, chunk.Web.URI)
				}
			}
		}
	}
	return sources
}

// responseCost extracts token counts from a Gemini response and estimates cost.
func responseCost(model string, resp *genai.GenerateContentResponse) float64 {
	if resp == nil || resp.UsageMetadata == nil {
		return 0
	}
	return EstimateCost(model, int(resp.UsageMetadata.PromptTokenCount), int(resp.UsageMetadata.CandidatesTokenCount))
}

// EstimateCost estimates the cost of a Gemini API call in USD.
// Based on approximate Gemini pricing as of 2026.
func EstimateCost(model string, inputTokens, outputTokens int) float64 {
	// Gemini Flash: ~$0.10/1M input, ~$0.40/1M output (approximate)
	if strings.Contains(model, "flash") {
		return float64(inputTokens)*0.0000001 + float64(outputTokens)*0.0000004
	}
	// Gemini Pro: ~$1.25/1M input, ~$5.00/1M output (approximate)
	return float64(inputTokens)*0.00000125 + float64(outputTokens)*0.000005
}
