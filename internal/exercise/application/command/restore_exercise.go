package command

import (
	"context"
	"fmt"

	"github.com/viethung213/gym-companion/internal/exercise/application/port"
	"github.com/viethung213/gym-companion/internal/exercise/domain"
	"github.com/viethung213/gym-companion/internal/shared/middleware"
)

type RestoreExerciseCommand struct {
	ID string
}

type RestoreExerciseHandler struct {
	repo  port.Repository
	clock port.Clock
	ids   port.IDGenerator
}

func NewRestoreExerciseHandler(
	repo port.Repository,
	clock port.Clock,
	ids port.IDGenerator,
) *RestoreExerciseHandler {
	return &RestoreExerciseHandler{
		repo:  repo,
		clock: clock,
		ids:   ids,
	}
}

func (h *RestoreExerciseHandler) Handle(
	ctx context.Context,
	cmd RestoreExerciseCommand,
) (*domain.Exercise, error) {
	if _, err := middleware.RequireAdmin(ctx); err != nil {
		return nil, err
	}

	exercise, err := h.repo.FindByID(ctx, cmd.ID)
	if err != nil {
		return nil, fmt.Errorf("find exercise: %w", err)
	}
	if exercise == nil {
		return nil, domain.ErrExerciseNotFound
	}

	now := h.clock.Now()
	if restoreErr := exercise.Restore(now); restoreErr != nil {
		return nil, restoreErr
	}

	event, err := newEvent(h.ids, domain.EventTypeExerciseRestored, exercise, now)
	if err != nil {
		return nil, err
	}

	if err = h.repo.Save(ctx, exercise, event); err != nil {
		return nil, fmt.Errorf("save exercise: %w", err)
	}

	return exercise, nil
}
