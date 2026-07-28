package command_test

import (
	"context"
	"errors"
	"testing"

	"github.com/viethung213/gym-companion/internal/workout_execution/application/apperror"
	"github.com/viethung213/gym-companion/internal/workout_execution/application/command"
	"github.com/viethung213/gym-companion/internal/workout_execution/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/workout_execution/domain/derror"
)

func TestLogWorkoutSetHandler(t *testing.T) {
	t.Run("invalid input", func(t *testing.T) {
		h := command.NewLogWorkoutSetHandler(&mockSessionRepo{})
		_, err := h.Handle(context.Background(), command.LogWorkoutSetCommand{})
		if !errors.Is(err, apperror.ErrInvalidInput) {
			t.Errorf("got %v, want %v", err, apperror.ErrInvalidInput)
		}
	})

	t.Run("find session error", func(t *testing.T) {
		h := command.NewLogWorkoutSetHandler(&mockSessionRepo{findErr: errors.New("db error")})
		_, err := h.Handle(context.Background(), command.LogWorkoutSetCommand{SessionID: "s1", ExerciseID: "ex1"})
		if err == nil {
			t.Fatal("got nil, want error")
		}
	})

	t.Run("session not found", func(t *testing.T) {
		h := command.NewLogWorkoutSetHandler(&mockSessionRepo{})
		_, err := h.Handle(context.Background(), command.LogWorkoutSetCommand{SessionID: "s1", ExerciseID: "ex1"})
		if !errors.Is(err, derror.ErrWorkoutSessionNotFound) {
			t.Errorf("got %v, want %v", err, derror.ErrWorkoutSessionNotFound)
		}
	})

	t.Run("log set error", func(t *testing.T) {
		session, _ := aggregate.NewWorkoutSession("s1", "u1", "p1")
		_ = session.Complete(false, false) // session no longer in progress
		h := command.NewLogWorkoutSetHandler(&mockSessionRepo{session: session})
		_, err := h.Handle(context.Background(), command.LogWorkoutSetCommand{SessionID: "s1", ExerciseID: "ex1", SetNumber: 1})
		if err == nil {
			t.Fatal("got nil, want error")
		}
	})

	t.Run("save session error", func(t *testing.T) {
		session, _ := aggregate.NewWorkoutSession("s1", "u1", "p1")
		h := command.NewLogWorkoutSetHandler(&mockSessionRepo{session: session, saveErr: errors.New("save err")})
		_, err := h.Handle(context.Background(), command.LogWorkoutSetCommand{SessionID: "s1", ExerciseID: "ex1", SetNumber: 1})
		if err == nil {
			t.Fatal("got nil, want error")
		}
	})

	t.Run("success", func(t *testing.T) {
		session, _ := aggregate.NewWorkoutSession("s1", "u1", "p1")
		h := command.NewLogWorkoutSetHandler(&mockSessionRepo{session: session})
		res, err := h.Handle(context.Background(), command.LogWorkoutSetCommand{SessionID: "s1", ExerciseID: "ex1", SetNumber: 1})
		if err != nil {
			t.Fatalf("got err = %v, want nil", err)
		}
		if res.SetLogID == "" {
			t.Errorf("got empty SetLogID")
		}
	})
}
