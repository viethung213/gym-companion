package adk

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/coaching/domain/roadmap"
	"github.com/viethung213/gym-companion/internal/coaching/domain/taxonomy"
)

// recoverySchedule makes Sunday 2026-11-29 07:00 a RECOVERY slot: seven daily
// slots from Monday leave the seventh beyond the training cap.
func recoverySchedule() *slotSchedule {
	return newSlotSchedule(dailySlots(), time.Date(2026, 11, 23, 0, 0, 0, 0, time.UTC), time.UTC)
}

func recoveryCatalog() *fakeCatalog {
	c := newFakeCatalog("bench-press", "hamstring-stretch", "easy-bike")
	stretch := c.byID["hamstring-stretch"]
	stretch.Category = taxonomy.CategoryStretching
	c.byID["hamstring-stretch"] = stretch
	bike := c.byID["easy-bike"]
	bike.Category = taxonomy.CategoryCardio
	c.byID["easy-bike"] = bike
	return c
}

func sundaySession(mainIDs ...string) SessionPlan {
	s := sessionOf("2026-11-29", mainIDs...)
	s.SlotTime = "07:00-08:00"
	s.SessionKind = "" // the model may omit it; the backend pins it
	return s
}

func TestValidator_RecoverySessionTakesOnlyStretchingOrCardio(t *testing.T) {
	v := newPlanValidator(recoveryCatalog()).withSchedule(recoverySchedule())

	plan := planOf(sundaySession("hamstring-stretch", "easy-bike"))
	plan.Weeks[0].Sessions[0].Prescription.MainExercises[0].TargetRPE = 8
	out, err := v.validate(context.Background(), plan, false)
	if err != nil {
		t.Fatal(err)
	}
	for _, is := range out.Issues {
		if strings.Contains(is.Reason, "RECOVERY") {
			t.Fatalf("unexpected recovery issue: %s", is.Reason)
		}
	}
	got := out.Plan.Weeks[0].Sessions[0]
	if got.SessionKind != kindRecovery {
		t.Fatalf("kind = %q, want RECOVERY pinned from the schedule", got.SessionKind)
	}
	if rpe := got.Prescription.MainExercises[0].TargetRPE; rpe != roadmap.RecoveryMaxRPE {
		t.Errorf("recovery RPE = %v, want capped to %d", rpe, roadmap.RecoveryMaxRPE)
	}

	strength := planOf(sundaySession("bench-press"))
	out, err = v.validate(context.Background(), strength, false)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, is := range out.Issues {
		found = found || strings.Contains(is.Reason, "RECOVERY session takes every exercise from recovery_exercises")
	}
	if !found {
		t.Fatalf("issues = %+v, want a recovery-category issue for a strength exercise", out.Issues)
	}
}

func TestValidator_RecoverySessionsDoNotCountTowardTrainingCap(t *testing.T) {
	v := newPlanValidator(recoveryCatalog())
	sessions := make([]SessionPlan, 0, 8)
	for d := range 8 {
		s := sessionOf("2026-11-2"+string(rune('3'+d%7)), "hamstring-stretch")
		s.SlotTime = "0" + string(rune('6'+d/7)) + ":00-07:00"
		sessions = append(sessions, s)
	}
	plan := planOf(sessions...)
	v = v.withRevise(func() []SessionToRevise {
		out := make([]SessionToRevise, len(sessions))
		for i, s := range sessions {
			kind := kindTraining
			if i >= maxSessionsPerWeek {
				kind = kindRecovery
			}
			out[i] = SessionToRevise{ScheduledDate: s.ScheduledDate, SlotTime: s.SlotTime, SessionKind: kind}
		}
		return out
	}())

	out, err := v.validate(context.Background(), plan, false)
	if err != nil {
		t.Fatal(err)
	}
	for _, is := range out.Issues {
		if strings.Contains(is.Reason, "training sessions, exceeding") {
			t.Fatalf("6 training + 2 recovery must pass the training cap: %s", is.Reason)
		}
	}
}
