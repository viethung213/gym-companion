package command

import (
	"context"
	"fmt"

	"github.com/viethung213/gym-companion/internal/workout_execution/application/apperror"
	"github.com/viethung213/gym-companion/internal/workout_execution/application/port"
	"github.com/viethung213/gym-companion/internal/workout_execution/domain/derror"
	"github.com/viethung213/gym-companion/internal/workout_execution/domain/repository"
)

// AbortWorkoutSessionCommand parameters.
type AbortWorkoutSessionCommand struct {
	SessionID string
	UserID    string
	Reason    string
}

// AbortWorkoutSessionResult result.
type AbortWorkoutSessionResult struct {
	SessionID string
	AbortedAt string
}

// AbortWorkoutSessionHandler handles session abortion.
type AbortWorkoutSessionHandler struct {
	sessionRepo repository.WorkoutSessionRepository
	outbox      port.OutboxWriter
	txManager   port.TxManager
}

// NewAbortWorkoutSessionHandler constructs AbortWorkoutSessionHandler.
func NewAbortWorkoutSessionHandler(
	sessionRepo repository.WorkoutSessionRepository,
	outbox port.OutboxWriter,
	txManager port.TxManager,
) *AbortWorkoutSessionHandler {
	return &AbortWorkoutSessionHandler{
		sessionRepo: sessionRepo,
		outbox:      outbox,
		txManager:   txManager,
	}
}

// Handle executes session abortion.
func (h *AbortWorkoutSessionHandler) Handle(
	ctx context.Context,
	cmd AbortWorkoutSessionCommand,
) (*AbortWorkoutSessionResult, error) {
	if cmd.SessionID == "" {
		return nil, apperror.ErrInvalidInput
	}

	var result *AbortWorkoutSessionResult
	saveFunc := func(txCtx context.Context) error {
		session, err := h.sessionRepo.FindByIDForUpdate(txCtx, cmd.SessionID)
		if err != nil {
			return fmt.Errorf("failed to find session: %w", err)
		}
		if session == nil {
			return derror.ErrWorkoutSessionNotFound
		}
		if cmd.UserID != "" && session.UserID() != cmd.UserID {
			return derror.ErrForbidden
		}

		if abortErr := session.Abort(cmd.Reason); abortErr != nil {
			return abortErr
		}

		if saveErr := h.sessionRepo.Save(txCtx, session); saveErr != nil {
			return saveErr
		}
		events := session.PopEvents()
		if len(events) > 0 && h.outbox != nil {
			if writeErr := h.outbox.WriteEvents(txCtx, "WorkoutSession", session.ID(), events); writeErr != nil {
				return writeErr
			}
		}

		var abortedAtStr string
		if session.EndedAt() != nil {
			abortedAtStr = session.EndedAt().Format("2006-01-02T15:04:05Z07:00")
		}

		result = &AbortWorkoutSessionResult{
			SessionID: session.ID(),
			AbortedAt: abortedAtStr,
		}
		return nil
	}

	if h.txManager != nil {
		if txErr := h.txManager.WithTransaction(ctx, saveFunc); txErr != nil {
			return nil, fmt.Errorf("failed to abort session tx: %w", txErr)
		}
	} else {
		if err := saveFunc(ctx); err != nil {
			return nil, err
		}
	}

	return result, nil
}
