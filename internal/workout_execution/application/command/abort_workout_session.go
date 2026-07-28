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
func (h *AbortWorkoutSessionHandler) Handle(ctx context.Context, cmd AbortWorkoutSessionCommand) (*AbortWorkoutSessionResult, error) {
	if cmd.SessionID == "" {
		return nil, apperror.ErrInvalidInput
	}

	session, err := h.sessionRepo.FindByID(ctx, cmd.SessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to find session: %w", err)
	}
	if session == nil {
		return nil, derror.ErrWorkoutSessionNotFound
	}

	if err := session.Abort(cmd.Reason); err != nil {
		return nil, err
	}

	err = h.txManager.WithTransaction(ctx, func(txCtx context.Context) error {
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
	})

	if err != nil {
		return nil, fmt.Errorf("failed to abort session tx: %w", err)
	}

	var abortedAtStr string
	if session.EndedAt() != nil {
		abortedAtStr = session.EndedAt().Format("2006-01-02T15:04:05Z07:00")
	}

	return &AbortWorkoutSessionResult{
		SessionID: session.ID(),
		AbortedAt: abortedAtStr,
	}, nil
}
