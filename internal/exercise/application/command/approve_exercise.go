package command

import (
	"context"
	"fmt"

	"github.com/viethung213/gym-companion/internal/exercise/application/port"
	"github.com/viethung213/gym-companion/internal/exercise/domain"
	"github.com/viethung213/gym-companion/internal/shared/middleware"
)

type ApproveExerciseCommand struct {
	ID string
}

type ApproveExerciseHandler struct {
	repo  port.Repository
	clock port.Clock
	ids   port.IDGenerator
}

func NewApproveExerciseHandler(repo port.Repository, clock port.Clock, ids port.IDGenerator) *ApproveExerciseHandler {
	return &ApproveExerciseHandler{
		repo:  repo,
		clock: clock,
		ids:   ids,
	}
}

func (h *ApproveExerciseHandler) Handle(ctx context.Context, cmd ApproveExerciseCommand) (*domain.Exercise, error) {
	if _, err := middleware.RequireAdmin(ctx); err != nil {
		return nil, err
	}

	exercise, err := h.repo.FindByID(ctx, cmd.ID)
	if err != nil {
		return nil, fmt.Errorf("find exercise: %w", err)
	}

	now := h.clock.Now()
	if err := exercise.Approve(now); err != nil {
		return nil, err
	}

	event, err := newEvent(h.ids, domain.EventTypeExerciseApproved, exercise, now)
	if err != nil {
		return nil, err
	}

	if err := h.repo.Save(ctx, exercise, event); err != nil {
		return nil, fmt.Errorf("save exercise: %w", err)
	}

	return exercise, nil
}
