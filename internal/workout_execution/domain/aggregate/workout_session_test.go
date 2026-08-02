package aggregate_test

import (
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/workout_execution/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/workout_execution/domain/derror"
	"github.com/viethung213/gym-companion/internal/workout_execution/domain/event"
	"github.com/viethung213/gym-companion/internal/workout_execution/domain/vo"
)

func TestWorkoutSession_Lifecycle(t *testing.T) {
	t.Run("Create new workout session", func(t *testing.T) {
		session, err := aggregate.NewWorkoutSession("sess-1", "user-1", "plan-1")
		if err != nil {
			t.Fatalf("got err = %v, want nil", err)
		}

		if got, want := session.ID(), "sess-1"; got != want {
			t.Errorf("got ID = %v, want %v", got, want)
		}
		if got, want := session.Status(), aggregate.StatusInProgress; got != want {
			t.Errorf("got Status = %v, want %v", got, want)
		}

		events := session.PopEvents()
		if got, want := len(events), 1; got != want {
			t.Errorf("got event count = %v, want %v", got, want)
		}
	})

	t.Run("Log set and calculate summary", func(t *testing.T) {
		session, _ := aggregate.NewWorkoutSession("sess-1", "user-1", "plan-1")
		formScore := float32(85.0)

		err := session.LogSet(aggregate.WorkoutSetLog{
			SetNumber:  1,
			ExerciseID: "ex-1",
			TargetReps: 10,
			ActualReps: 10,
			Weight:     50.0,
			FormScore:  &formScore,
			RPE:        8.0,
		})
		if err != nil {
			t.Fatalf("got err = %v, want nil", err)
		}

		summary := session.CalculateSummary()
		if got, want := summary.TotalSets, 1; got != want {
			t.Errorf("got TotalSets = %v, want %v", got, want)
		}
		if got, want := summary.TotalVolume, float32(500.0); got != want {
			t.Errorf("got TotalVolume = %v, want %v", got, want)
		}
	})

	t.Run("Complete session without overload confirmation required", func(t *testing.T) {
		session, _ := aggregate.NewWorkoutSession("sess-1", "user-1", "plan-1")
		session.PopEvents() // clear start event

		err := session.Complete(false, false)
		if err != nil {
			t.Fatalf("got err = %v, want nil", err)
		}

		if got, want := session.Status(), aggregate.StatusCompleted; got != want {
			t.Errorf("got Status = %v, want %v", got, want)
		}

		events := session.PopEvents()
		if got, want := len(events), 1; got != want {
			t.Fatalf("got event count = %v, want %v", got, want)
		}
		if ev, ok := events[0].(*event.WorkoutSessionCompleted); ok {
			if got, want := ev.PlanID, "plan-1"; got != want {
				t.Errorf("got PlanID = %v, want %v", got, want)
			}
		} else {
			t.Fatalf("expected *event.WorkoutSessionCompleted, got %T", events[0])
		}
	})

	t.Run("Complete session with overload requiring confirmation", func(t *testing.T) {
		session, _ := aggregate.NewWorkoutSession("sess-1", "user-1", "plan-1")

		err := session.Complete(false, true)
		if got, want := err, derror.ErrOverloadConfirmationRequired; got != want {
			t.Errorf("got err = %v, want %v", got, want)
		}

		err = session.Complete(true, true)
		if err != nil {
			t.Fatalf("got err = %v, want nil", err)
		}

		if got, want := session.Status(), aggregate.StatusCompleted; got != want {
			t.Errorf("got Status = %v, want %v", got, want)
		}
	})

	t.Run("Timeout past 240 minutes auto-aborts session", func(t *testing.T) {
		session, _ := aggregate.NewWorkoutSession("sess-1", "user-1", "plan-1")

		future := time.Now().UTC().Add(245 * time.Minute)
		isAborted := session.CheckTimeoutAndAutoAbort(future)

		if got, want := isAborted, true; got != want {
			t.Errorf("got isAborted = %v, want %v", got, want)
		}
		if got, want := session.Status(), aggregate.StatusAnomalous; got != want {
			t.Errorf("got Status = %v, want %v", got, want)
		}
	})
}

func TestCalculate1RMEpley(t *testing.T) {
	tests := []struct {
		name   string
		weight float32
		reps   int
		want   float32
	}{
		{
			name:   "1 Rep Max",
			weight: 100.0,
			reps:   1,
			want:   100.0,
		},
		{
			name:   "10 Reps at 100kg",
			weight: 100.0,
			reps:   10,
			want:   133.33333,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := aggregate.Calculate1RMEpley(tt.weight, tt.reps)
			diff := got - tt.want
			if diff < -0.1 || diff > 0.1 {
				t.Errorf("Calculate1RMEpley(%v, %v) got %v, want %v", tt.weight, tt.reps, got, tt.want)
			}
		})
	}
}

