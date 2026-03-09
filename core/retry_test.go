package core_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/seanmeyer/opportunity-hunter/core"
)

func TestWithRetry_SuccessFirstAttempt(t *testing.T) {
	calls := 0
	result, err := core.WithRetry(context.Background(), "test", core.DefaultRetryDelays, func() (string, error) {
		calls++
		return "ok", nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "ok" {
		t.Fatalf("expected 'ok', got %q", result)
	}
	if calls != 1 {
		t.Fatalf("expected 1 call, got %d", calls)
	}
}

func TestWithRetry_SuccessAfterRetries(t *testing.T) {
	calls := 0
	delays := []time.Duration{0, 0, 0} // no actual delays in test
	result, err := core.WithRetry(context.Background(), "test", delays, func() (int, error) {
		calls++
		if calls < 3 {
			return 0, errors.New("transient")
		}
		return 42, nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != 42 {
		t.Fatalf("expected 42, got %d", result)
	}
	if calls != 3 {
		t.Fatalf("expected 3 calls, got %d", calls)
	}
}

func TestWithRetry_AllAttemptsFail(t *testing.T) {
	calls := 0
	delays := []time.Duration{0, 0}
	_, err := core.WithRetry(context.Background(), "test", delays, func() (string, error) {
		calls++
		return "", errors.New("permanent")
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if calls != 2 {
		t.Fatalf("expected 2 calls, got %d", calls)
	}
}

func TestWithRetry_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	delays := []time.Duration{0, time.Second} // second attempt has delay
	calls := 0
	_, err := core.WithRetry(ctx, "test", delays, func() (string, error) {
		calls++
		return "", errors.New("fail")
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}
