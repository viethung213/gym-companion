package command

import (
	"context"
	"fmt"

	"github.com/viethung213/gym-companion/internal/exercise/application/port"
	"github.com/viethung213/gym-companion/internal/exercise/domain"
	"github.com/viethung213/gym-companion/internal/shared/middleware"
)

type UpdateExerciseCommand struct {
	ID   string
	Info domain.Info
}

type UpdateExerciseHandler struct {
	repo  port.Repository
	clock port.Clock
}

func NewUpdateExerciseHandler(repo port.Repository, clock port.Clock) *UpdateExerciseHandler {
	return &UpdateExerciseHandler{
		repo:  repo,
		clock: clock,
	}
}

func (h *UpdateExerciseHandler) Handle(ctx context.Context, cmd UpdateExerciseCommand) (*domain.Exercise, error) {
	if _, err := middleware.RequireAdmin(ctx); err != nil {
		return nil, err
	}

	exercise, err := h.repo.FindByID(ctx, cmd.ID)
	if err != nil {
		return nil, fmt.Errorf("find exercise: %w", err)
	}

	if err := exercise.UpdateInfo(cmd.Info, h.clock.Now()); err != nil {
		return nil, err
	}

	if err := h.repo.Save(ctx, exercise, nil); err != nil {
		return nil, fmt.Errorf("save exercise: %w", err)
	}

	return exercise, nil
}
