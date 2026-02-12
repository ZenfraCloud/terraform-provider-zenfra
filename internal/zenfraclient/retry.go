// ABOUTME: Retry logic for transient HTTP failures with exponential backoff.
// ABOUTME: Retries on 429 (respecting Retry-After), 502, 503, 504 status codes.

package zenfraclient

import (
	"context"
	"math"
	"math/rand/v2"
	"net/http"
	"strconv"
	"time"
)

const (
	defaultBaseDelay  = 500 * time.Millisecond
	defaultMaxDelay   = 30 * time.Second
	defaultMaxRetries = 3
)

// retryConfig holds retry parameters.
type retryConfig struct {
	baseDelay  time.Duration
	maxDelay   time.Duration
	maxRetries int
}

// defaultRetryConfig returns the default retry configuration.
func defaultRetryConfig() retryConfig {
	return retryConfig{
		baseDelay:  defaultBaseDelay,
		maxDelay:   defaultMaxDelay,
		maxRetries: defaultMaxRetries,
	}
}

// isRetryableStatus returns true if the HTTP status code is retryable.
func isRetryableStatus(statusCode int) bool {
	switch statusCode {
	case http.StatusTooManyRequests, // 429
		http.StatusBadGateway,      // 502
		http.StatusServiceUnavailable, // 503
		http.StatusGatewayTimeout:  // 504
		return true
	default:
		return false
	}
}

// retryDelay calculates the delay for the given attempt using exponential backoff with jitter.
// For 429 responses, it respects the Retry-After header if present.
func retryDelay(cfg retryConfig, attempt int, resp *http.Response) time.Duration {
	// Check Retry-After header for 429 responses.
	if resp != nil && resp.StatusCode == http.StatusTooManyRequests {
		if retryAfter := resp.Header.Get("Retry-After"); retryAfter != "" {
			if seconds, err := strconv.Atoi(retryAfter); err == nil && seconds > 0 {
				delay := time.Duration(seconds) * time.Second
				if delay > cfg.maxDelay {
					delay = cfg.maxDelay
				}
				return delay
			}
		}
	}

	// Exponential backoff: base * 2^attempt with jitter.
	delay := cfg.baseDelay * time.Duration(math.Pow(2, float64(attempt)))
	if delay > cfg.maxDelay {
		delay = cfg.maxDelay
	}

	// Add jitter: +/- 25% of delay.
	jitter := time.Duration(rand.Int64N(int64(delay) / 2))
	delay = delay/2 + delay/4 + jitter

	return delay
}

// sleepWithContext sleeps for the given duration, returning early if the context is cancelled.
func sleepWithContext(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
