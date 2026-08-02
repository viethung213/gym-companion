package command_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/workout_execution/application/apperror"
	"github.com/viethung213/gym-companion/internal/workout_execution/application/command"
	"github.com/viethung213/gym-companion/internal/workout_execution/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/workout_execution/domain/derror"
)

func TestLogWorkoutSetHandler(t *testing.T) {
	t.Run("invalid input", func(t *testing.T) {
		h := command.NewLogWorkoutSetHandler(&mockSessionRepo{}, newMockOutboxWriter(t), newMockTxManager(t))
		_, err := h.Handle(context.Background(), command.LogWorkoutSetCommand{})
		if !errors.Is(err, apperror.ErrInvalidInput) {
			t.Errorf("got %v, want %v", err, apperror.ErrInvalidInput)
		}
	})

	t.Run("find session error", func(t *testing.T) {
		h := command.NewLogWorkoutSetHandler(&mockSessionRepo{findErr: errors.New("db error")}, newMockOutboxWriter(t), newMockTxManager(t))
		_, err := h.Handle(context.Background(), command.LogWorkoutSetCommand{SessionID: "s1", ExerciseID: "ex1"})
		if err == nil {
			t.Fatal("got nil, want error")
		}
	})

	t.Run("session not found", func(t *testing.T) {
		h := command.NewLogWorkoutSetHandler(&mockSessionRepo{}, newMockOutboxWriter(t), newMockTxManager(t))
		_, err := h.Handle(context.Background(), command.LogWorkoutSetCommand{SessionID: "s1", ExerciseID: "ex1"})
		if !errors.Is(err, derror.ErrWorkoutSessionNotFound) {
			t.Errorf("got %v, want %v", err, derror.ErrWorkoutSessionNotFound)
		}
	})

	t.Run("log set error", func(t *testing.T) {
		session, _ := aggregate.NewWorkoutSession("s1", "u1", "p1")
		_ = session.Complete(false, false) // session no longer in progress
		h := command.NewLogWorkoutSetHandler(&mockSessionRepo{session: session}, newMockOutboxWriter(t), newMockTxManager(t))
		_, err := h.Handle(context.Background(), command.LogWorkoutSetCommand{SessionID: "s1", ExerciseID: "ex1", SetNumber: 1})
		if err == nil {
			t.Fatal("got nil, want error")
		}
	})

	t.Run("save session error", func(t *testing.T) {
		session, _ := aggregate.NewWorkoutSession("s1", "u1", "p1")
		h := command.NewLogWorkoutSetHandler(&mockSessionRepo{session: session, saveErr: errors.New("save err")}, newMockOutboxWriter(t), newMockTxManager(t))
		_, err := h.Handle(context.Background(), command.LogWorkoutSetCommand{SessionID: "s1", ExerciseID: "ex1", SetNumber: 1})
		if err == nil {
			t.Fatal("got nil, want error")
		}
	})

	t.Run("success", func(t *testing.T) {
		session, _ := aggregate.NewWorkoutSession("s1", "u1", "p1")
		h := command.NewLogWorkoutSetHandler(&mockSessionRepo{session: session}, newMockOutboxWriter(t), newMockTxManager(t))
		res, err := h.Handle(context.Background(), command.LogWorkoutSetCommand{SessionID: "s1", ExerciseID: "ex1", SetNumber: 1})
		if err != nil {
			t.Fatalf("got err = %v, want nil", err)
		}
		if res.SetLogID == "" {
			t.Errorf("got empty SetLogID")
		}
	})

	t.Run("anomalous timeout persists session state and returns error", func(t *testing.T) {
		now := time.Now().UTC()
		started := now.Add(-250 * time.Minute)
		session := aggregate.ReconstituteWorkoutSession(
			"s1", "u1", "p1",
			aggregate.StatusInProgress,
			nil, nil, nil, &started, nil,
			started, started,
		)
		repo := &mockSessionRepo{session: session}
		outbox := newMockOutboxWriter(t)
		h := command.NewLogWorkoutSetHandler(repo, outbox, newMockTxManager(t))

		_, err := h.Handle(context.Background(), command.LogWorkoutSetCommand{SessionID: "s1", ExerciseID: "ex1", SetNumber: 1})
		if !errors.Is(err, derror.ErrAnomalousSessionTimeout) {
			t.Fatalf("got err = %v, want %v", err, derror.ErrAnomalousSessionTimeout)
		}
		if repo.session.Status() != aggregate.StatusAnomalous {
			t.Errorf("got session status = %v, want StatusAnomalous", repo.session.Status())
		}
	})
}
