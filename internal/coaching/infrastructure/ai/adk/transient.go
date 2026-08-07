package adk

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"google.golang.org/genai"
)

const (
	// maxTransientRetries is a budget of its own: a rate limit is not a defect
	// in the plan, so waiting one out must not spend a generation round.
	maxTransientRetries = 2

	transientBaseDelay = 1 * time.Second
	maxTransientDelay  = 30 * time.Second
)

// retryableStatusCodes are the HTTP codes worth waiting out. 429 is the one
// that matters in practice: the free tier caps requests per day, and the API
// tells us how long to wait.
var retryableStatusCodes = map[int]struct{}{
	429: {}, // RESOURCE_EXHAUSTED
	500: {}, // INTERNAL
	502: {},
	503: {}, // UNAVAILABLE
	504: {}, // DEADLINE_EXCEEDED
}

// isTransient reports whether err is an infrastructure failure rather than a
// defect in the generated plan.
//
// The distinction decides whether the loop may retry: regenerating after a 429
// produces the identical 429, so treating one as "the model wrote a bad plan"
// burns every round in milliseconds and feeds the next prompt an API error dump
// the model cannot act on.
func isTransient(err error) bool {
	if err == nil {
		return false
	}

	// A cancelled context is terminal, never transient, even though the
	// underlying call may look like a timeout.
	if errors.Is(err, context.Canceled) {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return false
	}

	var apiErr genai.APIError
	if errors.As(err, &apiErr) {
		_, ok := retryableStatusCodes[apiErr.Code]
		return ok
	}

	// Fallback for layers that formatted the cause with %v instead of %w, which
	// breaks the errors.As chain. Matching on the gRPC status name rather than
	// prose keeps this from firing on a plan whose text happens to say "429".
	return strings.Contains(err.Error(), "RESOURCE_EXHAUSTED") ||
		strings.Contains(err.Error(), "UNAVAILABLE")
}

// transientDelay is how long to wait before retrying try (0-based).
//
// The API's own RetryInfo wins when present: it knows when the quota window
// reopens and guessing shorter only earns another rejection.
func transientDelay(err error, try int) time.Duration {
	if d, ok := retryInfoDelay(err); ok {
		return capDelay(d)
	}

	delay := transientBaseDelay << try // 1s, 2s, 4s, ...
	return capDelay(delay)
}

func capDelay(d time.Duration) time.Duration {
	if d <= 0 {
		return transientBaseDelay
	}
	if d > maxTransientDelay {
		return maxTransientDelay
	}
	return d
}

// retryInfoDelay pulls google.rpc.RetryInfo out of a genai APIError's details.
func retryInfoDelay(err error) (time.Duration, bool) {
	var apiErr genai.APIError
	if !errors.As(err, &apiErr) {
		return 0, false
	}

	for _, detail := range apiErr.Details {
		kind, _ := detail["@type"].(string)
		if !strings.HasSuffix(kind, "google.rpc.RetryInfo") {
			continue
		}

		// Serialized as a protobuf Duration string, e.g. "1s" or "0.5s".
		raw, ok := detail["retryDelay"].(string)
		if !ok {
			continue
		}
		if d, parseErr := time.ParseDuration(raw); parseErr == nil {
			return d, true
		}
	}

	return 0, false
}

// retryTransient calls fn, waiting out infrastructure failures on their own
// budget. Plan defects and terminal errors are returned untouched on the first
// try, so the caller's retry logic sees them immediately.
func retryTransient[T any](ctx context.Context, label string, fn func() (T, error)) (T, error) {
	var zero T

	for try := 0; ; try++ {
		out, err := fn()
		if err == nil {
			return out, nil
		}

		if !isTransient(err) || try >= maxTransientRetries {
			return zero, err
		}

		delay := transientDelay(err, try)
		log.Printf("coaching: %s hit a transient failure, waiting %s before retry %d/%d: %v",
			label, delay, try+1, maxTransientRetries, err)

		if waitErr := sleepCtx(ctx, delay); waitErr != nil {
			return zero, waitErr
		}
	}
}

// sleepCtx waits for d unless ctx ends first.
func sleepCtx(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()

	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("wait interrupted: %w", ctx.Err())
	}
}
