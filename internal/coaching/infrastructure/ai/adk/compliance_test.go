package adk

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// warmupNote is the fix that the live generator skipped: the hard one.
func warmupNote() ReviewNote {
	return ReviewNote{
		Area:   "warmup_specificity",
		Detail: "every back session warms up and cools down with pull-up",
		Fix:    "give each session a warm-up targeting the muscles it trains",
	}
}

func applied(area string) FeedbackOutcome {
	return FeedbackOutcome{Area: area, Applied: true, Evidence: "week 1 session 2 now uses band-pull-apart"}
}

func skipped(area string) FeedbackOutcome {
	return FeedbackOutcome{Area: area, Applied: false, Evidence: "week 1 session 2 still uses pull-up, unchanged"}
}

func TestCheckFeedbackCompliance(t *testing.T) {
	prior := []ReviewNote{warmupNote()}

	tests := []struct {
		name  string
		prior []ReviewNote
		give  *PlanReview
		want  bool // true means the verdict is usable
	}{
		{
			name: "round 1 has nothing to account for",
			give: approve(100),
			want: true,
		},
		{
			name:  "approving with the fix applied",
			prior: prior,
			give: &PlanReview{
				Approved: true, Score: 100, Confidence: 0.9,
				PreviousFeedback: []FeedbackOutcome{applied("warmup_specificity")},
			},
			want: true,
		},
		{
			// The live failure: the generator skipped a fix and round 2 approved
			// anyway, because nothing had told it what had been asked.
			name:  "approving while a fix is still unapplied",
			prior: prior,
			give: &PlanReview{
				Approved: true, Score: 100, Confidence: 0.9,
				PreviousFeedback: []FeedbackOutcome{skipped("warmup_specificity")},
			},
		},
		{
			name:  "rejecting because the fix is unapplied is fine",
			prior: prior,
			give: &PlanReview{
				Approved: false, Score: 60, Confidence: 0.9,
				PreviousFeedback: []FeedbackOutcome{skipped("warmup_specificity")},
				Feedback:         []ReviewNote{warmupNote()},
			},
			want: true,
		},
		{
			name:  "silent about what was asked",
			prior: prior,
			give:  approve(100),
		},
		{
			name:  "accounting for the wrong item",
			prior: prior,
			give: &PlanReview{
				Approved: true, Score: 100, Confidence: 0.9,
				PreviousFeedback: []FeedbackOutcome{applied("goal_fit")},
			},
		},
		{
			name:  "claiming applied without evidence",
			prior: prior,
			give: &PlanReview{
				Approved: true, Score: 100, Confidence: 0.9,
				PreviousFeedback: []FeedbackOutcome{{Area: "warmup_specificity", Applied: true}},
			},
		},
	}

	for _, tt := range tests {
		tt := tt

		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := validateReview(tt.give, tt.prior)

			if got := err == nil; got != tt.want {
				t.Errorf("usable = %t, want %t (err: %v)", got, tt.want, err)
			}
			if err != nil && !errors.Is(err, errReviewUnusable) {
				t.Errorf("err = %v, want it to wrap errReviewUnusable", err)
			}
		})
	}
}

// TestReviewLoop_CarriesLastRoundsAsksForward pins the plumbing: round 1 asks
// for something, and round 2 is handed exactly that so it can check.
func TestReviewLoop_CarriesLastRoundsAsksForward(t *testing.T) {
	v := newPlanValidator(newFakeCatalog("bench-press"))
	att := alwaysReturns(validPlan())

	round1 := &PlanReview{
		Approved: false, Score: 60, Confidence: 0.9,
		Feedback: []ReviewNote{warmupNote()},
	}
	round2 := &PlanReview{
		Approved: true, Score: 100, Confidence: 0.9,
		PreviousFeedback: []FeedbackOutcome{applied("warmup_specificity")},
	}
	rev := &recordingReview{verdicts: []*PlanReview{round1, round2}}

	if _, err := runWithRetries(context.Background(), v, att.fn, rev.fn); err != nil {
		t.Fatalf("runWithRetries: %v", err)
	}

	if rev.calls != 2 {
		t.Fatalf("review calls = %d, want 2", rev.calls)
	}
	if got := rev.seenPrior[0]; len(got) != 0 {
		t.Errorf("round 1 prior = %+v, want empty", got)
	}
	got := rev.seenPrior[1]
	if len(got) != 1 || got[0].Area != "warmup_specificity" {
		t.Errorf("round 2 prior = %+v, want round 1's single ask", got)
	}
}

// TestReviewLoop_SkippedFixIsRecordedOnTheShippedPlan covers the exhausted-rounds
// path: the plan still ships, but the objection travels with it rather than
// vanishing into an approval.
func TestReviewLoop_SkippedFixIsRecordedOnTheShippedPlan(t *testing.T) {
	v := newPlanValidator(newFakeCatalog("bench-press"))
	att := alwaysReturns(validPlan())

	stubborn := &PlanReview{
		Approved: false, Score: 60, Confidence: 0.9,
		Feedback: []ReviewNote{warmupNote()},
	}
	rev := &recordingReview{verdicts: []*PlanReview{stubborn, stubborn, stubborn}}

	got, err := runWithRetries(context.Background(), v, att.fn, rev.fn)
	if err != nil {
		t.Fatalf("runWithRetries: %v", err)
	}

	if !got.Degraded {
		t.Error("Degraded = false, want true")
	}
	if joined := strings.Join(got.Issues, "\n"); !strings.Contains(joined, "warmup_specificity") {
		t.Errorf("Issues = %v, want the unapplied ask recorded", got.Issues)
	}
}
