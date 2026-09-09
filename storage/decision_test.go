package storage_test

import (
	"context"
	"github.com/seanmeyer/opportunity-hunter/core"
	"github.com/seanmeyer/opportunity-hunter/testutil"
	"testing"
	"time"
)

func TestEvaluationHasSeparateDecisionStorage(t *testing.T) {
	db := testutil.NewTestDB(t)
	id, err := db.SaveEvaluation(context.Background(), core.Evaluation{HuntName: "powder", GroupKey: "Front Range", EvaluatedAt: time.Now(), RawLLMResponse: "Long research", StructuredResponse: `{"tier":"SKIP"}`})
	if err != nil {
		t.Fatal(err)
	}
	var decision string
	if err := db.RawDB().QueryRow("SELECT structured_response FROM evaluations WHERE id = ?", id).Scan(&decision); err != nil {
		t.Fatalf("no decision storage: %v", err)
	}
	for _, byID := range []bool{true, false} {
		var got core.Evaluation
		if byID {
			got, err = db.GetEvaluation(context.Background(), id)
		} else {
			got, err = db.GetLatestEvaluation(context.Background(), "powder", "Front Range")
		}
		if err != nil || got.StructuredResponse != `{"tier":"SKIP"}` || got.RawLLMResponse != "Long research" {
			t.Fatalf("decision round trip: %+v %v", got, err)
		}
	}
}
