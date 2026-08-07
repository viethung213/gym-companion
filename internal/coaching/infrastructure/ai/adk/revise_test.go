package adk

import (
	"context"
	"strings"
	"testing"

	"github.com/viethung213/gym-companion/internal/coaching/domain/roadmap"
)

func TestPlanValidator_Expecting_RejectsWrongSessionCount(t *testing.T) {
	v := newPlanValidator(newFakeCatalog("bench-press")).expecting(3)

	got, err := v.validate(context.Background(), planOf(
		sessionOf("2026-08-03", "bench-press"),
		sessionOf("2026-08-05", "bench-press"),
	), false)
	if err != nil {
		t.Fatalf("validate returned error: %v", err)
	}

	if len(got.Issues) != 1 {
		t.Fatalf("got %d issues, want 1: %v", len(got.Issues), got.Issues)
	}
	if !strings.Contains(got.Issues[0].Reason, "same order") {
		t.Errorf("got %q, want the issue to state the ordering contract", got.Issues[0].Reason)
	}
}

func TestPlanValidator_Expecting_AcceptsExactCount(t *testing.T) {
	v := newPlanValidator(newFakeCatalog("bench-press")).expecting(2)

	got, err := v.validate(context.Background(), planOf(
		sessionOf("2026-08-03", "bench-press"),
		sessionOf("2026-08-05", "bench-press"),
	), false)
	if err != nil {
		t.Fatalf("validate returned error: %v", err)
	}

	if len(got.Issues) != 0 {
		t.Errorf("got %d issues, want 0: %v", len(got.Issues), got.Issues)
	}
}

// A miscount must be retried rather than salvaged: dropping a session would
// shift every later one onto the wrong day.
func TestRunWithRetries_MiscountIsRetried(t *testing.T) {
	v := newPlanValidator(newFakeCatalog("bench-press")).expecting(2)
	att := &recordingAttempt{plans: []*GeneratedPlan{
		planOf(sessionOf("2026-08-03", "bench-press")),
		planOf(sessionOf("2026-08-03", "bench-press"), sessionOf("2026-08-05", "bench-press")),
	}}

	got, err := runWithRetries(context.Background(), v, att.fn, nil)
	if err != nil {
		t.Fatalf("runWithRetries returned error: %v", err)
	}

	if att.calls != 2 {
		t.Errorf("got %d attempts, want 2", att.calls)
	}
	if n := planSessionCount(got.Plan); n != 2 {
		t.Errorf("got %d sessions, want 2", n)
	}
	if !strings.Contains(strings.Join(att.seenIssues[1], "\n"), "same order") {
		t.Errorf("attempt 2 was not told about the miscount: %v", att.seenIssues[1])
	}
}

func TestPrescriptionToDTO_RoundTripsThroughDomain(t *testing.T) {
	c := mapperFor(t, "bench-press")
	domainPresc := roadmap.WorkoutPrescription{
		MainExercises: []roadmap.PrescribedExercise{{
			ExerciseID:   "bench-press",
			ExerciseName: "Bench Press",
			TargetSets:   4,
			TargetReps:   6,
			TargetWeight: 82.5,
			TargetRPE:    8,
			RestSetSec:   150,
		}},
	}

	dto := prescriptionToDTO(domainPresc)
	if len(dto.MainExercises) != 1 {
		t.Fatalf("got %d main exercises, want 1", len(dto.MainExercises))
	}

	got := dto.MainExercises[0]
	if got.ExerciseID != "bench-press" || got.TargetSets != 4 || got.TargetReps != 6 {
		t.Errorf("got %+v, want the stored values preserved", got)
	}
	if got.TargetWeightKg != 82.5 || got.TargetRPE != 8 || got.RestSetSec != 150 {
		t.Errorf("got %+v, want weight/rpe/rest preserved", got)
	}

	// Back to the domain, the numbers must survive unchanged.
	back := c.mapExercisesToDomain(context.Background(), dto.MainExercises, map[string]string{
		"bench-press": "Bench Press",
	})
	if back[0].TargetWeight != 82.5 || back[0].TargetSets != 4 {
		t.Errorf("got %+v after round trip, want the original numbers", back[0])
	}
}

