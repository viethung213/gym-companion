package roadmap

import (
	"errors"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

func newTestSessionInfo() SessionPlanInfo {
	return SessionPlanInfo{
		SessionPlanID: "sp-1",
		DayPlanID:     "dp-1",
		WeekPlanID:    "wp-1",
		RoadmapID:     "rm-1",
		UserID:        "user-1",
		ScheduledDate: time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC),
		SlotTime:      "06:00-07:00",
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

// Seen live: an ad-hoc session was saved as 0 minutes because nothing gave it
// a length. A session without one takes its prescription's.
func TestNewSessionPlan_LengthFollowsThePrescription(t *testing.T) {
	now := time.Date(2026, 7, 28, 12, 0, 0, 0, time.UTC)
	info := newTestSessionInfo()
	info.Prescription = WorkoutPrescription{MainExercises: []PrescribedExercise{
		{TargetSets: 3, TargetReps: 10, RestSetSec: 90, RestExerciseSec: 120},
		{TargetSets: 3, TargetReps: 10, RestSetSec: 90, RestExerciseSec: 120},
	}}

	sp, err := NewSessionPlan(&info, now)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := sp.Info().EstimatedDurationMinutes, info.Prescription.EstimatedMinutes(); got != want || got <= 0 {
		t.Errorf("length %d, want %d from the prescription", got, want)
	}

	info.EstimatedDurationMinutes = 45
	sp, err = NewSessionPlan(&info, now)
	if err != nil {
		t.Fatal(err)
	}
	if got := sp.Info().EstimatedDurationMinutes; got != 45 {
		t.Errorf("length %d, want the given 45 kept", got)
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

	// A workout reported after its week closed still completes the session.

	if err := sp.MarkCompleted(90, 0, now); err != nil {
		t.Fatalf("late completion of a skipped session: %v", err)
	}
	if sp.Status() != SessionPlanStatusCompleted || sp.Info().SessionSCR == nil {
		t.Errorf("status %s, SCR %v: want COMPLETED with metrics", sp.Status(), sp.Info().SessionSCR)
	}
}

func TestSessionPlan_MarkAborted_Idempotent(t *testing.T) {
	now := time.Now()

	info := newTestSessionInfo()

	sp, _ := NewSessionPlan(&info, now)

	if err := sp.MarkAborted(""); err != nil {
		t.Fatalf("MarkAborted: %v", err)
	}

	if sp.Status() != SessionPlanStatusAborted {
		t.Errorf("status = %s, want ABORTED", sp.Status())
	}

	// Idempotent
	if err := sp.MarkAborted(""); err != nil {
		t.Errorf("expected no-op on second MarkAborted, got %v", err)
	}

	// Cannot complete or skip after aborted
	if err := sp.MarkCompleted(90, 0, now); !errors.Is(err, ErrInvalidTransition) {
		t.Errorf("expected ErrInvalidTransition on MarkCompleted, got %v", err)
	}

	if err := sp.MarkSkipped(); !errors.Is(err, ErrInvalidTransition) {
		t.Errorf("expected ErrInvalidTransition on MarkSkipped, got %v", err)
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

	if !SessionPlanStatusAborted.Valid() {
		t.Error("ABORTED should be valid")
	}

	if SessionPlanStatus("").Valid() {
		t.Error("empty should be invalid")
	}

	if !SessionPlanStatusCompleted.IsFinal() {
		t.Error("COMPLETED should be final")
	}

	if !SessionPlanStatusAborted.IsFinal() {
		t.Error("ABORTED should be final")
	}

	if SessionPlanStatusPending.IsFinal() {
		t.Error("PENDING should not be final")
	}
}

func TestSessionPlanSource_Valid(t *testing.T) {
	tests := []struct {
		name string
		give SessionPlanSource
		want bool
	}{
		{name: "scheduled", give: SessionPlanSourceScheduled, want: true},
		{name: "adhoc", give: SessionPlanSourceAdHoc, want: true},
		{name: "solo_match", give: SessionPlanSourceSoloMatch, want: true},
		{name: "empty", give: SessionPlanSource(""), want: false},
		{name: "unknown", give: SessionPlanSource("UNKNOWN"), want: false},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := tt.give.Valid()
			if got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNewSessionPlan_SoloMatchSource(t *testing.T) {
	now := time.Date(2026, 7, 28, 12, 0, 0, 0, time.UTC)
	info := newTestSessionInfo()
	info.Source = SessionPlanSourceSoloMatch

	sp, err := NewSessionPlan(&info, now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := sp.Info().Source; got != SessionPlanSourceSoloMatch {
		t.Errorf("got source %s, want %s", got, SessionPlanSourceSoloMatch)
	}

	info.Status = SessionPlanStatusPending
	rehydrated, err := RehydrateSessionPlan(&info)
	if err != nil {
		t.Fatalf("unexpected rehydrate error: %v", err)
	}

	if got := rehydrated.Info().Source; got != SessionPlanSourceSoloMatch {
		t.Errorf("got rehydrated source %s, want %s", got, SessionPlanSourceSoloMatch)
	}
}

// A rewritten session takes the length of what it now prescribes: a session
// the adaptive cycle shortens must show its new length, and a length stored
// out of bounds by an older plan is corrected rather than kept.
func TestSessionPlan_RewritePrescription_UpdatesLength(t *testing.T) {
	now := time.Now()
	info := newTestSessionInfo()
	info.EstimatedDurationMinutes = 240 // out of bounds, stored by an older plan
	sp, err := NewSessionPlan(&info, now)
	if err != nil {
		t.Fatal(err)
	}

	short := WorkoutPrescription{MainExercises: []PrescribedExercise{
		{ExerciseID: "ex-1", TargetSets: 2, TargetReps: 8, RestSetSec: 90, RestExerciseSec: 90},
		{ExerciseID: "ex-2", TargetSets: 2, TargetReps: 8, RestSetSec: 90, RestExerciseSec: 90},
	}}
	if err := sp.RewritePrescription(short, []string{"chest"}, "express", now); err != nil {
		t.Fatal(err)
	}
	if got, want := sp.Info().EstimatedDurationMinutes, short.EstimatedMinutes(); got != want {
		t.Errorf("EstimatedDurationMinutes = %d, want %d from the new prescription", got, want)
	}
}

// An abort reported after the week was closed still lands; a completed
// session is never turned into an aborted one.
func TestSessionPlan_MarkAborted_AfterSkip(t *testing.T) {
	now := time.Now()
	info := newTestSessionInfo()
	sp, _ := NewSessionPlan(&info, now)
	if err := sp.MarkSkipped(); err != nil {
		t.Fatal(err)
	}
	if err := sp.MarkAborted(""); err != nil || sp.Status() != SessionPlanStatusAborted {
		t.Fatalf("abort after skip: err %v, status %s", err, sp.Status())
	}

	done, _ := NewSessionPlan(&info, now)
	_ = done.MarkCompleted(100, 0, now)
	if err := done.MarkAborted(""); !errors.Is(err, ErrInvalidTransition) {
		t.Errorf("abort after completion: err %v, want ErrInvalidTransition", err)
	}
}

func newPendingSession(t *testing.T) *SessionPlan {
	t.Helper()
	info := newTestSessionInfo()
	sp, err := NewSessionPlan(&info, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	return sp
}

func TestSessionPlan_StoppedForPain(t *testing.T) {
	for reason, want := range map[string]bool{"pain": true, "Pain: knee": true, "painful-music": false, "out-of-time": false, "": false} {
		sp := newPendingSession(t)
		if err := sp.MarkAborted(reason); err != nil {
			t.Fatal(err)
		}
		if got := sp.StoppedForPain(); got != want {
			t.Errorf("%q: StoppedForPain = %v, want %v", reason, got, want)
		}
	}
	if newPendingSession(t).StoppedForPain() {
		t.Error("a pending session reported a pain stop")
	}
}

// A long Vietnamese note is cut on a character boundary: a byte cut inside a
// character is invalid UTF-8 and the abort could not be saved.
func TestSessionPlan_AbortReasonKeepsValidUTF8(t *testing.T) {
	sp := newPendingSession(t)
	note := "pain: " + strings.Repeat("đau lưng dưới ", 60)
	if err := sp.MarkAborted(note); err != nil {
		t.Fatal(err)
	}
	got := sp.Info().AbortReason
	if len(got) > maxAbortReasonLen || !utf8.ValidString(got) || !strings.HasPrefix(note, got) {
		t.Errorf("reason len %d valid %v, want a valid prefix of at most %d bytes", len(got), utf8.ValidString(got), maxAbortReasonLen)
	}
	if !sp.StoppedForPain() {
		t.Error("truncated pain reason no longer reads as a pain stop")
	}
	if truncateRunes("abc", 10) != "abc" || truncateRunes("abcdef", 3) != "abc" {
		t.Error("ASCII truncation changed")
	}
}
