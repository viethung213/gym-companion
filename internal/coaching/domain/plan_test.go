package domain_test

import (
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/coaching/domain"
)

func TestCreateDailyPlan(t *testing.T) {
	t.Parallel()

	now := time.Now()
	scheduledDate := time.Date(2026, 7, 20, 0, 0, 0, 0, time.UTC)

	prescription := domain.WorkoutPrescription{
		MainExercises: []domain.PrescribedExercise{
			{
				ExerciseID:   "ex-1",
				ExerciseName: "Barbell Bench Press",
				TargetSets:   3,
				TargetReps:   10,
			},
		},
	}

	plan, err := domain.CreateDailyPlan(
		"p-1", "s-1", "r-1", "u-1",
		scheduledDate, prescription,
		"Test reasoning", now,
	)
	if err != nil {
		t.Fatalf("create daily plan failed: %v", err)
	}

	if got, want := plan.ID(), "p-1"; got != want {
		t.Errorf("ID got = %s, want = %s", got, want)
	}
	if got, want := plan.Status(), domain.DailyPlanStatusDraft; got != want {
		t.Errorf("Status got = %s, want = %s", got, want)
	}
	if got, want := len(plan.WorkoutPrescription().MainExercises), 1; got != want {
		t.Errorf("MainExercises count got = %d, want = %d", got, want)
	}

	// Test status transition Draft -> Active -> Completed
	if err := plan.Activate(now); err != nil {
		t.Fatalf("activate plan failed: %v", err)
	}
	if got, want := plan.Status(), domain.DailyPlanStatusActive; got != want {
		t.Errorf("Status after activate got = %s, want = %s", got, want)
	}

	if err := plan.Complete(now); err != nil {
		t.Fatalf("complete plan failed: %v", err)
	}
	if got, want := plan.Status(), domain.DailyPlanStatusCompleted; got != want {
		t.Errorf("Status after complete got = %s, want = %s", got, want)
	}
}
