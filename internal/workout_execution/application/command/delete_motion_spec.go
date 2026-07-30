package command

import (
	"context"
	"fmt"

	"github.com/viethung213/gym-companion/internal/workout_execution/application/apperror"
	"github.com/viethung213/gym-companion/internal/workout_execution/domain/repository"
)

// DeleteMotionSpecificationCommand contains parameters to delete a MotionSpecification.
type DeleteMotionSpecificationCommand struct {
	ExerciseID string
}

// DeleteMotionSpecificationHandler handles deleting a MotionSpecification.
type DeleteMotionSpecificationHandler struct {
	motionRepo repository.MotionSpecificationRepository
}

// NewDeleteMotionSpecificationHandler constructs the handler.
func NewDeleteMotionSpecificationHandler(
	motionRepo repository.MotionSpecificationRepository,
) *DeleteMotionSpecificationHandler {
	return &DeleteMotionSpecificationHandler{
		motionRepo: motionRepo,
	}
}

// Handle executes the DeleteMotionSpecification command.
func (h *DeleteMotionSpecificationHandler) Handle(
	ctx context.Context,
	cmd DeleteMotionSpecificationCommand,
) error {
	if cmd.ExerciseID == "" {
		return fmt.Errorf("delete motion spec: %w: exercise_id is required", apperror.ErrInvalidInput)
	}

	if err := h.motionRepo.Delete(ctx, cmd.ExerciseID); err != nil {
		return fmt.Errorf("delete motion spec: %w", err)
	}

	return nil
}
