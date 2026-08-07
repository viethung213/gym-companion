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

// errorClass says how the retry loop must treat a failed model call.
//
// Three classes, because two is not enough: an infrastructure failure can be
// worth waiting out or hopeless, and conflating "hopeless" with "the model
// wrote a bad plan" burns every generation round on an error no regeneration
// can fix. A denied API key did exactly that.
type errorClass int

const (
	// classPlanDefect: the model produced something unusable. Retrying with
	// feedback is the whole point of the loop.
	classPlanDefect errorClass = iota

	// classTransient: the call may succeed later. Wait on a separate budget.
	classTransient

	// classTerminal: no retry helps — bad key, denied project, unknown model,
	// malformed request, cancelled context.
	classTerminal
)

// classifyStatus maps an HTTP status from the model API to a retry class.
func classifyStatus(code int) errorClass {
	switch code {
	case 429, // RESOURCE_EXHAUSTED — quota window reopens
		500, // INTERNAL
		502,
		503, // UNAVAILABLE
		504: // DEADLINE_EXCEEDED
		return classTransient

	case 400, // INVALID_ARGUMENT — our request is wrong, not the plan
		401, // UNAUTHENTICATED
		403, // PERMISSION_DENIED — denied project or key
		404: // NOT_FOUND — no such model
		return classTerminal

	default:
		// An unrecognised status is treated as the model's fault only because
		// the loop already bounds that path; a wrong guess costs 2 extra calls,
		// not an unbounded retry.
		return classPlanDefect
	}
}

// classify decides how the loop must treat err.
//
// Regenerating after a 429 produces the identical 429; regenerating after a 403
// produces the identical 403. Treating either as "the model wrote a bad plan"
// burns every round in milliseconds and feeds the next prompt an API error dump
// the model cannot act on.
func classify(err error) errorClass {
	if err == nil {
		return classPlanDefect
	}

	// Cancellation is terminal even though the underlying call may look like a
	// timeout: the caller has already given up.
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return classTerminal
	}

	var apiErr genai.APIError
	if errors.As(err, &apiErr) {
		return classifyStatus(apiErr.Code)
	}

	// Fallback for layers that formatted the cause with %v instead of %w, which
	// breaks the errors.As chain. Matching on the gRPC status name rather than
	// prose keeps this from firing on a plan whose text happens to say "429".
	msg := err.Error()
	switch {
	case strings.Contains(msg, "RESOURCE_EXHAUSTED"),
		strings.Contains(msg, "UNAVAILABLE"):
		return classTransient
	case strings.Contains(msg, "PERMISSION_DENIED"),
		strings.Contains(msg, "UNAUTHENTICATED"),
		strings.Contains(msg, "INVALID_ARGUMENT"):
		return classTerminal
	default:
		return classPlanDefect
	}
}

// isTransient reports whether err is worth waiting out.
func isTransient(err error) bool { return classify(err) == classTransient }

// isUnfixableByModel reports whether no amount of regeneration can help, so the
// loop must surface err instead of spending a round on it.
func isUnfixableByModel(err error) bool { return classify(err) != classPlanDefect }

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
