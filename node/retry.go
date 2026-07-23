package node

import (
	"context"
	"math/rand"
	"time"
)

// WithRetry executes the operation with exponential backoff and random jitter.
func WithRetry(ctx context.Context, maxRetries int, baseDelay time.Duration, op func() error) error {
	var err error
	for i := 0; i < maxRetries; i++ {
		if err = op(); err == nil {
			return nil
		}
		if i == maxRetries-1 {
			break
		}

		// Exponential backoff
		delay := baseDelay * time.Duration(1<<i)
		// Add small random jitter (up to 25% of delay)
		jitter := time.Duration(rand.Int63n(max(1, int64(delay)/4)))
		delay += jitter

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
		}
	}
	return err
}

func max(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
