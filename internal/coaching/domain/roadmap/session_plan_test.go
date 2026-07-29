package roadmap

import (
	"errors"
	"testing"
	"time"
)

func newTestSessionInfo() SessionPlanInfo {

	return SessionPlanInfo{

		SessionPlanID: "sp-1",

		DayPlanID: "dp-1",

		WeekPlanID: "wp-1",

		RoadmapID: "rm-1",

		UserID: "user-1",

		ScheduledDate: time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC),

		SlotTime: "06:00-07:00",
	}

}

func TestNewSessionPlan_DefaultsToPending(t *testing.T) {

	now := time.Date(2026, 7, 28, 12, 0, 0, 0, time.UTC)

	info := newTestSessionInfo()

	sp, err := NewSessionPlan(&info, now)

	if err != nil {

		t.Fatalf("unexpected error: %v", err)

	}

	if sp.Status() != SessionPlanStatusPending {

		t.Errorf("expected PENDING, got %s", sp.Status())

	}

	if !sp.Info().GeneratedAt.Equal(now) {

		t.Errorf("expected GeneratedAt=%v, got %v", now, sp.Info().GeneratedAt)

	}

	if sp.Info().CompletedAt != nil {

		t.Errorf("expected CompletedAt nil, got %v", sp.Info().CompletedAt)

	}

}

func TestNewSessionPlan_NilChecks(t *testing.T) {

	if _, err := NewSessionPlan(nil, time.Now()); err == nil {

		t.Errorf("expected error for nil SessionPlanInfo")

	}

	if _, err := RehydrateSessionPlan(nil); err == nil {

		t.Errorf("expected error for nil SessionPlanInfo")

	}

}

func TestNewSessionPlan_RequiresFields(t *testing.T) {

	tests := []struct {
		name string

		give func() SessionPlanInfo
	}{

		{"missing id", func() SessionPlanInfo { i := newTestSessionInfo(); i.SessionPlanID = ""; return i }},

		{"missing day_plan_id", func() SessionPlanInfo { i := newTestSessionInfo(); i.DayPlanID = ""; return i }},

		{"missing user_id", func() SessionPlanInfo { i := newTestSessionInfo(); i.UserID = ""; return i }},

		{"missing scheduled_date", func() SessionPlanInfo { i := newTestSessionInfo(); i.ScheduledDate = time.Time{}; return i }},
	}

	now := time.Now()

	for _, tt := range tests {

		tt := tt

		t.Run(tt.name, func(t *testing.T) {

			t.Parallel()

			info := tt.give()

			_, err := NewSessionPlan(&info, now)

			if !errors.Is(err, ErrInvalidRoadmap) {

				t.Errorf("expected ErrInvalidRoadmap, got %v", err)

			}

		})

	}

}

func TestSessionPlan_MarkCompleted(t *testing.T) {

	now := time.Date(2026, 7, 28, 12, 0, 0, 0, time.UTC)

	info := newTestSessionInfo()

	sp, _ := NewSessionPlan(&info, now)

	completedAt := now.Add(24 * time.Hour)

	if err := sp.MarkCompleted(85.0, 0.5, completedAt); err != nil {

		t.Fatalf("MarkCompleted: %v", err)

	}

	if sp.Status() != SessionPlanStatusCompleted {

		t.Errorf("status = %s, want COMPLETED", sp.Status())

	}

	gotInfo := sp.Info()

	if gotInfo.CompletedAt == nil || !gotInfo.CompletedAt.Equal(completedAt) {

		t.Errorf("CompletedAt not set correctly")

	}

	if gotInfo.SessionSCR == nil || *gotInfo.SessionSCR != 85.0 {

		t.Errorf("SCR not stored")

	}

	if gotInfo.SessionDeltaRPE == nil || *gotInfo.SessionDeltaRPE != 0.5 {

		t.Errorf("ΔRPE not stored")

	}

}

