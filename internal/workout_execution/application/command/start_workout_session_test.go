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

func TestStartWorkoutSessionHandler(t *testing.T) {
	t.Run("invalid input", func(t *testing.T) {
		h := command.NewStartWorkoutSessionHandler(&mockSessionRepo{}, nil, nil, &mockTxManager{})
		_, err := h.Handle(context.Background(), command.StartWorkoutSessionCommand{})
		if !errors.Is(err, apperror.ErrInvalidInput) {
			t.Errorf("got %v, want %v", err, apperror.ErrInvalidInput)
		}
	})

	t.Run("query active session error", func(t *testing.T) {
		h := command.NewStartWorkoutSessionHandler(&mockSessionRepo{findErr: errors.New("db error")}, nil, nil, &mockTxManager{})
		_, err := h.Handle(context.Background(), command.StartWorkoutSessionCommand{UserID: "u1", PlanID: "p1"})
		if err == nil {
			t.Fatal("got nil, want error")
		}
	})

	t.Run("active session already exists", func(t *testing.T) {
		active, _ := aggregate.NewWorkoutSession("s1", "u1", "p1")
		h := command.NewStartWorkoutSessionHandler(&mockSessionRepo{activeSession: active}, nil, nil, &mockTxManager{})
		_, err := h.Handle(context.Background(), command.StartWorkoutSessionCommand{UserID: "u1", PlanID: "p1"})
		if !errors.Is(err, derror.ErrActiveSessionAlreadyExists) {
			t.Errorf("got %v, want %v", err, derror.ErrActiveSessionAlreadyExists)
		}
	})

	t.Run("plan validation error", func(t *testing.T) {
		h := command.NewStartWorkoutSessionHandler(&mockSessionRepo{}, &mockPlanClient{err: errors.New("plan error")}, nil, &mockTxManager{})
		_, err := h.Handle(context.Background(), command.StartWorkoutSessionCommand{UserID: "u1", PlanID: "p1"})
		if err == nil {
			t.Fatal("got nil, want error")
		}
	})

	t.Run("plan not found", func(t *testing.T) {
		h := command.NewStartWorkoutSessionHandler(&mockSessionRepo{}, &mockPlanClient{exists: false}, nil, &mockTxManager{})
		_, err := h.Handle(context.Background(), command.StartWorkoutSessionCommand{UserID: "u1", PlanID: "p1"})
		if !errors.Is(err, apperror.ErrDailyPlanNotFound) {
			t.Errorf("got %v, want %v", err, apperror.ErrDailyPlanNotFound)
		}
	})

	t.Run("save session tx error", func(t *testing.T) {
		h := command.NewStartWorkoutSessionHandler(&mockSessionRepo{saveErr: errors.New("save err")}, &mockPlanClient{exists: true}, nil, &mockTxManager{})
		_, err := h.Handle(context.Background(), command.StartWorkoutSessionCommand{UserID: "u1", PlanID: "p1"})
		if err == nil {
			t.Fatal("got nil, want error")
		}
	})

	t.Run("outbox write tx error", func(t *testing.T) {
		h := command.NewStartWorkoutSessionHandler(&mockSessionRepo{}, &mockPlanClient{exists: true}, &mockOutboxWriter{err: errors.New("outbox err")}, &mockTxManager{})
		_, err := h.Handle(context.Background(), command.StartWorkoutSessionCommand{UserID: "u1", PlanID: "p1"})
		if err == nil {
			t.Fatal("got nil, want error")
		}
	})

	t.Run("tx manager error", func(t *testing.T) {
		h := command.NewStartWorkoutSessionHandler(&mockSessionRepo{}, &mockPlanClient{exists: true}, &mockOutboxWriter{}, &mockTxManager{err: errors.New("tx err")})
		_, err := h.Handle(context.Background(), command.StartWorkoutSessionCommand{UserID: "u1", PlanID: "p1"})
		if err == nil {
			t.Fatal("got nil, want error")
		}
	})

	t.Run("success", func(t *testing.T) {
		h := command.NewStartWorkoutSessionHandler(&mockSessionRepo{}, &mockPlanClient{exists: true}, &mockOutboxWriter{}, &mockTxManager{})
		res, err := h.Handle(context.Background(), command.StartWorkoutSessionCommand{UserID: "u1", PlanID: "p1"})
		if err != nil {
			t.Fatalf("got err = %v, want nil", err)
		}
		if res.SessionID == "" || res.StartedAt == "" {
			t.Errorf("got empty result fields")
		}
	})
}
