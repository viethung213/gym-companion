package adk

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// recordingReview hands back canned verdicts and records what it was asked about.
type recordingReview struct {
	verdicts []*PlanReview
	errs     []error
	calls    int
	seen     []ValidationReport
}

func (r *recordingReview) fn(round int, _ *GeneratedPlan, report ValidationReport) (*PlanReview, error) {
	r.calls++
	r.seen = append(r.seen, report)

	i := round - 1
	if i < len(r.errs) && r.errs[i] != nil {
		return nil, r.errs[i]
	}
	if i < len(r.verdicts) {
		return r.verdicts[i], nil
	}
	if len(r.verdicts) == 0 {
		return nil, errors.New("no canned verdict")
	}
	return r.verdicts[len(r.verdicts)-1], nil
}

func approve(score int) *PlanReview {
	return &PlanReview{Approved: true, Score: score, Confidence: 0.9}
}

func reject(score int, confidence float64) *PlanReview {
	return &PlanReview{
		Approved:   false,
		Score:      score,
		Confidence: confidence,
		Feedback: []ReviewNote{{
			Area:   "goal_fit",
			Detail: "week 1 prescribes 4 reps for a hypertrophy goal",
			Fix:    "raise main-exercise target_reps into the 8-12 range",
		}},
	}
}

func validPlan() *GeneratedPlan {
	return planOf(sessionOf("2026-08-03", "bench-press"))
}

func TestReviewLoop_ApprovedShipsImmediately(t *testing.T) {
	v := newPlanValidator(newFakeCatalog("bench-press"))
	att := alwaysReturns(validPlan())
	rev := &recordingReview{verdicts: []*PlanReview{approve(90)}}

	got, err := runWithRetries(context.Background(), v, att.fn, rev.fn)
	if err != nil {
		t.Fatalf("runWithRetries: %v", err)
	}

	if att.calls != 1 {
		t.Errorf("generator calls = %d, want 1", att.calls)
	}
	if rev.calls != 1 {
		t.Errorf("review calls = %d, want 1", rev.calls)
	}
	if got.Degraded {
		t.Errorf("Degraded = true, want false")
	}
}

func TestReviewLoop_RejectionSendsStructuredFeedbackBack(t *testing.T) {
	v := newPlanValidator(newFakeCatalog("bench-press"))
	plan := validPlan()
	att := alwaysReturns(plan)
	rev := &recordingReview{verdicts: []*PlanReview{reject(40, 0.9), approve(85)}}

	got, err := runWithRetries(context.Background(), v, att.fn, rev.fn)
	if err != nil {
		t.Fatalf("runWithRetries: %v", err)
	}

	if att.calls != 2 {
		t.Fatalf("generator calls = %d, want 2", att.calls)
	}

	// Round 2 must carry the plan under revision and the verdict, not a bare
	// "try again": that is the whole point of reflection.
	if got := att.seenPlans[1]; got == nil {
		t.Errorf("round 2 previous_plan = nil, want the rejected plan")
	}
	fb := att.seenReviews[1]
	if fb == nil {
		t.Fatalf("round 2 review_feedback = nil, want the verdict")
	}
	if len(fb.Feedback) == 0 || fb.Feedback[0].Fix == "" {
		t.Errorf("round 2 feedback = %+v, want an entry carrying a fix", fb.Feedback)
	}
	if got.Degraded {
		t.Errorf("Degraded = true, want false after a later approval")
	}
}

func TestReviewLoop_ValidationFailureSkipsReviewer(t *testing.T) {
	v := newPlanValidator(newFakeCatalog("bench-press"))
	att := &recordingAttempt{plans: []*GeneratedPlan{
		planOf(sessionOf("2026-08-03", "not-in-catalog")),
		validPlan(),
	}}
	rev := &recordingReview{verdicts: []*PlanReview{approve(88)}}

	if _, err := runWithRetries(context.Background(), v, att.fn, rev.fn); err != nil {
		t.Fatalf("runWithRetries: %v", err)
	}

	// Reviewing a plan with a fabricated exercise id buys nothing: the defect
	// is already stated precisely, so gate 1 short-circuits to the generator.
	if rev.calls != 1 {
		t.Errorf("review calls = %d, want 1 (round 1 must not be reviewed)", rev.calls)
	}
	if att.calls != 2 {
		t.Errorf("generator calls = %d, want 2", att.calls)
	}
}

