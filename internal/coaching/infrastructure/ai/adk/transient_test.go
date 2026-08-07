package adk

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"google.golang.org/genai"
)

// quotaError reproduces the 429 captured from the live API, including the
// RetryInfo detail that carries the delay the server asked for.
func quotaError() error {
	return genai.APIError{
		Code:    429,
		Status:  "RESOURCE_EXHAUSTED",
		Message: "You exceeded your current quota",
		Details: []map[string]any{
			{
				"@type":      "type.googleapis.com/google.rpc.RetryInfo",
				"retryDelay": "2s",
			},
		},
	}
}

func TestIsTransient(t *testing.T) {
	tests := []struct {
		name string
		give error
		want bool
	}{
		{name: "nil"},
		{name: "quota exhausted", give: quotaError(), want: true},
		{name: "service unavailable", give: genai.APIError{Code: 503, Status: "UNAVAILABLE"}, want: true},
		{name: "internal", give: genai.APIError{Code: 500}, want: true},
		{name: "bad request is the caller's fault", give: genai.APIError{Code: 400}},
		{name: "unauthenticated will not fix itself", give: genai.APIError{Code: 401}},
		{name: "wrapped quota still detected", give: fmt.Errorf("generator: %w", quotaError()), want: true},
		{
			name: "status name survives a %v-formatted chain",
			give: errors.New("dynamic child failed: Status: RESOURCE_EXHAUSTED, Details: [...]"),
			want: true,
		},
		{name: "cancellation is terminal", give: context.Canceled},
		{name: "deadline is terminal", give: context.DeadlineExceeded},
		{name: "a plan defect", give: errors.New("exercise_id \"squat-x\" is not in the exercise catalog")},
	}

	for _, tt := range tests {
		tt := tt

		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := isTransient(tt.give); got != tt.want {
				t.Errorf("isTransient = %t, want %t", got, tt.want)
			}
		})
	}
}

func TestTransientDelay_PrefersServerRetryInfo(t *testing.T) {
	// The server said 2s. Guessing the 1s base instead just earns another 429.
	if got, want := transientDelay(quotaError(), 0), 2*time.Second; got != want {
		t.Errorf("delay = %s, want %s", got, want)
	}
}

func TestTransientDelay_BacksOffWithoutRetryInfo(t *testing.T) {
	bare := genai.APIError{Code: 503, Status: "UNAVAILABLE"}

	tests := []struct {
		give int
		want time.Duration
	}{
		{give: 0, want: 1 * time.Second},
		{give: 1, want: 2 * time.Second},
		{give: 2, want: 4 * time.Second},
	}

	for _, tt := range tests {
		if got := transientDelay(bare, tt.give); got != tt.want {
			t.Errorf("delay(try=%d) = %s, want %s", tt.give, got, tt.want)
		}
	}
}

func TestTransientDelay_Capped(t *testing.T) {
	slow := genai.APIError{
		Code: 429,
		Details: []map[string]any{
			{"@type": "type.googleapis.com/google.rpc.RetryInfo", "retryDelay": "10h"},
		},
	}

	if got := transientDelay(slow, 0); got != maxTransientDelay {
		t.Errorf("delay = %s, want it capped at %s", got, maxTransientDelay)
	}
}

func TestRetryTransient_PlanDefectReturnsImmediately(t *testing.T) {
	calls := 0
	defect := errors.New("exercise_id not in catalog")

	_, err := retryTransient(context.Background(), "generator", func() (string, error) {
		calls++
		return "", defect
	})

	if !errors.Is(err, defect) {
		t.Errorf("err = %v, want the defect unwrapped", err)
	}
	if calls != 1 {
		t.Errorf("calls = %d, want 1: a plan defect must not be waited out", calls)
	}
}

func TestRetryTransient_StopsWhenContextEnds(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	calls := 0
	start := time.Now()

	_, err := retryTransient(ctx, "generator", func() (string, error) {
		calls++
		return "", quotaError()
	})

	if err == nil {
		t.Fatal("err = nil, want the cancellation")
	}
	// The server asked for 2s; a cancelled context must not be made to wait it out.
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Errorf("waited %s, want an immediate return on a cancelled context", elapsed)
	}
	if calls != 1 {
		t.Errorf("calls = %d, want 1", calls)
	}
}

