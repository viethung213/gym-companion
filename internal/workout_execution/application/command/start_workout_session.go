package command

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/viethung213/gym-companion/internal/workout_execution/application/apperror"
	"github.com/viethung213/gym-companion/internal/workout_execution/application/port"
	"github.com/viethung213/gym-companion/internal/workout_execution/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/workout_execution/domain/derror"
	"github.com/viethung213/gym-companion/internal/workout_execution/domain/repository"
)

// StartWorkoutSessionCommand contains parameters to start an ad-hoc session.
type StartWorkoutSessionCommand struct {
	UserID string
	PlanID string
}

// StartWorkoutSessionResult returns result of starting a session.
type StartWorkoutSessionResult struct {
	SessionID string
	StartedAt string
}

// StartWorkoutSessionHandler handles initiating an ad-hoc workout session.
type StartWorkoutSessionHandler struct {
	sessionRepo repository.WorkoutSessionRepository
	planClient  port.DailyWorkoutPlanClient
	outbox      port.OutboxWriter
	txManager   port.TxManager
}

// NewStartWorkoutSessionHandler constructs the command handler.
func NewStartWorkoutSessionHandler(
	sessionRepo repository.WorkoutSessionRepository,
	planClient port.DailyWorkoutPlanClient,
	outbox port.OutboxWriter,
	txManager port.TxManager,
) *StartWorkoutSessionHandler {
	return &StartWorkoutSessionHandler{
		sessionRepo: sessionRepo,
		planClient:  planClient,
		outbox:      outbox,
		txManager:   txManager,
	}
}

// Handle executes the StartWorkoutSession command.
func (h *StartWorkoutSessionHandler) Handle(ctx context.Context, cmd StartWorkoutSessionCommand) (*StartWorkoutSessionResult, error) {
	if cmd.UserID == "" || cmd.PlanID == "" {
		return nil, apperror.ErrInvalidInput
	}

	activeSession, err := h.sessionRepo.FindActiveSessionByUserID(ctx, cmd.UserID)
	if err != nil {
		return nil, fmt.Errorf("failed to query active sessions: %w", err)
	}
	if activeSession != nil {
		return nil, derror.ErrActiveSessionAlreadyExists
	}

	if h.planClient != nil {
		exists, err := h.planClient.ValidatePlanExists(ctx, cmd.UserID, cmd.PlanID)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", apperror.ErrDailyPlanNotFound, err)
		}
		if !exists {
			return nil, apperror.ErrDailyPlanNotFound
		}
	}

	sessionID := uuid.NewString()
	session, err := aggregate.NewWorkoutSession(sessionID, cmd.UserID, cmd.PlanID)
	if err != nil {
		return nil, err
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
		return nil, fmt.Errorf("failed to execute start session tx: %w", err)
	}

	var startedAtStr string
	if session.StartedAt() != nil {
		startedAtStr = session.StartedAt().Format("2006-01-02T15:04:05Z07:00")
	}

	return &StartWorkoutSessionResult{
		SessionID: session.ID(),
		StartedAt: startedAtStr,
	}, nil
}
