package command

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/viethung213/gym-companion/internal/workout_execution/application/apperror"
	"github.com/viethung213/gym-companion/internal/workout_execution/application/port"
	"github.com/viethung213/gym-companion/internal/workout_execution/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/workout_execution/domain/derror"
	"github.com/viethung213/gym-companion/internal/workout_execution/domain/repository"
	"github.com/viethung213/gym-companion/internal/workout_execution/domain/vo"
)

// LogWorkoutSetCommand contains parameters to log a completed set.
type LogWorkoutSetCommand struct {
	SessionID   string
	UserID      string
	SetNumber   int
	ExerciseID  string
	TargetReps  int
	ActualReps  int
	Weight      float32
	FormScore   *float32
	RPE         float32
	CameraAngle string
	Reps        []vo.RepLog
}

// LogWorkoutSetResult returns result of set logging.
type LogWorkoutSetResult struct {
	SetLogID string
}

// LogWorkoutSetHandler handles set logging.
type LogWorkoutSetHandler struct {
	sessionRepo repository.WorkoutSessionRepository
	outbox      port.OutboxWriter
	txManager   port.TxManager
}

// NewLogWorkoutSetHandler constructs LogWorkoutSetHandler.
func NewLogWorkoutSetHandler(
	sessionRepo repository.WorkoutSessionRepository,
	outbox port.OutboxWriter,
	txManager port.TxManager,
) *LogWorkoutSetHandler {
	return &LogWorkoutSetHandler{
		sessionRepo: sessionRepo,
		outbox:      outbox,
		txManager:   txManager,
	}
}

// Handle executes LogWorkoutSet.
func (h *LogWorkoutSetHandler) Handle(ctx context.Context, cmd LogWorkoutSetCommand) (*LogWorkoutSetResult, error) {
	if cmd.SessionID == "" || cmd.ExerciseID == "" {
		return nil, apperror.ErrInvalidInput
	}

	setLogID := fmt.Sprintf("set-%s-%s-%d", cmd.SessionID, cmd.ExerciseID, cmd.SetNumber)
	setLog := aggregate.WorkoutSetLog{
		ID:          setLogID,
		SessionID:   cmd.SessionID,
		SetNumber:   cmd.SetNumber,
		ExerciseID:  cmd.ExerciseID,
		TargetReps:  cmd.TargetReps,
		ActualReps:  cmd.ActualReps,
		Weight:      cmd.Weight,
		FormScore:   cmd.FormScore,
		RPE:         cmd.RPE,
		CameraAngle: cmd.CameraAngle,
		Reps:        cmd.Reps,
		CreatedAt:   time.Now().UTC(),
	}

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

		if err := session.LogSet(setLog); err != nil {
			return err
		}

		if err := h.sessionRepo.Save(txCtx, session); err != nil {
			return fmt.Errorf("failed to save set log: %w", err)
		}

		events := session.PopEvents()
		if len(events) > 0 && h.outbox != nil {
			if err := h.outbox.WriteEvents(txCtx, "WorkoutSession", session.ID(), events); err != nil {
				return fmt.Errorf("failed to write outbox events: %w", err)
			}
		}

		return nil
	}

	const maxRetries = 3
	var lastErr error

	for attempt := 0; attempt < maxRetries; attempt++ {
		var err error
		if h.txManager != nil {
			err = h.txManager.WithTransaction(ctx, saveFunc)
		} else {
			err = saveFunc(ctx)
		}

		if err == nil {
			return &LogWorkoutSetResult{
				SetLogID: setLogID,
			}, nil
		}

		if errors.Is(err, derror.ErrOptimisticLocking) {
			lastErr = err
			time.Sleep(time.Duration(attempt*20+10) * time.Millisecond)
			continue
		}

		return nil, err
	}

	return nil, fmt.Errorf("exceeded max retries on optimistic lock conflict: %w", lastErr)
}