func TestSessionPlan_MarkCompleted_IdempotentAndBlocksSkipped(t *testing.T) {

	now := time.Now()

	info := newTestSessionInfo()

	sp, _ := NewSessionPlan(&info, now)

	_ = sp.MarkCompleted(80, 0, now)

	// Idempotent second call is no-op

	if err := sp.MarkCompleted(90, 1, now.Add(time.Hour)); err != nil {

		t.Errorf("expected no-op on second MarkCompleted, got %v", err)

	}

	if v := *sp.Info().SessionSCR; v != 80 {

		t.Errorf("SCR overwritten: %v", v)

	}

	// Cannot mark skipped after completed

	err := sp.MarkSkipped()

	if !errors.Is(err, ErrInvalidTransition) {

		t.Errorf("expected ErrInvalidTransition, got %v", err)

	}

}

func TestSessionPlan_MarkSkipped_Idempotent(t *testing.T) {

	now := time.Now()

	info := newTestSessionInfo()

	sp, _ := NewSessionPlan(&info, now)

	if err := sp.MarkSkipped(); err != nil {

		t.Fatalf("MarkSkipped: %v", err)

	}

	// Idempotent

	if err := sp.MarkSkipped(); err != nil {

		t.Errorf("expected no-op on second MarkSkipped, got %v", err)

	}

	// Cannot complete after skipped

	if err := sp.MarkCompleted(90, 0, now); !errors.Is(err, ErrInvalidTransition) {

		t.Errorf("expected ErrInvalidTransition, got %v", err)

	}

}

func TestSessionPlan_RewritePrescription_OnlyPending(t *testing.T) {

	now := time.Now()

	info := newTestSessionInfo()

	sp, _ := NewSessionPlan(&info, now)

	newPresc := WorkoutPrescription{

		MainExercises: []PrescribedExercise{

			{ExerciseID: "ex-1", ExerciseName: "Bench Press", TargetSets: 4, TargetReps: 8, TargetWeight: 60},
		},
	}

	if err := sp.RewritePrescription(newPresc, []string{"chest"}, "regen", now.Add(time.Hour)); err != nil {

		t.Fatalf("RewritePrescription: %v", err)

	}

	if len(sp.Info().Prescription.MainExercises) != 1 {

		t.Errorf("prescription not applied")

	}

	if sp.Info().Reasoning != "regen" {

		t.Errorf("reasoning not applied")

	}

	// After completion, cannot rewrite

	_ = sp.MarkCompleted(80, 0, now)

	err := sp.RewritePrescription(newPresc, nil, "again", now.Add(2*time.Hour))

	if !errors.Is(err, ErrSessionAlreadyFinal) {

		t.Errorf("expected ErrSessionAlreadyFinal, got %v", err)

	}

}

func TestSessionPlan_Info_ReturnsDeepCopy(t *testing.T) {

	now := time.Now()

	info := newTestSessionInfo()

	info.TargetMuscleGroups = []string{"chest", "triceps"}

	info.Prescription = WorkoutPrescription{

		MainExercises: []PrescribedExercise{{ExerciseID: "e1"}},
	}

	sp, _ := NewSessionPlan(&info, now)

	got := sp.Info()

	got.TargetMuscleGroups[0] = "MUTATED"

	got.Prescription.MainExercises[0].ExerciseID = "MUTATED"

	if sp.Info().TargetMuscleGroups[0] != "chest" {

		t.Errorf("Info() didn't deep-copy TargetMuscleGroups")

	}

	if sp.Info().Prescription.MainExercises[0].ExerciseID != "e1" {

		t.Errorf("Info() didn't deep-copy Prescription")

	}

}

func TestSessionPlanStatus_Valid(t *testing.T) {

	if !SessionPlanStatusPending.Valid() {

		t.Error("PENDING should be valid")

	}

	if SessionPlanStatus("").Valid() {

		t.Error("empty should be invalid")

	}

	if !SessionPlanStatusCompleted.IsFinal() {

		t.Error("COMPLETED should be final")

	}

	if SessionPlanStatusPending.IsFinal() {

		t.Error("PENDING should not be final")

	}

}
