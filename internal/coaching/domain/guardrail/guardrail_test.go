package guardrail

import (
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/coaching/application/port"
	"github.com/viethung213/gym-companion/internal/coaching/domain/roadmap"
)

func buildValidRoadmap(t *testing.T) *roadmap.Roadmap {

	t.Helper()

	base := time.Date(2026, 8, 3, 0, 0, 0, 0, time.UTC) // Monday

	weeks := make([]*roadmap.WeekPlan, 0, 4)

	for w := 0; w < 4; w++ {

		weekStart := base.AddDate(0, 0, w*7)

		phase := []roadmap.Phase{roadmap.PhaseAccumulation, roadmap.PhaseOverload, roadmap.PhasePeak, roadmap.PhaseDeload}[w]

		wp, err := roadmap.NewWeekPlan(&roadmap.WeekPlanInfo{

			WeekPlanID: "wp-" + itoa(w+1),

			RoadmapID: "rm-1",

			UserID: "user-1",

			WeekNumber: int32(w + 1),

			Phase: phase,

			TargetRPE: 7.0,

			StartDate: weekStart,

			EndDate: weekStart.AddDate(0, 0, 6),
		})

		if err != nil {

			t.Fatalf("wp: %v", err)

		}

		for d := 0; d < 3; d++ {

			dp, _ := roadmap.NewDayPlan(&roadmap.DayPlanInfo{

				DayPlanID: "dp-" + itoa(w) + "-" + itoa(d),

				WeekPlanID: wp.ID(),

				RoadmapID: "rm-1",

				UserID: "user-1",

				ScheduledDate: weekStart.AddDate(0, 0, d),
			})

			sp, _ := roadmap.NewSessionPlan(&roadmap.SessionPlanInfo{

				SessionPlanID: "sp-" + itoa(w) + "-" + itoa(d),

				DayPlanID: dp.ID(),

				WeekPlanID: wp.ID(),

				RoadmapID: "rm-1",

				UserID: "user-1",

				ScheduledDate: weekStart.AddDate(0, 0, d),

				TargetMuscleGroups: []string{"chest"},

				Prescription: roadmap.WorkoutPrescription{

					MainExercises: []roadmap.PrescribedExercise{

						{ExerciseID: "ex-bench", ExerciseName: "Bench", TargetSets: 3, TargetReps: 8, TargetWeight: 60, TargetRPE: 7},
					},
				},
			}, time.Now())

			_ = dp.AddSession(sp)

			_ = wp.AddDay(dp)

		}

		weeks = append(weeks, wp)

	}

	r, err := roadmap.NewRoadmap(&roadmap.Info{

		RoadmapID: "rm-1",

		UserID: "user-1",

		StartDate: base,

		EndDate: base.AddDate(0, 0, 28),
	}, weeks, time.Now())

	if err != nil {

		t.Fatalf("roadmap: %v", err)

	}

	return r

}

func TestGuardrail_Approved_Baseline(t *testing.T) {

	r := buildValidRoadmap(t)

	e := NewEngine(nil, func(id string) float64 {

		if id == "ex-bench" {

			return 60.0

		}

		return 0

	}, nil)

	got := e.Check(r)

	if got.Status != StatusApproved {

		t.Errorf("status = %s, violations = %+v", got.Status, got.Violations)

	}

}

func TestGuardrail_RejectsWeightOverBand(t *testing.T) {

	r := buildValidRoadmap(t)

	// Baseline PR 40 → 60kg exceeds +30% (52kg cap).

	e := NewEngine(nil, func(id string) float64 {

		if id == "ex-bench" {

			return 40.0

		}

		return 0

	}, nil)

	got := e.Check(r)

	if got.Status != StatusRejected {

		t.Fatalf("expected rejected")

	}

	foundBRAC02 := false

	for _, v := range got.Violations {

		if v.Code == "BR-AC-02" {

			foundBRAC02 = true

			break

		}

	}

	if !foundBRAC02 {

		t.Errorf("expected BR-AC-02 violation, got: %+v", got.Violations)

	}

}

func TestGuardrail_RejectsInjuryTargeted(t *testing.T) {

	r := buildValidRoadmap(t)

	// All sessions target "chest" — injury on chest should reject them all.

	e := NewEngine(nil, nil, []port.InjuryStatus{

		{MuscleGroup: "chest"},
	})

	got := e.Check(r)

	if got.Status != StatusRejected {

		t.Fatalf("expected rejected")

	}

	foundBRAC09 := false

	for _, v := range got.Violations {

		if v.Code == "BR-AC-09" {

			foundBRAC09 = true

			break

		}

	}

	if !foundBRAC09 {

		t.Errorf("expected BR-AC-09 violation")

	}

}

func TestGuardrail_RecoveredInjury_NotFlagged(t *testing.T) {

	r := buildValidRoadmap(t)

	rec := time.Now()

	e := NewEngine(nil, nil, []port.InjuryStatus{

		{MuscleGroup: "chest", RecoveredAt: &rec},
	})

	got := e.Check(r)

	if got.Status != StatusApproved {

		t.Errorf("expected approved for recovered injury, got %s: %+v", got.Status, got.Violations)

	}

}

func TestGuardrail_NilRoadmapRejected(t *testing.T) {

	e := NewEngine(nil, nil, nil)

	got := e.Check(nil)

	if got.Status != StatusRejected {

		t.Errorf("nil roadmap should reject")

	}

}
