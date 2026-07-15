package command

import (
	"context"
	"fmt"

	"github.com/viethung213/gym-companion/internal/exercise/application/port"
	"github.com/viethung213/gym-companion/internal/exercise/domain"
	"github.com/viethung213/gym-companion/internal/shared/middleware"
)

type CreateExerciseCommand struct {
	Info domain.Info
}

type CreateExerciseHandler struct {
	repo  port.Repository
	clock port.Clock
	ids   port.IDGenerator
}

func NewCreateExerciseHandler(
	repo port.Repository,
	clock port.Clock,
	ids port.IDGenerator,
) *CreateExerciseHandler {
	return &CreateExerciseHandler{
		repo:  repo,
		clock: clock,
		ids:   ids,
	}
}

func (h *CreateExerciseHandler) Handle(
	ctx context.Context,
	cmd *CreateExerciseCommand,
) (*domain.Exercise, error) {
	if _, err := middleware.RequireAdmin(ctx); err != nil {
		return nil, err
	}

	id, err := h.ids.NewID()
	if err != nil {
		return nil, fmt.Errorf("generate exercise id: %w", err)
	}

	now := h.clock.Now()
	info := cmd.Info
	info.ID = id

	exercise, err := domain.NewExercise(info, now)
	if err != nil {
		return nil, err
	}

	var event *domain.Event
	event, err = newEvent(h.ids, domain.EventTypeExerciseCreated, exercise, now)
	if err != nil {
		return nil, err
	}

	if err = h.repo.Save(ctx, exercise, event); err != nil {
		return nil, fmt.Errorf("save exercise: %w", err)
	}

	return exercise, nil
}
