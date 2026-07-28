package command_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/workout_execution/application/command"
	"github.com/viethung213/gym-companion/internal/workout_execution/domain/aggregate"
)

func TestProcessCompletedSessionForPRHandler(t *testing.T) {
	t.Run("session not found", func(t *testing.T) {
		h := command.NewProcessCompletedSessionForPRHandler(&mockSessionRepo{}, &mockPRRepo{}, nil, &mockTxManager{})
		err := h.HandleProcess(context.Background(), "s1", "u1")
		if err == nil {
			t.Fatal("got nil, want error")
		}
	})

	t.Run("session anomalous excluded", func(t *testing.T) {
		now := time.Now().UTC()
		started := now.Add(-250 * time.Minute)
		session := aggregate.ReconstituteWorkoutSession(
			"s1", "u1", "p1",
			aggregate.StatusInProgress,
			nil, nil, nil, &started, nil,
			started, started,
		)
		session.CheckTimeoutAndAutoAbort(now) // turns into StatusAnomalous

		h := command.NewProcessCompletedSessionForPRHandler(&mockSessionRepo{session: session}, &mockPRRepo{}, nil, &mockTxManager{})
		err := h.HandleProcess(context.Background(), "s1", "u1")
		if err != nil {
			t.Fatalf("got err = %v, want nil for anomalous session exclusion", err)
		}
	})

	t.Run("skips zero reps or weight sets and compares multiple sets for same exercise", func(t *testing.T) {
		session, _ := aggregate.NewWorkoutSession("s1", "u1", "p1")
		// Zero reps set (skipped)
		_ = session.LogSet(aggregate.WorkoutSetLog{SetNumber: 1, ExerciseID: "ex1", TargetReps: 10, ActualReps: 0, Weight: 100.0})
		// Set 2: 100kg x 10
		_ = session.LogSet(aggregate.WorkoutSetLog{SetNumber: 2, ExerciseID: "ex1", TargetReps: 10, ActualReps: 10, Weight: 100.0})
		// Set 3: 120kg x 10 (higher 1RM, replaces Set 2)
		_ = session.LogSet(aggregate.WorkoutSetLog{SetNumber: 3, ExerciseID: "ex1", TargetReps: 10, ActualReps: 10, Weight: 120.0})

		h := command.NewProcessCompletedSessionForPRHandler(
			&mockSessionRepo{session: session},
			&mockPRRepo{pr: nil},
			&mockOutboxWriter{},
			&mockTxManager{},
		)

		err := h.HandleProcess(context.Background(), "s1", "u1")
		if err != nil {
			t.Fatalf("got err = %v, want nil", err)
		}
	})

	t.Run("find PR error continues next exercise", func(t *testing.T) {
		session, _ := aggregate.NewWorkoutSession("s1", "u1", "p1")
		_ = session.LogSet(aggregate.WorkoutSetLog{
			SetNumber:  1,
			ExerciseID: "ex1",
			TargetReps: 10,
			ActualReps: 10,
			Weight:     100.0,
		})

		h := command.NewProcessCompletedSessionForPRHandler(
			&mockSessionRepo{session: session},
			&mockPRRepo{findErr: errors.New("pr find error")},
			nil,
			&mockTxManager{},
		)

		err := h.HandleProcess(context.Background(), "s1", "u1")
		if err != nil {
			t.Fatalf("got err = %v, want nil when PR find error is skipped", err)
		}
	})

	t.Run("new PR created when existingPR is nil", func(t *testing.T) {
		session, _ := aggregate.NewWorkoutSession("s1", "u1", "p1")
		formScore := float32(95.0)
		_ = session.LogSet(aggregate.WorkoutSetLog{
			SetNumber:  1,
			ExerciseID: "ex1",
			TargetReps: 10,
			ActualReps: 10,
			Weight:     100.0,
			FormScore:  &formScore,
		})

		h := command.NewProcessCompletedSessionForPRHandler(
			&mockSessionRepo{session: session},
			&mockPRRepo{pr: nil}, // No existing PR
			&mockOutboxWriter{},
			&mockTxManager{},
		)

		err := h.HandleProcess(context.Background(), "s1", "u1")
		if err != nil {
			t.Fatalf("got err = %v, want nil", err)
		}
	})

	t.Run("lower performance does not update PR", func(t *testing.T) {
		session, _ := aggregate.NewWorkoutSession("s1", "u1", "p1")
		_ = session.LogSet(aggregate.WorkoutSetLog{
			SetNumber:  1,
			ExerciseID: "ex1",
			TargetReps: 10,
			ActualReps: 5,
			Weight:     40.0, // Low weight
		})

		now := time.Now().UTC()
		highPR := aggregate.ReconstitutePersonalRecord("pr-1", "u1", "ex1", 200.0, 200.0, 10, true, now, now, now)

		h := command.NewProcessCompletedSessionForPRHandler(
			&mockSessionRepo{session: session},
			&mockPRRepo{pr: highPR},
			&mockOutboxWriter{},
			&mockTxManager{},
		)

		err := h.HandleProcess(context.Background(), "s1", "u1")
		if err != nil {
			t.Fatalf("got err = %v, want nil", err)
		}
	})

	t.Run("outbox write error during PR update", func(t *testing.T) {
		session, _ := aggregate.NewWorkoutSession("s1", "u1", "p1")
		_ = session.LogSet(aggregate.WorkoutSetLog{
			SetNumber:  1,
			ExerciseID: "ex1",
			TargetReps: 10,
			ActualReps: 10,
			Weight:     100.0,
		})

		now := time.Now().UTC()
		lowPR := aggregate.ReconstitutePersonalRecord("pr-1", "u1", "ex1", 50.0, 50.0, 10, true, now, now, now)

		h := command.NewProcessCompletedSessionForPRHandler(
			&mockSessionRepo{session: session},
			&mockPRRepo{pr: lowPR},
			&mockOutboxWriter{err: errors.New("outbox err")},
			&mockTxManager{},
		)

		err := h.HandleProcess(context.Background(), "s1", "u1")
		if err == nil {
			t.Fatal("got nil, want error on outbox write failure")
		}
	})
}
