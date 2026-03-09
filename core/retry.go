package core

import (
	"context"
	"log/slog"
	"time"
)

// DefaultRetryDelays is the standard retry schedule for external calls.
var DefaultRetryDelays = []time.Duration{0, 1 * time.Second, 5 * time.Second}

// WithRetry retries fn up to len(delays) times with the given delays between attempts.
func WithRetry[T any](ctx context.Context, name string, delays []time.Duration, fn func() (T, error)) (T, error) {
	var lastErr error
	for i, delay := range delays {
		if delay > 0 {
			select {
			case <-ctx.Done():
				var zero T
				return zero, ctx.Err()
			case <-time.After(delay):
			}
		}
		result, err := fn()
		if err == nil {
			return result, nil
		}
		lastErr = err
		slog.Warn("retry", "attempt", i+1, "op", name, "err", err)
	}
	var zero T
	return zero, lastErr
}
