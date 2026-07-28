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

func TestAbortWorkoutSessionHandler(t *testing.T) {
	t.Run("invalid input", func(t *testing.T) {
		h := command.NewAbortWorkoutSessionHandler(&mockSessionRepo{}, nil, &mockTxManager{})
		_, err := h.Handle(context.Background(), command.AbortWorkoutSessionCommand{})
		if !errors.Is(err, apperror.ErrInvalidInput) {
			t.Errorf("got %v, want %v", err, apperror.ErrInvalidInput)
		}
	})

	t.Run("find session error", func(t *testing.T) {
		h := command.NewAbortWorkoutSessionHandler(&mockSessionRepo{findErr: errors.New("db error")}, nil, &mockTxManager{})
		_, err := h.Handle(context.Background(), command.AbortWorkoutSessionCommand{SessionID: "s1"})
		if err == nil {
			t.Fatal("got nil, want error")
		}
	})

	t.Run("session not found", func(t *testing.T) {
		h := command.NewAbortWorkoutSessionHandler(&mockSessionRepo{}, nil, &mockTxManager{})
		_, err := h.Handle(context.Background(), command.AbortWorkoutSessionCommand{SessionID: "s1"})
		if !errors.Is(err, derror.ErrWorkoutSessionNotFound) {
			t.Errorf("got %v, want %v", err, derror.ErrWorkoutSessionNotFound)
		}
	})

	t.Run("abort already completed error", func(t *testing.T) {
		session, _ := aggregate.NewWorkoutSession("s1", "u1", "p1")
		_ = session.Complete(false, false)
		h := command.NewAbortWorkoutSessionHandler(&mockSessionRepo{session: session}, nil, &mockTxManager{})
		_, err := h.Handle(context.Background(), command.AbortWorkoutSessionCommand{SessionID: "s1", Reason: "user stop"})
		if err == nil {
			t.Fatal("got nil, want error")
		}
	})

	t.Run("save session tx error", func(t *testing.T) {
		session, _ := aggregate.NewWorkoutSession("s1", "u1", "p1")
		h := command.NewAbortWorkoutSessionHandler(&mockSessionRepo{session: session, saveErr: errors.New("save err")}, nil, &mockTxManager{})
		_, err := h.Handle(context.Background(), command.AbortWorkoutSessionCommand{SessionID: "s1", Reason: "user stop"})
		if err == nil {
			t.Fatal("got nil, want error")
		}
	})

	t.Run("outbox write tx error", func(t *testing.T) {
		session, _ := aggregate.NewWorkoutSession("s1", "u1", "p1")
		h := command.NewAbortWorkoutSessionHandler(&mockSessionRepo{session: session}, &mockOutboxWriter{err: errors.New("outbox err")}, &mockTxManager{})
		_, err := h.Handle(context.Background(), command.AbortWorkoutSessionCommand{SessionID: "s1", Reason: "user stop"})
		if err == nil {
			t.Fatal("got nil, want error")
		}
	})

	t.Run("tx manager error", func(t *testing.T) {
		session, _ := aggregate.NewWorkoutSession("s1", "u1", "p1")
		h := command.NewAbortWorkoutSessionHandler(&mockSessionRepo{session: session}, &mockOutboxWriter{}, &mockTxManager{err: errors.New("tx err")})
		_, err := h.Handle(context.Background(), command.AbortWorkoutSessionCommand{SessionID: "s1", Reason: "user stop"})
		if err == nil {
			t.Fatal("got nil, want error")
		}
	})

	t.Run("success", func(t *testing.T) {
		session, _ := aggregate.NewWorkoutSession("s1", "u1", "p1")
		h := command.NewAbortWorkoutSessionHandler(&mockSessionRepo{session: session}, &mockOutboxWriter{}, &mockTxManager{})
		res, err := h.Handle(context.Background(), command.AbortWorkoutSessionCommand{SessionID: "s1", Reason: "user stop"})
		if err != nil {
			t.Fatalf("got err = %v, want nil", err)
		}
		if res.SessionID != "s1" || res.AbortedAt == "" {
			t.Errorf("got invalid result fields")
		}
	})
}