func TestPrescriptionToDTO_EmptySlotsStayNil(t *testing.T) {
	dto := prescriptionToDTO(roadmap.WorkoutPrescription{})

	if dto.WarmUps != nil || dto.MainExercises != nil || dto.CoolDowns != nil {
		t.Errorf("got %+v, want nil slices for empty slots", dto)
	}
}

func TestCurrentPhase_FindsOwningWeek(t *testing.T) {
	c := mapperFor(t, "bench-press")
	plan := planOf(sessionOf("2026-08-03", "bench-press"))
	plan.Weeks[0].Phase = "OVERLOAD"

	rm, err := c.mapToDomainRoadmap(context.Background(), plan, nil, "user-1", getMapNow())
	if err != nil {
		t.Fatalf("mapToDomainRoadmap returned error: %v", err)
	}

	session := rm.Weeks()[0].Days()[0].Sessions()[0]
	if got := currentPhase(rm, session); got != "OVERLOAD" {
		t.Errorf("got phase %q, want OVERLOAD", got)
	}
}

func TestCurrentPhase_UnknownWeekIsEmpty(t *testing.T) {
	c := mapperFor(t, "bench-press")
	rm, err := c.mapToDomainRoadmap(context.Background(),
		planOf(sessionOf("2026-08-03", "bench-press")), nil, "user-1", getMapNow())
	if err != nil {
		t.Fatalf("mapToDomainRoadmap returned error: %v", err)
	}

	orphan, err := roadmap.NewSessionPlan(&roadmap.SessionPlanInfo{
		SessionPlanID: "orphan",
		DayPlanID:     "d",
		WeekPlanID:    "not-in-this-roadmap",
		RoadmapID:     rm.ID(),
		UserID:        "user-1",
		ScheduledDate: getMapNow(),
	}, getMapNow())
	if err != nil {
		t.Fatalf("new session plan: %v", err)
	}

	if got := currentPhase(rm, orphan); got != "" {
		t.Errorf("got phase %q, want empty for a week not in the roadmap", got)
	}
}

func TestReasonHandoff_TakeDeletes(t *testing.T) {
	c := &CoachingContextAgent{reasons: make(map[string]string)}

	c.putReason("sess-1", "SCR below target for 3 sessions")

	if got := c.takeReason("sess-1"); got != "SCR below target for 3 sessions" {
		t.Errorf("got reason %q, want the stored one", got)
	}
	if got := c.takeReason("sess-1"); got != "" {
		t.Errorf("got reason %q on second take, want empty: the slot must not leak", got)
	}
	if len(c.reasons) != 0 {
		t.Errorf("got %d entries left, want 0", len(c.reasons))
	}
}

func TestReasonHandoff_UnknownSessionIsEmpty(t *testing.T) {
	c := &CoachingContextAgent{reasons: make(map[string]string)}

	if got := c.takeReason("never-stored"); got != "" {
		t.Errorf("got %q, want empty", got)
	}
}

func TestMapToRegeneratedSessions_PreservesInputOrder(t *testing.T) {
	c := mapperFor(t, "bench-press", "squat")
	plan := planOf(
		sessionOf("2026-08-03", "bench-press"),
		sessionOf("2026-08-05", "squat"),
		sessionOf("2026-08-07", "bench-press"),
	)

	infos := c.mapToRegeneratedSessions(context.Background(), plan, nil, "user-1", getMapNow())
	if len(infos) != 3 {
		t.Fatalf("got %d infos, want 3", len(infos))
	}

	// Order carries the identity here, so it must match the plan exactly.
	wantDates := []string{"2026-08-03", "2026-08-05", "2026-08-07"}
	for i, want := range wantDates {
		if got := infos[i].ScheduledDate.Format(scheduledDateISO); got != want {
			t.Errorf("position %d: got %s, want %s", i, got, want)
		}
	}
}
