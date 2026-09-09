package storage_test

import (
	"context"
	"github.com/seanmeyer/opportunity-hunter/core"
	"github.com/seanmeyer/opportunity-hunter/storage"
	"path/filepath"
	"testing"
	"time"
)

func TestDecisionMigrationPreservesOldEvaluationsAndReopens(t *testing.T) {
	path := filepath.Join(t.TempDir(), "old.db")
	db, err := storage.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	id, err := db.SaveEvaluation(context.Background(), core.Evaluation{HuntName: "powder", RawLLMResponse: "Existing research", EvaluatedAt: time.Now()})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.RawDB().Exec("ALTER TABLE evaluations DROP COLUMN structured_response"); err != nil {
		t.Fatal(err)
	}
	db.Close()
	for range 2 {
		db, err = storage.Open(path)
		if err != nil {
			t.Fatal(err)
		}
		got, err := db.GetEvaluation(context.Background(), id)
		if err != nil || got.RawLLMResponse != "Existing research" || got.StructuredResponse != "" {
			t.Fatalf("migration lost old data: %+v %v", got, err)
		}
		db.Close()
	}
}
