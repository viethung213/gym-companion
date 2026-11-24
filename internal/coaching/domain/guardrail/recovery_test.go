package guardrail

import (
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/coaching/domain/roadmap"
)

// addSessions adds n sessions of kind to week 1, one per day from firstDay
// (0 = Monday), at the given main-work RPE.
func addSessions(t *testing.T, r *roadmap.Roadmap, firstDay, n int, kind roadmap.SessionKind, rpe float32) {
	t.Helper()
	w := r.Weeks()[0]
	start := w.Info().StartDate
	for i := range n {
		date := start.AddDate(0, 0, firstDay+i)
		id := string(kind) + "-" + itoa(i)
		dp, err := roadmap.NewDayPlan(&roadmap.DayPlanInfo{
			DayPlanID: "dp-" + id, WeekPlanID: w.ID(), RoadmapID: "rm-1", UserID: "user-1", ScheduledDate: date,
		})
		if err != nil {
			t.Fatal(err)
		}
		sp, err := roadmap.NewSessionPlan(&roadmap.SessionPlanInfo{
			SessionPlanID: "sp-" + id, DayPlanID: dp.ID(), WeekPlanID: w.ID(), RoadmapID: "rm-1", UserID: "user-1",
			ScheduledDate: date, Kind: kind,
			Prescription: roadmap.WorkoutPrescription{MainExercises: []roadmap.PrescribedExercise{
				{ExerciseID: "ex-bench", ExerciseName: "Bench", TargetSets: 2, TargetReps: 10, TargetWeight: 60, TargetRPE: rpe},
			}},
		}, time.Now())
		if err != nil {
			t.Fatal(err)
		}
		if err := dp.AddSession(sp); err != nil {
			t.Fatal(err)
		}
		if err := w.AddDay(dp); err != nil {
			t.Fatal(err)
		}
	}
}

func benchWeight(id string) float64 {
	if id == "ex-bench" {
		return 60
	}
	return 0
}

func TestGuardrail_RecoverySessionsDoNotCountTowardWeeklyCap(t *testing.T) {
	r := buildValidRoadmap(t)                                 // 3 training sessions a week
	addSessions(t, r, 3, 3, roadmap.SessionKindTraining, 6.5) // Thu-Sat
	addSessions(t, r, 6, 1, roadmap.SessionKindRecovery, 3)   // Sun

	got := NewEngine(nil, nil, benchWeight).Check(r, nil, time.Time{})
	if got.Status != StatusApproved {
		t.Fatalf("6 training + 1 recovery: status = %s, violations = %+v", got.Status, got.Violations)
	}
}

func TestGuardrail_RecoverySessionHasItsOwnRPECeiling(t *testing.T) {
	r := buildValidRoadmap(t)
	addSessions(t, r, 6, 1, roadmap.SessionKindRecovery, 7) // phase-valid, too hard for recovery

	assertViolation(t, NewEngine(nil, nil, benchWeight).Check(r, nil, time.Time{}), "BR-AC-10")
}
