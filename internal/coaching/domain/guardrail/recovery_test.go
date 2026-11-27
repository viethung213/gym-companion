package guardrail

import (
	"context"
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

func benchWeight(context.Context, string) map[string]float64 {
	return map[string]float64{"ex-bench": 60}
}

func TestGuardrail_RecoverySessionsDoNotCountTowardWeeklyCap(t *testing.T) {
	r := buildValidRoadmap(t)                                 // 3 training sessions a week
	addSessions(t, r, 3, 3, roadmap.SessionKindTraining, 6.5) // Thu-Sat
	addSessions(t, r, 6, 1, roadmap.SessionKindRecovery, 3)   // Sun

	got := NewEngine(nil, nil, benchWeight).Check(context.Background(), r, nil, time.Time{})
	if got.Status != StatusApproved {
		t.Fatalf("6 training + 1 recovery: status = %s, violations = %+v", got.Status, got.Violations)
	}
}

func TestGuardrail_RecoverySessionHasItsOwnRPECeiling(t *testing.T) {
	r := buildValidRoadmap(t)
	addSessions(t, r, 6, 1, roadmap.SessionKindRecovery, 7) // phase-valid, too hard for recovery

	assertViolation(t, NewEngine(nil, nil, benchWeight).Check(context.Background(), r, nil, time.Time{}), "BR-AC-10")
}

// Reproduces the review finding: the athlete's own ad-hoc session in a week
// of 6 scheduled ones counted toward BR-AC-01, so every later rewrite of the
// roadmap (week review adaptation, regenerate) was refused. The cap bounds what
// the coach schedules; extra workouts the athlete adds are theirs.
func TestGuardrail_AdHocSessionsDoNotCountTowardWeeklyCap(t *testing.T) {
	r := buildValidRoadmap(t)                                 // 3 training sessions a week
	addSessions(t, r, 3, 3, roadmap.SessionKindTraining, 6.5) // Thu-Sat: 6 scheduled
	w := r.Weeks()[0]
	date := w.Info().StartDate.AddDate(0, 0, 6)
	dp, err := roadmap.NewDayPlan(&roadmap.DayPlanInfo{
		DayPlanID: "dp-adhoc", WeekPlanID: w.ID(), RoadmapID: "rm-1", UserID: "user-1", ScheduledDate: date,
	})
	if err != nil {
		t.Fatal(err)
	}
	sp, err := roadmap.NewSessionPlan(&roadmap.SessionPlanInfo{
		SessionPlanID: "sp-adhoc", DayPlanID: dp.ID(), WeekPlanID: w.ID(), RoadmapID: "rm-1", UserID: "user-1",
		ScheduledDate: date, Source: roadmap.SessionPlanSourceAdHoc,
		Prescription: roadmap.WorkoutPrescription{MainExercises: []roadmap.PrescribedExercise{
			{ExerciseID: "ex-bench", ExerciseName: "Bench", TargetSets: 2, TargetReps: 10, TargetWeight: 60, TargetRPE: 6.5},
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

	got := NewEngine(nil, nil, benchWeight).Check(context.Background(), r, nil, time.Time{})
	if got.Status != StatusApproved {
		t.Fatalf("6 scheduled + 1 ad-hoc: status = %s, violations = %+v", got.Status, got.Violations)
	}
}