func TestRunWithRetries_QuotaErrorDoesNotBurnRounds(t *testing.T) {
	v := newPlanValidator(newFakeCatalog("bench-press"))

	// This is the 11:16 incident: three rounds spent in 0.29s against an API
	// that had asked for a 1s wait.
	att := &recordingAttempt{errs: []error{
		fmt.Errorf("generator: %w", quotaError()),
		fmt.Errorf("generator: %w", quotaError()),
		fmt.Errorf("generator: %w", quotaError()),
	}}

	_, err := runWithRetries(context.Background(), v, att.fn, nil)

	if !errors.Is(err, ErrPlanGenerationFailed) {
		t.Fatalf("err = %v, want ErrPlanGenerationFailed", err)
	}
	if att.calls != 1 {
		t.Errorf("generator calls = %d, want 1: an unrecovered 429 must not be retried as a plan defect", att.calls)
	}
	if !strings.Contains(err.Error(), "infrastructure failure") {
		t.Errorf("err = %v, want it reported as infrastructure, not as a bad plan", err)
	}
}

func TestRunWithRetries_PlanDefectStillRetries(t *testing.T) {
	v := newPlanValidator(newFakeCatalog("bench-press"))

	att := &recordingAttempt{
		errs:  []error{errors.New("model returned prose instead of JSON")},
		plans: []*GeneratedPlan{nil, validPlan()},
	}

	if _, err := runWithRetries(context.Background(), v, att.fn, nil); err != nil {
		t.Fatalf("runWithRetries: %v", err)
	}

	if att.calls != 2 {
		t.Errorf("generator calls = %d, want 2: a genuine defect still earns a retry", att.calls)
	}
}

func TestClassify_TerminalErrorsAreNotRetried(t *testing.T) {
	// Observed live: a key whose project is denied burned all three generation
	// rounds in one second, because 403 is neither transient nor a plan defect.
	denied := genai.APIError{
		Code:    403,
		Status:  "PERMISSION_DENIED",
		Message: "Your project has been denied access. Please contact support.",
	}

	tests := []struct {
		name string
		give error
		want errorClass
	}{
		{name: "quota exhausted waits", give: quotaError(), want: classTransient},
		{name: "unavailable waits", give: genai.APIError{Code: 503}, want: classTransient},
		{name: "permission denied is hopeless", give: denied, want: classTerminal},
		{name: "unauthenticated is hopeless", give: genai.APIError{Code: 401}, want: classTerminal},
		{name: "invalid argument is our bug", give: genai.APIError{Code: 400}, want: classTerminal},
		{name: "unknown model is hopeless", give: genai.APIError{Code: 404}, want: classTerminal},
		{name: "cancellation is hopeless", give: context.Canceled, want: classTerminal},
		{name: "wrapped 403 still detected", give: fmt.Errorf("generator: %w", denied), want: classTerminal},
		{
			name: "status name survives a %v-formatted chain",
			give: errors.New("dynamic child failed: Status: PERMISSION_DENIED"),
			want: classTerminal,
		},
		{name: "bad json is the model's fault", give: errors.New("unmarshal plan: invalid character"), want: classPlanDefect},
	}

	for _, tt := range tests {
		tt := tt

		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := classify(tt.give); got != tt.want {
				t.Errorf("classify = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestRunWithRetries_DeniedKeyDoesNotBurnRounds(t *testing.T) {
	v := newPlanValidator(newFakeCatalog("bench-press"))

	denied := genai.APIError{Code: 403, Status: "PERMISSION_DENIED"}
	att := &recordingAttempt{errs: []error{
		fmt.Errorf("generator: %w", denied),
		fmt.Errorf("generator: %w", denied),
		fmt.Errorf("generator: %w", denied),
	}}

	_, err := runWithRetries(context.Background(), v, att.fn, nil)

	if !errors.Is(err, ErrPlanGenerationFailed) {
		t.Fatalf("err = %v, want ErrPlanGenerationFailed", err)
	}
	if att.calls != 1 {
		t.Errorf("generator calls = %d, want 1: a denied key must not be retried as a plan defect", att.calls)
	}
	if !strings.Contains(err.Error(), "infrastructure failure") {
		t.Errorf("err = %v, want it reported as infrastructure", err)
	}
}

func TestRetryTransient_TerminalErrorReturnsImmediately(t *testing.T) {
	calls := 0
	start := time.Now()

	_, err := retryTransient(context.Background(), "generator", func() (string, error) {
		calls++
		return "", genai.APIError{Code: 403, Status: "PERMISSION_DENIED"}
	})

	if err == nil {
		t.Fatal("err = nil, want the denial")
	}
	if calls != 1 {
		t.Errorf("calls = %d, want 1: a denial must not be waited out", calls)
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Errorf("waited %s, want an immediate return", elapsed)
	}
}