func TestReviewLoop_LowConfidenceRejectionDoesNotRegenerate(t *testing.T) {
	v := newPlanValidator(newFakeCatalog("bench-press"))
	att := alwaysReturns(validPlan())
	rev := &recordingReview{verdicts: []*PlanReview{reject(30, 0.2)}}

	got, err := runWithRetries(context.Background(), v, att.fn, rev.fn)
	if err != nil {
		t.Fatalf("runWithRetries: %v", err)
	}

	if att.calls != 1 {
		t.Errorf("generator calls = %d, want 1: an unsure reviewer must not spend a round", att.calls)
	}
	if !got.Degraded {
		t.Errorf("Degraded = false, want true so the verdict is not lost")
	}
}

func TestReviewLoop_ExhaustedRoundsShipDegraded(t *testing.T) {
	v := newPlanValidator(newFakeCatalog("bench-press"))
	att := alwaysReturns(validPlan())
	rev := &recordingReview{verdicts: []*PlanReview{reject(20, 0.9), reject(25, 0.9), reject(30, 0.9)}}

	got, err := runWithRetries(context.Background(), v, att.fn, rev.fn)
	if err != nil {
		t.Fatalf("runWithRetries: %v", err)
	}

	if att.calls != maxGenerationAttempts {
		t.Errorf("generator calls = %d, want %d", att.calls, maxGenerationAttempts)
	}
	if !got.Degraded {
		t.Errorf("Degraded = false, want true")
	}
	if got.Review == nil || got.Review.Approved {
		t.Errorf("Review = %+v, want the failing verdict attached", got.Review)
	}

	joined := strings.Join(got.Issues, "\n")
	if !strings.Contains(joined, "goal_fit") {
		t.Errorf("Issues = %v, want the reviewer objections recorded", got.Issues)
	}
}

func TestReviewLoop_ReviewerDownShipsPlan(t *testing.T) {
	v := newPlanValidator(newFakeCatalog("bench-press"))
	att := alwaysReturns(validPlan())
	rev := &recordingReview{errs: []error{errors.New("429 quota exceeded")}}

	got, err := runWithRetries(context.Background(), v, att.fn, rev.fn)
	if err != nil {
		t.Fatalf("runWithRetries: %v", err)
	}

	// A plan that cleared every deterministic gate must not be lost because the
	// advisory reviewer was unreachable.
	if att.calls != 1 {
		t.Errorf("generator calls = %d, want 1", att.calls)
	}
	if !got.Degraded {
		t.Errorf("Degraded = false, want true")
	}
}

func TestValidateReview(t *testing.T) {
	tests := []struct {
		name string
		give *PlanReview
		want bool // true means usable
	}{
		{name: "approved", give: approve(90), want: true},
		{name: "rejected with an actionable fix", give: reject(40, 0.9), want: true},
		{name: "nil verdict"},
		{name: "score above range", give: &PlanReview{Approved: true, Score: 140, Confidence: 0.9}},
		{name: "score below range", give: &PlanReview{Approved: true, Score: -1, Confidence: 0.9}},
		{name: "confidence above range", give: &PlanReview{Approved: true, Score: 90, Confidence: 1.4}},
		{name: "rejected with no feedback", give: &PlanReview{Score: 40, Confidence: 0.9}},
		{
			name: "rejected with a fix-less note",
			give: &PlanReview{Score: 40, Confidence: 0.9, Feedback: []ReviewNote{{Area: "goal_fit", Detail: "too few reps"}}},
		},
		{
			name: "approved but scored below threshold contradicts itself",
			give: &PlanReview{Approved: true, Score: 40, Confidence: 0.9},
		},
	}

	for _, tt := range tests {
		tt := tt

		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := validateReview(tt.give)

			if got := err == nil; got != tt.want {
				t.Errorf("usable = %t, want %t (err: %v)", got, tt.want, err)
			}
		})
	}
}