func TestWorkoutSession_MarkCriticalInactivity(t *testing.T) {
	now := time.Now().UTC()
	lastCriticalAt := now.Add(-6 * time.Minute)

	t.Run("transitions IN_PROGRESS to ANOMALOUS and emits event", func(t *testing.T) {
		session, _ := aggregate.NewWorkoutSession("sess-ci-1", "user-1", "plan-1")
		session.PopEvents() // clear start event

		err := session.MarkCriticalInactivity(now, lastCriticalAt)
		if err != nil {
			t.Fatalf("got err = %v, want nil", err)
		}
		if got, want := session.Status(), aggregate.StatusAnomalous; got != want {
			t.Errorf("got Status = %v, want %v", got, want)
		}

		events := session.PopEvents()
		if got, want := len(events), 1; got != want {
			t.Fatalf("got event count = %v, want %v", got, want)
		}
	})

	t.Run("returns error when session is not IN_PROGRESS", func(t *testing.T) {
		session, _ := aggregate.NewWorkoutSession("sess-ci-2", "user-1", "plan-1")
		_ = session.Complete(false, false)

		err := session.MarkCriticalInactivity(now, lastCriticalAt)
		if err == nil {
			t.Fatal("got nil, want error")
		}
		if got, want := session.Status(), aggregate.StatusCompleted; got != want {
			t.Errorf("got Status = %v, want %v (should not change)", got, want)
		}
	})

	t.Run("returns error when session is already ANOMALOUS", func(t *testing.T) {
		session, _ := aggregate.NewWorkoutSession("sess-ci-3", "user-1", "plan-1")
		_ = session.AbortAnomalous("previous reason")

		err := session.MarkCriticalInactivity(now, lastCriticalAt)
		if err == nil {
			t.Fatal("got nil, want error for already-anomalous session")
		}
	})
}

func TestWorkoutSession_DefensiveCopy(t *testing.T) {
	session, err := aggregate.NewWorkoutSession("sess-def-1", "user-1", "plan-1")
	if err != nil {
		t.Fatalf("got err = %v, want nil", err)
	}

	formScore := float32(90.0)
	rep := vo.NewRepLog(1, 100.0, []string{"ERR_BAD_FORM"}, map[string]float32{"knee": 45.0})

	set := aggregate.WorkoutSetLog{
		SetNumber:  1,
		ExerciseID: "ex-1",
		TargetReps: 10,
		ActualReps: 10,
		Weight:     60.0,
		FormScore:  &formScore,
		RPE:        8.0,
		Reps:       []vo.RepLog{rep},
	}

	err = session.LogSet(set)
	if err != nil {
		t.Fatalf("LogSet err = %v, want nil", err)
	}

	// 1. Mutate input set after LogSet
	formScore = 10.0
	set.Reps[0].GetErrorCodes()[0] = "MUTATED"
	sessionSets := session.Sets()
	if got := *sessionSets[0].FormScore; got != float32(90.0) {
		t.Errorf("FormScore in aggregate modified via input pointer! got %v, want 90.0", got)
	}

	// 2. Mutate output set returned by Sets()
	*sessionSets[0].FormScore = 50.0
	if got := *session.Sets()[0].FormScore; got != float32(90.0) {
		t.Errorf("FormScore in aggregate modified via Sets() getter! got %v, want 90.0", got)
	}

	// 3. Mutate StartedAt pointer returned by getter
	st := session.StartedAt()
	if st != nil {
		*st = time.Time{}
		if session.StartedAt().IsZero() {
			t.Errorf("StartedAt in aggregate modified via StartedAt() getter!")
		}
	}
}

