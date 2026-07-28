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

func TestStartScheduledWorkoutSessionHandler(t *testing.T) {
	t.Run("invalid input", func(t *testing.T) {
		h := command.NewStartScheduledWorkoutSessionHandler(&mockSessionRepo{}, nil, &mockTxManager{})
		_, err := h.Handle(context.Background(), command.StartScheduledWorkoutSessionCommand{})
		if !errors.Is(err, apperror.ErrInvalidInput) {
			t.Errorf("got %v, want %v", err, apperror.ErrInvalidInput)
		}
	})

	t.Run("scheduled session not found", func(t *testing.T) {
		h := command.NewStartScheduledWorkoutSessionHandler(&mockSessionRepo{}, nil, &mockTxManager{})
		_, err := h.Handle(context.Background(), command.StartScheduledWorkoutSessionCommand{
			SessionID: "s-missing",
			UserID:    "u1",
		})
		if !errors.Is(err, derror.ErrWorkoutSessionNotFound) {
			t.Errorf("got %v, want %v", err, derror.ErrWorkoutSessionNotFound)
		}
	})

	t.Run("user id mismatch", func(t *testing.T) {
		scheduled, _ := aggregate.NewScheduledWorkoutSession("sched-1", "u1", "p1", time.Now().UTC())
		h := command.NewStartScheduledWorkoutSessionHandler(&mockSessionRepo{session: scheduled}, nil, &mockTxManager{})
		_, err := h.Handle(context.Background(), command.StartScheduledWorkoutSessionCommand{
			SessionID: "sched-1",
			UserID:    "u2-wrong",
		})
		if !errors.Is(err, derror.ErrWorkoutSessionNotFound) {
			t.Errorf("got %v, want %v", err, derror.ErrWorkoutSessionNotFound)
		}
	})

	t.Run("start pre-scheduled session success", func(t *testing.T) {
		scheduled, _ := aggregate.NewScheduledWorkoutSession("sched-1", "u1", "p1", time.Now().UTC())
		h := command.NewStartScheduledWorkoutSessionHandler(&mockSessionRepo{session: scheduled}, &mockOutboxWriter{}, &mockTxManager{})
		res, err := h.Handle(context.Background(), command.StartScheduledWorkoutSessionCommand{
			SessionID: "sched-1",
			UserID:    "u1",
		})
		if err != nil {
			t.Fatalf("got err = %v, want nil", err)
		}
		if res.SessionID != "sched-1" {
			t.Errorf("got %v, want sched-1", res.SessionID)
		}
		if scheduled.Status() != aggregate.StatusInProgress {
			t.Errorf("got status = %v, want IN_PROGRESS", scheduled.Status())
		}
	})
}
