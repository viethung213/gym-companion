package command

import (
	"context"
	"fmt"

	"github.com/viethung213/gym-companion/internal/workout_execution/application/apperror"
	"github.com/viethung213/gym-companion/internal/workout_execution/application/port"
	"github.com/viethung213/gym-companion/internal/workout_execution/domain/derror"
	"github.com/viethung213/gym-companion/internal/workout_execution/domain/repository"
)

// StartScheduledWorkoutSessionCommand contains parameters to start a pre-scheduled session.
type StartScheduledWorkoutSessionCommand struct {
	SessionID string
	UserID    string
}

// StartScheduledWorkoutSessionResult returns result of starting a scheduled session.
type StartScheduledWorkoutSessionResult struct {
	SessionID string
	StartedAt string
}

// StartScheduledWorkoutSessionHandler handles starting a pre-scheduled workout session.
type StartScheduledWorkoutSessionHandler struct {
	sessionRepo repository.WorkoutSessionRepository
	outbox      port.OutboxWriter
	txManager   port.TxManager
}

// NewStartScheduledWorkoutSessionHandler constructs the command handler.
func NewStartScheduledWorkoutSessionHandler(
	sessionRepo repository.WorkoutSessionRepository,
	outbox port.OutboxWriter,
	txManager port.TxManager,
) *StartScheduledWorkoutSessionHandler {
	return &StartScheduledWorkoutSessionHandler{
		sessionRepo: sessionRepo,
		outbox:      outbox,
		txManager:   txManager,
	}
}

// Handle executes the StartScheduledWorkoutSession command.
func (h *StartScheduledWorkoutSessionHandler) Handle(ctx context.Context, cmd StartScheduledWorkoutSessionCommand) (*StartScheduledWorkoutSessionResult, error) {
	if cmd.SessionID == "" || cmd.UserID == "" {
		return nil, apperror.ErrInvalidInput
	}

	session, err := h.sessionRepo.FindByID(ctx, cmd.SessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to find scheduled session: %w", err)
	}
	if session == nil || session.UserID() != cmd.UserID {
		return nil, derror.ErrWorkoutSessionNotFound
	}

	if err := session.Start(); err != nil {
		return nil, fmt.Errorf("failed to start scheduled session: %w", err)
	}

	if err := h.txManager.WithTransaction(ctx, func(txCtx context.Context) error {
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
	}); err != nil {
		return nil, fmt.Errorf("failed to execute start scheduled session tx: %w", err)
	}

	var startedAtStr string
	if session.StartedAt() != nil {
		startedAtStr = session.StartedAt().Format("2006-01-02T15:04:05Z07:00")
	}

	return &StartScheduledWorkoutSessionResult{
		SessionID: session.ID(),
		StartedAt: startedAtStr,
	}, nil
}