func TestWorkoutSession_NewScheduledAndAbort(t *testing.T) {
	t.Run("NewScheduledWorkoutSession validation and state", func(t *testing.T) {
		_, err := aggregate.NewScheduledWorkoutSession("", "u1", "p1", time.Now())
		if err == nil {
			t.Error("want error for empty ID")
		}

		scheduledAt := time.Now().UTC()
		sess, err := aggregate.NewScheduledWorkoutSession("s-sched-1", "u1", "p1", scheduledAt)
		if err != nil {
			t.Fatalf("got err = %v, want nil", err)
		}
		if sess.Status() != aggregate.StatusScheduled {
			t.Errorf("got status = %v, want SCHEDULED", sess.Status())
		}
		if sess.ScheduledAt() == nil || !sess.ScheduledAt().Equal(scheduledAt) {
			t.Errorf("got scheduledAt = %v, want %v", sess.ScheduledAt(), scheduledAt)
		}

		err = sess.Start()
		if err != nil {
			t.Fatalf("Start() err = %v, want nil", err)
		}
		if sess.Status() != aggregate.StatusInProgress {
			t.Errorf("got status = %v, want IN_PROGRESS", sess.Status())
		}
	})

	t.Run("Abort and AbortAnomalous", func(t *testing.T) {
		sess, _ := aggregate.NewWorkoutSession("s-abort-1", "u1", "p1")
		sess.PopEvents() // clear start

		err := sess.Abort("user cancelled")
		if err != nil {
			t.Fatalf("Abort err = %v, want nil", err)
		}
		if sess.Status() != aggregate.StatusAborted {
			t.Errorf("got status = %v, want ABORTED", sess.Status())
		}
		events := sess.PopEvents()
		if len(events) != 1 {
			t.Fatalf("got %d events, want 1", len(events))
		}

		// Double abort should return error
		err = sess.Abort("again")
		if err == nil {
			t.Error("want error on double abort")
		}
	})

	t.Run("RecordBodyMetricUpdate", func(t *testing.T) {
		sess, _ := aggregate.NewWorkoutSession("s-bm-1", "u1", "p1")
		sess.PopEvents()

		sess.RecordBodyMetricUpdate(75.5)
		events := sess.PopEvents()
		if len(events) != 1 {
			t.Fatalf("got %d events, want 1 for RecordBodyMetricUpdate", len(events))
		}
	})
}

func TestWorkoutSession_NewScheduledWorkoutSession_Validation(t *testing.T) {
	tests := []struct {
		name    string
		id      string
		userID  string
		planID  string
		wantErr bool
	}{
		{
			name:    "empty id returns error",
			id:      "",
			userID:  "u1",
			planID:  "p1",
			wantErr: true,
		},
		{
			name:    "empty userID returns error",
			id:      "s1",
			userID:  "",
			planID:  "p1",
			wantErr: true,
		},
		{
			name:    "empty planID returns error",
			id:      "s1",
			userID:  "u1",
			planID:  "",
			wantErr: true,
		},
		{
			name:    "valid params creates scheduled session",
			id:      "s1",
			userID:  "u1",
			planID:  "p1",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			schedTime := time.Now().UTC()
			sess, err := aggregate.NewScheduledWorkoutSession(tt.id, tt.userID, tt.planID, schedTime)
			if (err != nil) != tt.wantErr {
				t.Fatalf("NewScheduledWorkoutSession() err = %v, wantErr = %v", err, tt.wantErr)
			}
			if !tt.wantErr {
				if sess.Status() != aggregate.StatusScheduled {
					t.Errorf("got status = %v, want SCHEDULED", sess.Status())
				}
				if sess.StartedAt() != nil {
					t.Errorf("got startedAt = %v, want nil for scheduled session", sess.StartedAt())
				}
				if sess.ScheduledAt() == nil || !sess.ScheduledAt().Equal(schedTime) {
					t.Errorf("got scheduledAt = %v, want %v", sess.ScheduledAt(), schedTime)
				}
			}
		})
	}
}

func TestWorkoutSession_AbortAndAbortAnomalous_Detailed(t *testing.T) {
	t.Run("Abort emits non-anomalous event", func(t *testing.T) {
		sess, _ := aggregate.NewWorkoutSession("s-ab-1", "u1", "p1")
		sess.PopEvents() // clear start event

		err := sess.Abort("user stopped early")
		if err != nil {
			t.Fatalf("Abort() err = %v, want nil", err)
		}
		if sess.Status() != aggregate.StatusAborted {
			t.Errorf("got status = %v, want ABORTED", sess.Status())
		}
		if sess.EndedAt() == nil {
			t.Error("want endedAt non-nil after Abort")
		}

		events := sess.PopEvents()
		if len(events) != 1 {
			t.Fatalf("got %d events, want 1", len(events))
		}
		ev, ok := events[0].(*event.WorkoutSessionAborted)
		if !ok {
			t.Fatalf("expected *event.WorkoutSessionAborted, got %T", events[0])
		}
		if ev.IsAnomalous {
			t.Error("want IsAnomalous = false for Abort()")
		}
		if ev.Reason != "user stopped early" {
			t.Errorf("got Reason = %q, want 'user stopped early'", ev.Reason)
		}
	})

	t.Run("AbortAnomalous emits anomalous event", func(t *testing.T) {
		sess, _ := aggregate.NewWorkoutSession("s-ab-2", "u1", "p1")
		sess.PopEvents() // clear start event

		err := sess.AbortAnomalous("critical fall detected")
		if err != nil {
			t.Fatalf("AbortAnomalous() err = %v, want nil", err)
		}
		if sess.Status() != aggregate.StatusAnomalous {
			t.Errorf("got status = %v, want ANOMALOUS", sess.Status())
		}
		if sess.EndedAt() == nil {
			t.Error("want endedAt non-nil after AbortAnomalous")
		}

		events := sess.PopEvents()
		if len(events) != 1 {
			t.Fatalf("got %d events, want 1", len(events))
		}
		ev, ok := events[0].(*event.WorkoutSessionAborted)
		if !ok {
			t.Fatalf("expected *event.WorkoutSessionAborted, got %T", events[0])
		}
		if !ev.IsAnomalous {
			t.Error("want IsAnomalous = true for AbortAnomalous()")
		}
		if ev.Reason != "critical fall detected" {
			t.Errorf("got Reason = %q, want 'critical fall detected'", ev.Reason)
		}
	})

	t.Run("Abort and AbortAnomalous on terminated session returns error", func(t *testing.T) {
		sess, _ := aggregate.NewWorkoutSession("s-ab-3", "u1", "p1")
		_ = sess.Complete(false, false)

		if err := sess.Abort("reason"); err == nil {
			t.Error("want error calling Abort on completed session")
		}
		if err := sess.AbortAnomalous("reason"); err == nil {
			t.Error("want error calling AbortAnomalous on completed session")
		}
	})
}

