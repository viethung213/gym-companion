package adk

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// recordingAttempt wraps a sequence of canned responses and records what
// feedback it was handed on each call.
type recordingAttempt struct {
	plans      []*GeneratedPlan
	errs       []error
	calls      int
	seenIssues  [][]string
	seenPlans   []*GeneratedPlan
	seenReviews []*PlanReview
}

func (r *recordingAttempt) fn(attempt int, fb *attemptFeedback) (*GeneratedPlan, error) {
	r.calls++
	r.seenIssues = append(r.seenIssues, fb.Issues)
	r.seenPlans = append(r.seenPlans, fb.PreviousPlan)
	r.seenReviews = append(r.seenReviews, fb.Review)

	i := attempt - 1
	if i < len(r.errs) && r.errs[i] != nil {
		return nil, r.errs[i]
	}
	if i < len(r.plans) {
		return r.plans[i], nil
	}
	if len(r.plans) == 0 {
		return nil, errors.New("no canned plan")
	}
	return r.plans[len(r.plans)-1], nil
}

// alwaysReturns builds an attempt func that hands back the same plan every time.
func alwaysReturns(plan *GeneratedPlan) *recordingAttempt {
	return &recordingAttempt{plans: []*GeneratedPlan{plan, plan, plan}}
}

func TestRunWithRetries_SucceedsFirstAttempt(t *testing.T) {
	v := newPlanValidator(newFakeCatalog("bench-press"))
	att := alwaysReturns(planOf(sessionOf("2026-08-03", "bench-press")))

	got, err := runWithRetries(context.Background(), v, att.fn, nil)
	if err != nil {
		t.Fatalf("runWithRetries returned error: %v", err)
	}

	if att.calls != 1 {
		t.Errorf("got %d attempts, want 1", att.calls)
	}
	if got.Degraded {
		t.Error("got Degraded = true, want false")
	}
}

func TestRunWithRetries_SucceedsOnSecondAttempt(t *testing.T) {
	v := newPlanValidator(newFakeCatalog("bench-press"))
	att := &recordingAttempt{plans: []*GeneratedPlan{
		planOf(sessionOf("2026-08-03", "made-up")),
		planOf(sessionOf("2026-08-03", "bench-press")),
	}}

	got, err := runWithRetries(context.Background(), v, att.fn, nil)
	if err != nil {
		t.Fatalf("runWithRetries returned error: %v", err)
	}

	if att.calls != 2 {
		t.Errorf("got %d attempts, want 2", att.calls)
	}
	if got.Degraded {
		t.Error("got Degraded = true, want false: attempt 2 was clean")
	}
}

// TestRunWithRetries_FeedsIssuesForward proves the feedback is actually plumbed
// through rather than merely declared: the second attempt must be told which ID
// was rejected.
func TestRunWithRetries_FeedsIssuesForward(t *testing.T) {
	v := newPlanValidator(newFakeCatalog("bench-press"))
	att := &recordingAttempt{plans: []*GeneratedPlan{
		planOf(sessionOf("2026-08-03", "barbell-bench-press")),
		planOf(sessionOf("2026-08-03", "bench-press")),
	}}

	if _, err := runWithRetries(context.Background(), v, att.fn, nil); err != nil {
		t.Fatalf("runWithRetries returned error: %v", err)
	}

	if len(att.seenIssues) < 2 {
		t.Fatalf("got %d recorded calls, want at least 2", len(att.seenIssues))
	}
	if len(att.seenIssues[0]) != 0 {
		t.Errorf("got %v on the first attempt, want no prior issues", att.seenIssues[0])
	}

	second := strings.Join(att.seenIssues[1], "\n")
	if !strings.Contains(second, "barbell-bench-press") {
		t.Errorf("got feedback %q, want it to name the rejected id barbell-bench-press", second)
	}
}

func TestRunWithRetries_ThirdAttemptDropsInvalid(t *testing.T) {
	v := newPlanValidator(newFakeCatalog("bench-press", "squat"))
	att := alwaysReturns(planOf(sessionOf("2026-08-03", "bench-press", "made-up", "squat")))

	got, err := runWithRetries(context.Background(), v, att.fn, nil)
	if err != nil {
		t.Fatalf("runWithRetries returned error: %v", err)
	}

	if att.calls != maxGenerationAttempts {
		t.Errorf("got %d attempts, want %d", att.calls, maxGenerationAttempts)
	}
	if !got.Degraded {
		t.Error("got Degraded = false, want true")
	}

	mains := got.Plan.Weeks[0].Sessions[0].Prescription.MainExercises
	if len(mains) != 2 {
		t.Fatalf("got %d surviving exercises, want 2", len(mains))
	}
	for _, m := range mains {
		if m.ExerciseID == "made-up" {
			t.Error("invalid exercise survived the salvage pass")
		}
	}
	if len(got.Issues) == 0 {
		t.Error("got no Issues on a degraded result, want the salvage reasons recorded")
	}
}

