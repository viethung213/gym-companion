package command_test

import (
	"context"
	"errors"
	"testing"

	"github.com/viethung213/gym-companion/internal/workout_execution/application/apperror"
	"github.com/viethung213/gym-companion/internal/workout_execution/application/command"
	"github.com/viethung213/gym-companion/internal/workout_execution/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/workout_execution/domain/derror"
	"github.com/viethung213/gym-companion/internal/workout_execution/domain/service"
)

type mockVolumeProvider struct {
	volumes []float32
}

func (m *mockVolumeProvider) GetRecentVolumesForMuscleGroup(ctx context.Context, userID, muscleGroup string, limit int) ([]float32, error) {
	return m.volumes, nil
}

func TestCompleteWorkoutSessionHandler(t *testing.T) {
	t.Run("invalid input", func(t *testing.T) {
		h := command.NewCompleteWorkoutSessionHandler(&mockSessionRepo{}, nil, nil, nil, &mockTxManager{})
		_, err := h.Handle(context.Background(), command.CompleteWorkoutSessionCommand{})
		if !errors.Is(err, apperror.ErrInvalidInput) {
			t.Errorf("got %v, want %v", err, apperror.ErrInvalidInput)
		}
	})

	t.Run("find session error", func(t *testing.T) {
		h := command.NewCompleteWorkoutSessionHandler(&mockSessionRepo{findErr: errors.New("db error")}, nil, nil, nil, &mockTxManager{})
		_, err := h.Handle(context.Background(), command.CompleteWorkoutSessionCommand{SessionID: "s1"})
		if err == nil {
			t.Fatal("got nil, want error")
		}
	})

	t.Run("session not found", func(t *testing.T) {
		h := command.NewCompleteWorkoutSessionHandler(&mockSessionRepo{}, nil, nil, nil, &mockTxManager{})
		_, err := h.Handle(context.Background(), command.CompleteWorkoutSessionCommand{SessionID: "s1"})
		if !errors.Is(err, derror.ErrWorkoutSessionNotFound) {
			t.Errorf("got %v, want %v", err, derror.ErrWorkoutSessionNotFound)
		}
	})

	t.Run("overload requires confirmation error", func(t *testing.T) {
		session, _ := aggregate.NewWorkoutSession("s1", "u1", "p1")
		_ = session.LogSet(aggregate.WorkoutSetLog{
			SetNumber:  1,
			ExerciseID: "ex1",
			TargetReps: 10,
			ActualReps: 10,
			Weight:     1000.0, // High volume to trigger overload
		})

		guard := service.NewTrainingLoadGuard(&mockVolumeProvider{volumes: []float32{100.0}})
		h := command.NewCompleteWorkoutSessionHandler(
			&mockSessionRepo{session: session},
			guard,
			&mockExerciseClient{group: "Chest"},
			nil,
			&mockTxManager{},
		)

		_, err := h.Handle(context.Background(), command.CompleteWorkoutSessionCommand{SessionID: "s1", ConfirmOverload: false})
		if !errors.Is(err, derror.ErrOverloadConfirmationRequired) {
			t.Errorf("got %v, want ErrOverloadConfirmationRequired", err)
		}
	})

	t.Run("tx manager save session error", func(t *testing.T) {
		session, _ := aggregate.NewWorkoutSession("s1", "u1", "p1")
		h := command.NewCompleteWorkoutSessionHandler(
			&mockSessionRepo{session: session, saveErr: errors.New("save error")},
			nil, nil, nil,
			&mockTxManager{},
		)
		_, err := h.Handle(context.Background(), command.CompleteWorkoutSessionCommand{SessionID: "s1"})
		if err == nil {
			t.Fatal("got nil, want error")
		}
	})

	t.Run("outbox write error", func(t *testing.T) {
		session, _ := aggregate.NewWorkoutSession("s1", "u1", "p1")
		h := command.NewCompleteWorkoutSessionHandler(
			&mockSessionRepo{session: session},
			nil, nil,
			&mockOutboxWriter{err: errors.New("outbox err")},
			&mockTxManager{},
		)
		_, err := h.Handle(context.Background(), command.CompleteWorkoutSessionCommand{SessionID: "s1"})
		if err == nil {
			t.Fatal("got nil, want error")
		}
	})

	t.Run("success with exercise client", func(t *testing.T) {
		session, _ := aggregate.NewWorkoutSession("s1", "u1", "p1")
		_ = session.LogSet(aggregate.WorkoutSetLog{
			SetNumber:  1,
			ExerciseID: "ex1",
			TargetReps: 10,
			ActualReps: 10,
			Weight:     50.0,
		})

		guard := service.NewTrainingLoadGuard(&mockVolumeProvider{volumes: []float32{500.0}})
		h := command.NewCompleteWorkoutSessionHandler(
			&mockSessionRepo{session: session},
			guard,
			&mockExerciseClient{group: "Chest"},
			&mockOutboxWriter{},
			&mockTxManager{},
		)

		res, err := h.Handle(context.Background(), command.CompleteWorkoutSessionCommand{SessionID: "s1"})
		if err != nil {
			t.Fatalf("got err = %v, want nil", err)
		}
		if res.SessionID != "s1" || res.TotalSets != 1 {
			t.Errorf("got invalid summary result: %+v", res)
		}
	})

	t.Run("success with optional weight update emits BodyMetricUpdated event", func(t *testing.T) {
		session, _ := aggregate.NewWorkoutSession("s1", "u1", "p1")
		outbox := &mockOutboxWriter{}
		h := command.NewCompleteWorkoutSessionHandler(
			&mockSessionRepo{session: session},
			nil, nil,
			outbox,
			&mockTxManager{},
		)

		weight := float32(75.5)
		res, err := h.Handle(context.Background(), command.CompleteWorkoutSessionCommand{
			SessionID:      "s1",
			WeightUpdateKg: &weight,
		})
		if err != nil {
			t.Fatalf("got err = %v, want nil", err)
		}
		if res.SessionID != "s1" {
			t.Errorf("got invalid result: %+v", res)
		}
		if len(outbox.events) != 3 {
			t.Errorf("got %d events in outbox, want 3 (WorkoutSessionStarted + WorkoutSessionCompleted + BodyMetricUpdated)", len(outbox.events))
		}
	})
}
