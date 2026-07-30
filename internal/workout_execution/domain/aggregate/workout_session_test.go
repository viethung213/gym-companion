package aggregate_test

import (
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/workout_execution/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/workout_execution/domain/derror"
	"github.com/viethung213/gym-companion/internal/workout_execution/domain/event"
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
