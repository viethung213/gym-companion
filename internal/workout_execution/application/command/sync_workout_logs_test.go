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

func TestSyncWorkoutLogsHandler(t *testing.T) {
	t.Run("invalid input", func(t *testing.T) {
		h := command.NewSyncWorkoutLogsHandler(&mockSessionRepo{}, nil, nil)
		err := h.Handle(context.Background(), command.SyncWorkoutLogsCommand{})
		if !errors.Is(err, apperror.ErrInvalidInput) {
			t.Errorf("got %v, want %v", err, apperror.ErrInvalidInput)
		}
	})

	t.Run("find session error", func(t *testing.T) {
		h := command.NewSyncWorkoutLogsHandler(&mockSessionRepo{findErr: errors.New("db error")}, nil, nil)
		err := h.Handle(context.Background(), command.SyncWorkoutLogsCommand{SessionID: "s1"})
		if err == nil {
			t.Fatal("got nil, want error")
		}
	})

	t.Run("session not found", func(t *testing.T) {
		h := command.NewSyncWorkoutLogsHandler(&mockSessionRepo{}, nil, nil)
		err := h.Handle(context.Background(), command.SyncWorkoutLogsCommand{SessionID: "s1"})
		if !errors.Is(err, derror.ErrWorkoutSessionNotFound) {
			t.Errorf("got %v, want %v", err, derror.ErrWorkoutSessionNotFound)
		}
	})

	t.Run("save session error", func(t *testing.T) {
		session, _ := aggregate.NewWorkoutSession("s1", "u1", "p1")
		h := command.NewSyncWorkoutLogsHandler(&mockSessionRepo{session: session, saveErr: errors.New("save err")}, nil, nil)
		err := h.Handle(context.Background(), command.SyncWorkoutLogsCommand{SessionID: "s1", Errors: []command.ErrorLogDTO{{ErrorCode: "ERR_1"}}})
		if err == nil {
			t.Fatal("got nil, want error")
		}
	})

	t.Run("success", func(t *testing.T) {
		session, _ := aggregate.NewWorkoutSession("s1", "u1", "p1")
		h := command.NewSyncWorkoutLogsHandler(&mockSessionRepo{session: session}, nil, nil)
		err := h.Handle(context.Background(), command.SyncWorkoutLogsCommand{SessionID: "s1", Errors: []command.ErrorLogDTO{{ErrorCode: "ERR_1"}}})
		if err != nil {
			t.Fatalf("got err = %v, want nil", err)
		}
	})

	t.Run("success with CRITICAL error keeps session IN_PROGRESS so sets can be logged", func(t *testing.T) {
		session, _ := aggregate.NewWorkoutSession("s1", "u1", "p1")
		h := command.NewSyncWorkoutLogsHandler(&mockSessionRepo{session: session}, nil, nil)
		err := h.Handle(context.Background(), command.SyncWorkoutLogsCommand{
			SessionID: "s1",
			Errors: []command.ErrorLogDTO{
				{ErrorCode: "ERR_BAR_TRAPPED", Severity: "CRITICAL"},
			},
		})
		if err != nil {
			t.Fatalf("got err = %v, want nil", err)
		}
		if session.Status() != aggregate.StatusInProgress {
			t.Errorf("got status = %v, want %v", session.Status(), aggregate.StatusInProgress)
		}
	})
}
