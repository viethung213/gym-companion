package domain

import (
	"testing"
	"time"
)

func TestDailyWorkoutPlan(t *testing.T) {
	now := time.Now()
	
	presc, err := NewWorkoutPrescription(3, 10, 50, 7, 90)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	pe, err := NewPlannedExercise("ex-1", presc, "note")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	dp, err := NewDailyWorkoutPlan("dp-1", "ws-1", "user-1", []PlannedExercise{pe}, now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	
	if dp.Status() != PlanStatusDraft {
		t.Errorf("expected DRAFT")
	}

	if err := dp.AddExercise(pe); err != nil {
		t.Fatalf("unexpected error adding exercise: %v", err)
	}

	if err := dp.Activate(now); err != nil {
		t.Fatalf("unexpected error activating plan: %v", err)
	}

	if err := dp.Complete(now); err != nil {
		t.Fatalf("unexpected error completing plan: %v", err)
	}

	if err := dp.AddExercise(pe); err == nil {
		t.Errorf("expected error adding exercise to completed plan")
	}

	// Cancel
	dp2, _ := NewDailyWorkoutPlan("dp-2", "ws-1", "user-1", []PlannedExercise{pe}, now)
	if err := dp2.Cancel(now); err != nil {
		t.Errorf("unexpected error cancelling plan")
	}
}

