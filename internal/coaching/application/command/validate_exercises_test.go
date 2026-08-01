package command

import (
	"strings"
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/coaching/domain/roadmap"
)

// roadmapWithPrescription builds a minimal one-week roadmap holding a single
// session with the given main exercises.
func roadmapWithPrescription(t *testing.T, exercises ...roadmap.PrescribedExercise) *roadmap.Roadmap {
	t.Helper()

	now := time.Date(2026, 8, 3, 0, 0, 0, 0, time.UTC)

	wp, err := roadmap.NewWeekPlan(&roadmap.WeekPlanInfo{
		WeekPlanID: "week-1",
		RoadmapID:  "roadmap-1",
		UserID:     "user-1",
		WeekNumber: 1,
		Phase:      roadmap.PhaseAccumulation,
		TargetRPE:  6.5,
		StartDate:  now,
		EndDate:    now.AddDate(0, 0, 7),
	})
	if err != nil {
		t.Fatalf("new week plan: %v", err)
	}

	dp, err := roadmap.NewDayPlan(&roadmap.DayPlanInfo{
		DayPlanID:     "day-1",
		WeekPlanID:    "week-1",
		RoadmapID:     "roadmap-1",
		UserID:        "user-1",
		ScheduledDate: now,
	})
	if err != nil {
		t.Fatalf("new day plan: %v", err)
	}

	sp, err := roadmap.NewSessionPlan(&roadmap.SessionPlanInfo{
		SessionPlanID:      "session-1",
		DayPlanID:          "day-1",
		WeekPlanID:         "week-1",
		RoadmapID:          "roadmap-1",
		UserID:             "user-1",
		ScheduledDate:      now,
		TargetMuscleGroups: []string{"chest"},
		Prescription:       roadmap.WorkoutPrescription{MainExercises: exercises},
	}, now)
	if err != nil {
		t.Fatalf("new session plan: %v", err)
	}

	if err := dp.AddSession(sp); err != nil {
		t.Fatalf("add session: %v", err)
	}
	if err := wp.AddDay(dp); err != nil {
		t.Fatalf("add day: %v", err)
	}

	rm, err := roadmap.NewRoadmap(&roadmap.Info{
		RoadmapID: "roadmap-1",
		UserID:    "user-1",
		Status:    roadmap.StatusActive,
		StartDate: now,
		EndDate:   now.AddDate(0, 0, 28),
		CreatedAt: now,
		UpdatedAt: now,
	}, []*roadmap.WeekPlan{wp}, now)
	if err != nil {
		t.Fatalf("new roadmap: %v", err)
	}
	return rm
}

func TestValidateExercisesResolved_AcceptsFullyResolved(t *testing.T) {
	rm := roadmapWithPrescription(t, roadmap.PrescribedExercise{
		ExerciseID:   "bench-press",
		ExerciseName: "Bench Press",
		TargetSets:   3,
		TargetReps:   8,
	})

	if err := validateExercisesResolved(rm); err != nil {
		t.Errorf("validateExercisesResolved returned error: %v", err)
	}
}

// A blank name means the plan skipped catalog resolution, so the exercise was
// never verified to exist.
func TestValidateExercisesResolved_RejectsUnresolvedName(t *testing.T) {
	rm := roadmapWithPrescription(t, roadmap.PrescribedExercise{
		ExerciseID: "bench-press",
		TargetSets: 3,
		TargetReps: 8,
	})

	err := validateExercisesResolved(rm)
	if err == nil {
		t.Fatal("got nil error, want a rejection for the unresolved name")
	}
	if !strings.Contains(err.Error(), "bench-press") {
		t.Errorf("got error %q, want it to name the offending exercise", err)
	}
}

func TestValidateExercisesResolved_RejectsMissingID(t *testing.T) {
	rm := roadmapWithPrescription(t, roadmap.PrescribedExercise{
		ExerciseName: "Bench Press",
		TargetSets:   3,
	})

	err := validateExercisesResolved(rm)
	if err == nil {
		t.Fatal("got nil error, want a rejection for the missing exercise_id")
	}
	if !strings.Contains(err.Error(), "exercise_id") {
		t.Errorf("got error %q, want it to name exercise_id", err)
	}
}

func TestValidateExercisesResolved_ChecksWarmUpsAndCoolDowns(t *testing.T) {
	rm := roadmapWithPrescription(t, roadmap.PrescribedExercise{
		ExerciseID:   "bench-press",
		ExerciseName: "Bench Press",
	})

	// Reach past the constructor to plant an unresolved warm-up, mimicking a
	// mapper that filled main exercises but skipped the other slots.
	sess := rm.Weeks()[0].Days()[0].Sessions()[0]
	presc := sess.Info().Prescription
	presc.WarmUps = []roadmap.PrescribedExercise{{ExerciseID: "push-up"}}
	if err := sess.RewritePrescription(presc, []string{"chest"}, "test", time.Now()); err != nil {
		t.Fatalf("rewrite prescription: %v", err)
	}

	err := validateExercisesResolved(rm)
	if err == nil {
		t.Fatal("got nil error, want the unresolved warm-up to be caught")
	}
	if !strings.Contains(err.Error(), "push-up") {
		t.Errorf("got error %q, want it to name the warm-up exercise", err)
	}
}