func TestWorkoutSession_RecordBodyMetricUpdate_EdgeCases(t *testing.T) {
	t.Run("weight > 0 emits BodyMetricUpdated event", func(t *testing.T) {
		sess, _ := aggregate.NewWorkoutSession("s-bm-edge", "u1", "p1")
		sess.PopEvents()

		sess.RecordBodyMetricUpdate(82.5)
		events := sess.PopEvents()
		if len(events) != 1 {
			t.Fatalf("got %d events, want 1", len(events))
		}
		ev, ok := events[0].(*event.BodyMetricUpdated)
		if !ok {
			t.Fatalf("expected *event.BodyMetricUpdated, got %T", events[0])
		}
		if ev.UserID != "u1" || ev.WeightKg != 82.5 {
			t.Errorf("got UserID=%s WeightKg=%v, want u1 82.5", ev.UserID, ev.WeightKg)
		}
	})

	t.Run("weight <= 0 emits no event", func(t *testing.T) {
		sess, _ := aggregate.NewWorkoutSession("s-bm-zero", "u1", "p1")
		sess.PopEvents()

		sess.RecordBodyMetricUpdate(0)
		sess.RecordBodyMetricUpdate(-10.0)

		events := sess.PopEvents()
		if len(events) != 0 {
			t.Errorf("got %d events, want 0 for weight <= 0", len(events))
		}
	})
}

func TestWorkoutSession_LogSet_DuplicateSetNumberOverwritesExisting(t *testing.T) {
	sess, _ := aggregate.NewWorkoutSession("s-dup-1", "u1", "p1")

	fs1 := float32(80.0)
	set1 := aggregate.WorkoutSetLog{
		ID:         "set-uuid-100",
		SetNumber:  1,
		ExerciseID: "ex-bench",
		TargetReps: 10,
		ActualReps: 10,
		Weight:     50.0,
		FormScore:  &fs1,
		RPE:        7.0,
	}

	err := sess.LogSet(set1)
	if err != nil {
		t.Fatalf("first LogSet err = %v, want nil", err)
	}

	if len(sess.Sets()) != 1 {
		t.Fatalf("got sets count = %d, want 1", len(sess.Sets()))
	}

	// Ghi đè set trùng set_number và exercise_id với input ID rỗng ("")
	fs2 := float32(95.0)
	set1Override := aggregate.WorkoutSetLog{
		ID:         "", // empty ID should retain existing set ID "set-uuid-100"
		SetNumber:  1,
		ExerciseID: "ex-bench",
		TargetReps: 10,
		ActualReps: 12,
		Weight:     55.0,
		FormScore:  &fs2,
		RPE:        8.5,
	}

	err = sess.LogSet(set1Override)
	if err != nil {
		t.Fatalf("second LogSet err = %v, want nil", err)
	}

	sets := sess.Sets()
	if len(sets) != 1 {
		t.Fatalf("got sets count after duplicate set_number = %d, want 1 (overwritten)", len(sets))
	}

	updatedSet := sets[0]
	if updatedSet.ID != "set-uuid-100" {
		t.Errorf("got set ID = %q, want 'set-uuid-100' (retained existing ID)", updatedSet.ID)
	}
	if updatedSet.Weight != 55.0 {
		t.Errorf("got Weight = %v, want 55.0", updatedSet.Weight)
	}
	if updatedSet.ActualReps != 12 {
		t.Errorf("got ActualReps = %d, want 12", updatedSet.ActualReps)
	}
	if updatedSet.FormScore == nil || *updatedSet.FormScore != 95.0 {
		t.Errorf("got FormScore = %v, want 95.0", updatedSet.FormScore)
	}
}
