package command

import (
	"context"
	"errors"
	"fmt"

	"github.com/viethung213/gym-companion/internal/workout_execution/application/apperror"
	"github.com/viethung213/gym-companion/internal/workout_execution/application/port"
	"github.com/viethung213/gym-companion/internal/workout_execution/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/workout_execution/domain/derror"
	"github.com/viethung213/gym-companion/internal/workout_execution/domain/repository"
)

// UpdateMotionSpecificationCommand contains parameters to update an existing MotionSpecification.
type UpdateMotionSpecificationCommand struct {
	ExerciseID             string
	OnnxDetectorURL        string
	OnnxSkeletonURL        string
	LocalRulesURL          string
	DialogueEngineURL      string
	RecommendedCameraAngle string
}

// UpdateMotionSpecificationHandler handles updating a MotionSpecification.
type UpdateMotionSpecificationHandler struct {
	motionRepo repository.MotionSpecificationRepository
	outbox     port.OutboxWriter
	txManager  port.TxManager
}

// NewUpdateMotionSpecificationHandler constructs the handler.
func NewUpdateMotionSpecificationHandler(
	motionRepo repository.MotionSpecificationRepository,
	outbox port.OutboxWriter,
	txManager port.TxManager,
) *UpdateMotionSpecificationHandler {
	return &UpdateMotionSpecificationHandler{
		motionRepo: motionRepo,
		outbox:     outbox,
		txManager:  txManager,
	}
}

// Handle executes the UpdateMotionSpecification command.
func (h *UpdateMotionSpecificationHandler) Handle(
	ctx context.Context,
	cmd UpdateMotionSpecificationCommand,
) (*aggregate.MotionSpecification, error) {
	if cmd.ExerciseID == "" {
		return nil, fmt.Errorf("update motion spec: %w: exercise_id is required", apperror.ErrInvalidInput)
	}

	spec, err := h.motionRepo.FindByExerciseID(ctx, cmd.ExerciseID)
	if err != nil {
		if errors.Is(err, derror.ErrNotFound) || errors.Is(err, derror.ErrMotionSpecNotFound) {
			return nil, fmt.Errorf("update motion spec: %w: exercise_id '%s'", derror.ErrMotionSpecNotFound, cmd.ExerciseID)
		}
		return nil, fmt.Errorf("update motion spec find: %w", err)
	}

	spec.UpdateSpec(
		cmd.OnnxDetectorURL,
		cmd.OnnxSkeletonURL,
		cmd.LocalRulesURL,
		cmd.DialogueEngineURL,
		cmd.RecommendedCameraAngle,
	)

	saveAndPublish := func(execCtx context.Context) error {
		if err := h.motionRepo.Save(execCtx, spec); err != nil {
			return err
		}
		events := spec.PopEvents()
		if len(events) > 0 && h.outbox != nil {
			if err := h.outbox.WriteEvents(execCtx, "MotionSpecification", spec.ExerciseID(), events); err != nil {
				return err
			}
		}
		return nil
	}

	var saveErr error
	if h.txManager != nil {
		saveErr = h.txManager.WithTransaction(ctx, saveAndPublish)
	} else {
		saveErr = saveAndPublish(ctx)
	}

	if saveErr != nil {
		return nil, fmt.Errorf("update motion spec save: %w", saveErr)
	}

	return spec, nil
}
