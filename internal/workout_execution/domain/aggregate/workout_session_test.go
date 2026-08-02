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
