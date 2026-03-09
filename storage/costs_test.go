package storage_test

import (
	"context"
	"math"
	"testing"
	"time"
)

func TestRecordAndMonthlySpend(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	db.RecordCost(ctx, "comedy", 0.003, "gemini-2.0-flash", true)
	db.RecordCost(ctx, "comedy", 0.005, "gemini-2.0-flash", true)
	db.RecordCost(ctx, "powder", 0.010, "gemini-2.0-flash", true)

	result, err := db.MonthlySpend(ctx, time.Now())
	if err != nil {
		t.Fatal(err)
	}

	assertClose := func(name string, got, want float64) {
		t.Helper()
		if math.Abs(got-want) > 0.0001 {
			t.Fatalf("%s: expected %.4f, got %.4f", name, want, got)
		}
	}

	assertClose("total", result.Total, 0.018)
	assertClose("comedy", result.ByHunt["comedy"], 0.008)
	assertClose("powder", result.ByHunt["powder"], 0.010)
}
