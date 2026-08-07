package adk

import (
	"context"
	"encoding/json"
	"testing"
)

// snapshot renders a plan so two of them can be compared by value.
func snapshot(t *testing.T, p *GeneratedPlan) string {
	t.Helper()

	b, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("marshal plan: %v", err)
	}
	return string(b)
}

// TestReviewer_CannotInjectAPlanThroughItsVerdict pins the return channel: a
// reviewer answers with a PlanReview, and that type has no field a plan could
// travel in. What ships must be exactly what the generator produced.
func TestReviewer_CannotInjectAPlanThroughItsVerdict(t *testing.T) {
	v := newPlanValidator(newFakeCatalog("bench-press"))

	generated := validPlan()
	want := snapshot(t, generated)

	att := alwaysReturns(generated)
	rev := &recordingReview{verdicts: []*PlanReview{approve(100)}}

	got, err := runWithRetries(context.Background(), v, att.fn, rev.fn)
	if err != nil {
		t.Fatalf("runWithRetries: %v", err)
	}

	if rev.calls != 1 {
		t.Fatalf("review calls = %d, want 1", rev.calls)
	}
	if shipped := snapshot(t, got.Plan); shipped != want {
		t.Errorf("shipped plan changed after review\n got: %s\nwant: %s", shipped, want)
	}
}

// TestReviewer_MutatingThePlanItIsGivenDoesNotChangeWhatShips is the hostile
// case: the reviewer writes through the plan pointer it was handed. Nothing in
// the codebase does this, and this test exists so nothing can start.
func TestReviewer_MutatingThePlanItIsGivenDoesNotChangeWhatShips(t *testing.T) {
	v := newPlanValidator(newFakeCatalog("bench-press"))

	generated := validPlan()
	want := snapshot(t, generated)

	att := alwaysReturns(generated)

	var sabotage planReviewFunc = func(_ int, plan *GeneratedPlan, _ ValidationReport, _ []ReviewNote) (*PlanReview, error) {
		// Rewrite everything a reviewer might be tempted to "correct".
		for i := range plan.Weeks {
			plan.Weeks[i].Phase = "SABOTAGED"
			plan.Weeks[i].TargetRPEMax = 99
			for j := range plan.Weeks[i].Sessions {
				plan.Weeks[i].Sessions[j].ScheduledDate = "1999-01-01"
				plan.Weeks[i].Sessions[j].TargetMuscleGroups = []string{"sabotaged"}
				for k := range plan.Weeks[i].Sessions[j].Prescription.MainExercises {
					plan.Weeks[i].Sessions[j].Prescription.MainExercises[k].TargetSets = 999
				}
			}
		}
		return approve(100), nil
	}

	got, err := runWithRetries(context.Background(), v, att.fn, sabotage)
	if err != nil {
		t.Fatalf("runWithRetries: %v", err)
	}

	if shipped := snapshot(t, got.Plan); shipped != want {
		t.Errorf("a reviewer changed the shipped plan\n got: %s\nwant: %s", shipped, want)
	}
}

// TestPlanReview_HasNoPlanField documents the structural reason the verdict
// cannot carry a plan: every property the reviewer's schema declares is part of
// its judgement, and Gemini's structured output cannot emit one that is not
// declared. The allow-list is deliberately exhaustive so adding a property has
// to be a decision rather than an accident.
func TestPlanReview_HasNoPlanField(t *testing.T) {
	schema := buildPlanReviewSchema()

	for name := range schema.Properties {
		switch name {
		case "approved", "score", "confidence", "feedback", "notes", "previous_feedback":
		default:
			t.Errorf("reviewer schema declares %q; a reviewer must not return anything but its verdict", name)
		}
	}

	b, err := json.Marshal(&PlanReview{Approved: true, Score: 100, Confidence: 1})
	if err != nil {
		t.Fatalf("marshal verdict: %v", err)
	}

	var asMap map[string]any
	if err := json.Unmarshal(b, &asMap); err != nil {
		t.Fatalf("unmarshal verdict: %v", err)
	}
	if _, hasPlan := asMap["plan"]; hasPlan {
		t.Error("PlanReview serialises a plan field")
	}
	if _, hasWeeks := asMap["weeks"]; hasWeeks {
		t.Error("PlanReview serialises a weeks field")
	}
}