func TestRunWithRetries_AllInvalidReturnsError(t *testing.T) {
	v := newPlanValidator(newFakeCatalog("bench-press"))
	att := alwaysReturns(planOf(sessionOf("2026-08-03", "fake-a", "fake-b")))

	got, err := runWithRetries(context.Background(), v, att.fn, nil)
	if err == nil {
		t.Fatal("got nil error, want ErrPlanGenerationFailed")
	}
	if !errors.Is(err, ErrPlanGenerationFailed) {
		t.Errorf("got error %v, want it to wrap ErrPlanGenerationFailed", err)
	}
	if got != nil {
		t.Errorf("got result %+v, want nil", got)
	}
	if att.calls != maxGenerationAttempts {
		t.Errorf("got %d attempts, want %d", att.calls, maxGenerationAttempts)
	}
}

func TestRunWithRetries_ParseErrorIsRetried(t *testing.T) {
	v := newPlanValidator(newFakeCatalog("bench-press"))
	att := &recordingAttempt{
		errs:  []error{errors.New("invalid character 'x' looking for beginning of value")},
		plans: []*GeneratedPlan{nil, planOf(sessionOf("2026-08-03", "bench-press"))},
	}

	got, err := runWithRetries(context.Background(), v, att.fn, nil)
	if err != nil {
		t.Fatalf("runWithRetries returned error: %v", err)
	}
	if att.calls != 2 {
		t.Errorf("got %d attempts, want 2", att.calls)
	}
	if got.Degraded {
		t.Error("got Degraded = true, want false")
	}

	second := strings.Join(att.seenIssues[1], "\n")
	if !strings.Contains(second, "invalid character") {
		t.Errorf("got feedback %q, want it to carry the parse failure", second)
	}
}

// TestRunWithRetries_CatalogOutageAbortsImmediately guards against burning
// three model calls when the catalog is simply unreachable.
func TestRunWithRetries_CatalogOutageAbortsImmediately(t *testing.T) {
	outage := errors.New("connection refused")
	cat := newFakeCatalog("bench-press")
	cat.err = outage
	v := newPlanValidator(cat)
	att := alwaysReturns(planOf(sessionOf("2026-08-03", "bench-press")))

	_, err := runWithRetries(context.Background(), v, att.fn, nil)
	if err == nil {
		t.Fatal("got nil error, want the transport failure")
	}
	if !errors.Is(err, outage) {
		t.Errorf("got error %v, want it to wrap %v", err, outage)
	}
	if errors.Is(err, ErrPlanGenerationFailed) {
		t.Error("an outage must not be reported as a plan-generation failure")
	}
	if att.calls != 1 {
		t.Errorf("got %d attempts, want 1: an outage must not be retried", att.calls)
	}
}

func TestRunWithRetries_StopsAtMaxAttempts(t *testing.T) {
	v := newPlanValidator(newFakeCatalog("bench-press"))
	att := &recordingAttempt{errs: []error{
		errors.New("boom 1"), errors.New("boom 2"), errors.New("boom 3"),
	}}

	if _, err := runWithRetries(context.Background(), v, att.fn, nil); err == nil {
		t.Fatal("got nil error, want failure after exhausting attempts")
	}

	if att.calls != maxGenerationAttempts {
		t.Errorf("got %d attempts, want exactly %d", att.calls, maxGenerationAttempts)
	}
}

func TestRunWithRetries_HonoursCancelledContext(t *testing.T) {
	v := newPlanValidator(newFakeCatalog("bench-press"))
	att := alwaysReturns(planOf(sessionOf("2026-08-03", "bench-press")))

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := runWithRetries(ctx, v, att.fn, nil)
	if err == nil {
		t.Fatal("got nil error, want context cancellation")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("got error %v, want it to wrap context.Canceled", err)
	}
	if att.calls != 0 {
		t.Errorf("got %d attempts, want 0 on an already-cancelled context", att.calls)
	}
}

func TestFormatIssues_CapsFeedbackLength(t *testing.T) {
	issues := make([]planIssue, 0, maxFeedbackIssues+5)
	for i := range maxFeedbackIssues + 5 {
		issues = append(issues, planIssue{
			WeekNumber: 1, SessionIdx: i, Slot: "main_exercises",
			ExerciseID: "fake", Reason: "is not in the exercise catalog",
		})
	}

	got := formatIssues(issues)
	if len(got) != maxFeedbackIssues+1 {
		t.Fatalf("got %d lines, want %d capped issues plus one summary",
			len(got), maxFeedbackIssues+1)
	}
	if !strings.Contains(got[len(got)-1], "further issues") {
		t.Errorf("got last line %q, want a truncation summary", got[len(got)-1])
	}
}
