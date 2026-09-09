package powder

import (
	"context"
	"encoding/json"
	"github.com/seanmeyer/opportunity-hunter/core"
	"github.com/seanmeyer/opportunity-hunter/llm"
	genai "google.golang.org/genai"
	"testing"
)

type judgmentResponse struct{ decision map[string]any }

func (f judgmentResponse) TwoStepAdvice(context.Context, string, *genai.Schema) (llm.TwoStepResult, error) {
	b, _ := json.Marshal(f.decision)
	return llm.TwoStepResult{Research: "Detailed research, not JSON", Structured: f.decision, RawJSON: string(b)}, nil
}

func TestEvaluatorPreservesSkipAndDowngrade(t *testing.T) {
	e := powderEvaluator{llm: judgmentResponse{map[string]any{"tier": "SKIP", "recommendation": "Skip: access closed", "summary": "Not worth the trip"}}}
	result, err := e.Evaluate(context.Background(), core.EvalContext{Opportunities: []core.Opportunity{{ID: 7, Attributes: PowderAttrs{SnowfallIn: 40}.Encode()}}, PriorEval: &core.Evaluation{RawLLMResponse: "Old research"}, PriorPicks: []core.Pick{{OpportunityID: 7, DisplayScore: "WORTH_A_LOOK", Attributes: PowderAttrs{SnowfallIn: 30}.Encode()}}})
	if err != nil {
		t.Fatal(err)
	}
	p := result.Picks[0]
	if p.Score != 0 || p.DisplayScore != "SKIP" || p.Reason != "Skip: access closed" || changeClassFromPick(p) != "downgrade" {
		t.Fatalf("lost negative judgment: %+v", p)
	}
	if result.Evaluation.StructuredResponse == "" || result.Evaluation.RawLLMResponse != "Detailed research, not JSON" {
		t.Fatal("lost decision/research separation")
	}
}

func TestEvaluatorRejectsMissingJudgment(t *testing.T) {
	for _, d := range []map[string]any{{"tier": "invented", "recommendation": "Go"}, {"tier": "RECOMMENDED", "recommendation": " "}} {
		e := powderEvaluator{llm: judgmentResponse{d}}
		if _, err := e.Evaluate(context.Background(), core.EvalContext{}); err == nil {
			t.Fatal("accepted malformed judgment")
		}
	}
}
