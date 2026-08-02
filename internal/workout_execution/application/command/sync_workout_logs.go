package command

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/viethung213/gym-companion/internal/workout_execution/application/apperror"
	"github.com/viethung213/gym-companion/internal/workout_execution/application/port"
	"github.com/viethung213/gym-companion/internal/workout_execution/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/workout_execution/domain/derror"
	"github.com/viethung213/gym-companion/internal/workout_execution/domain/repository"
)

// ErrorLogDTO input error item.
type ErrorLogDTO struct {
	ErrorCode  string
	Severity   string
	Timestamp  time.Time
	SetNumber  int
	RepNumber  int
	ExerciseID string
}

// SyncWorkoutLogsCommand parameters.
type SyncWorkoutLogsCommand struct {
	SessionID string
	Errors    []ErrorLogDTO
}

// SyncWorkoutLogsHandler handles posture error batch sync.
type SyncWorkoutLogsHandler struct {
	sessionRepo repository.WorkoutSessionRepository
	outbox      port.OutboxWriter
	txManager   port.TxManager
}

// NewSyncWorkoutLogsHandler constructs handler.
func NewSyncWorkoutLogsHandler(
	sessionRepo repository.WorkoutSessionRepository,
	outbox port.OutboxWriter,
	txManager port.TxManager,
) *SyncWorkoutLogsHandler {
	return &SyncWorkoutLogsHandler{
		sessionRepo: sessionRepo,
		outbox:      outbox,
		txManager:   txManager,
	}
}

// Handle executes batch error sync.
func (h *SyncWorkoutLogsHandler) Handle(ctx context.Context, cmd SyncWorkoutLogsCommand) error {
	if cmd.SessionID == "" {
		return apperror.ErrInvalidInput
	}

	domainErrors := make([]aggregate.SessionError, len(cmd.Errors))
	for i, e := range cmd.Errors {
		domainErrors[i] = aggregate.SessionError{
			ID:         uuid.NewString(),
			SessionID:  cmd.SessionID,
			SetNumber:  e.SetNumber,
			RepNumber:  e.RepNumber,
			ExerciseID: e.ExerciseID,
			ErrorCode:  e.ErrorCode,
			Severity:   e.Severity,
			Timestamp:  e.Timestamp,
		}
	}

	saveFunc := func(txCtx context.Context) error {
		session, err := h.sessionRepo.FindByIDForUpdate(txCtx, cmd.SessionID)
		if err != nil {
			return fmt.Errorf("failed to find session: %w", err)
		}
		if session == nil {
			return derror.ErrWorkoutSessionNotFound
		}

		session.AddErrors(domainErrors)

		if err := h.sessionRepo.Save(txCtx, session); err != nil {
			return err
		}
		events := session.PopEvents()
		if len(events) > 0 && h.outbox != nil {
			if err := h.outbox.WriteEvents(txCtx, "WorkoutSession", session.ID(), events); err != nil {
				return err
			}
		}
		return nil
	}

	if h.txManager != nil {
		if err := h.txManager.WithTransaction(ctx, saveFunc); err != nil {
			return fmt.Errorf("failed to sync workout logs tx: %w", err)
		}
	} else {
		if err := saveFunc(ctx); err != nil {
			return fmt.Errorf("failed to sync workout logs: %w", err)
		}
	}

	return nil
}
