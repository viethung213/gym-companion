package command

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/viethung213/gym-companion/internal/workout_execution/application/apperror"
	"github.com/viethung213/gym-companion/internal/workout_execution/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/workout_execution/domain/derror"
	"github.com/viethung213/gym-companion/internal/workout_execution/domain/repository"
	"github.com/viethung213/gym-companion/internal/workout_execution/domain/vo"
)

// LogWorkoutSetCommand contains parameters to log a completed set.
type LogWorkoutSetCommand struct {
	SessionID   string
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
}

// NewLogWorkoutSetHandler constructs LogWorkoutSetHandler.
func NewLogWorkoutSetHandler(sessionRepo repository.WorkoutSessionRepository) *LogWorkoutSetHandler {
	return &LogWorkoutSetHandler{
		sessionRepo: sessionRepo,
	}
}

// Handle executes LogWorkoutSet.
func (h *LogWorkoutSetHandler) Handle(ctx context.Context, cmd LogWorkoutSetCommand) (*LogWorkoutSetResult, error) {
	if cmd.SessionID == "" || cmd.ExerciseID == "" {
		return nil, apperror.ErrInvalidInput
	}

	session, err := h.sessionRepo.FindByID(ctx, cmd.SessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to find session: %w", err)
	}
	if session == nil {
		return nil, derror.ErrWorkoutSessionNotFound
	}

	setLogID := uuid.NewString()
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

	if err := session.LogSet(setLog); err != nil {
		return nil, err
	}

	if err := h.sessionRepo.Save(ctx, session); err != nil {
		return nil, fmt.Errorf("failed to save set log: %w", err)
	}

	return &LogWorkoutSetResult{
		SetLogID: setLogID,
	}, nil
}
