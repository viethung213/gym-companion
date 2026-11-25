package guardrail

import (
	"context"
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/coaching/domain/roadmap"
	"github.com/viethung213/gym-companion/internal/coaching/domain/service"
	"github.com/viethung213/gym-companion/internal/coaching/domain/taxonomy"
)

// History may reference exercises that left the catalog (a re-seed changes
// IDs); only sessions that can still change are checked against it.
func TestGuardrail_CatalogCheckSkipsHistory(t *testing.T) {
	r := buildValidRoadmap(t) // weeks from Mon 2026-08-03, ex-bench everywhere
	missing := &fakeCatalogReader{validIDs: map[string]bool{}}
	afterRoadmap := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)

	if got := NewEngine(nil, missing, nil).Check(context.Background(), r, nil, afterRoadmap); got.Status != StatusApproved {
		t.Fatalf("only past sessions reference the missing exercise: status = %s, violations = %+v", got.Status, got.Violations)
	}
	assertViolation(t, NewEngine(nil, missing, nil).Check(context.Background(), r, nil, time.Time{}), "BR-AC-12")
}

// A session later today has not started, so a regenerate may rewrite it and
// the injury rules must see it.
func TestGuardrail_LaterTodayIsInScope(t *testing.T) {
	r := buildValidRoadmap(t)
	w := r.Weeks()[0]
	date := w.Info().StartDate.AddDate(0, 0, 4) // Friday, no session yet
	dp, err := roadmap.NewDayPlan(&roadmap.DayPlanInfo{
		DayPlanID: "dp-fri", WeekPlanID: w.ID(), RoadmapID: "rm-1", UserID: "user-1", ScheduledDate: date,
	})
	if err != nil {
		t.Fatal(err)
	}
	sp, err := roadmap.NewSessionPlan(&roadmap.SessionPlanInfo{
		SessionPlanID: "sp-fri", DayPlanID: dp.ID(), WeekPlanID: w.ID(), RoadmapID: "rm-1", UserID: "user-1",
		ScheduledDate: date, SlotTime: "18:00-19:00",
		Prescription: roadmap.WorkoutPrescription{MainExercises: []roadmap.PrescribedExercise{
			{ExerciseID: "ex-bench", ExerciseName: "Bench", TargetSets: 3, TargetReps: 8, TargetWeight: 60, TargetRPE: 6.5},
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

	severeChest := []service.Injury{{Region: taxonomy.JointRegionChest, Severity: taxonomy.InjurySeveritySevere}}
	morning := date.Add(10 * time.Hour) // 10:00 UTC that Friday
	got := NewEngine(nil, benchCatalog(), benchWeight).Check(context.Background(), r, severeChest, morning)
	for _, v := range got.Violations {
		if v.Code == "BR-AC-09" && v.Path == "sessions.sp-fri[0]" {
			return
		}
	}
	t.Fatalf("the 18:00 session must be injury-checked at 10:00; violations = %+v", got.Violations)
}

// A session that can no longer change must not block a rewrite of the ones
// that can. An injury lowers the RPE of a session legally (the cap applies
// while it is in scope); once that session is past it would otherwise fail
// BR-AC-10 forever, and every later adaptation or regeneration with it.
func TestGuardrail_PhaseWindowSkipsHistory(t *testing.T) {
	lowered := validRPE()
	lowered[1] = 6 // OVERLOAD week trained below its window, as an injury cap allows
	r := buildRoadmap(t, lowered, validSets())
	afterWeek2 := r.Weeks()[2].Info().StartDate

	if got := NewEngine(nil, benchCatalog(), benchWeight).Check(context.Background(), r, nil, afterWeek2); got.Status != StatusApproved {
		t.Fatalf("only a past week is outside its window: status = %s, violations = %+v", got.Status, got.Violations)
	}
	assertViolation(t, NewEngine(nil, benchCatalog(), benchWeight).Check(context.Background(), r, nil, time.Time{}), "BR-AC-10")
}

// A DELOAD week already trained cannot be rewritten, so its volume no longer
// blocks rewrites of what is left; an upcoming one still must unload.
func TestGuardrail_DeloadVolumeOnlyBlocksWhatCanChange(t *testing.T) {
	r := buildRoadmap(t, validRPE(), [4]int32{3, 3, 3, 3}) // DELOAD holds PEAK volume
	afterTheBlock := r.Weeks()[3].Info().EndDate

	got := NewEngine(nil, benchCatalog(), benchWeight).Check(context.Background(), r, nil, afterTheBlock)
	for _, v := range got.Violations {
		if v.Code == "BR-AC-11" {
			t.Fatalf("the DELOAD week is history but still blocks: %+v", got.Violations)
		}
	}
	assertViolation(t, NewEngine(nil, benchCatalog(), benchWeight).Check(context.Background(), r, nil, time.Time{}), "BR-AC-11")
}
