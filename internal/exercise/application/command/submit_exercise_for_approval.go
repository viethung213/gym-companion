package command

import (
	"context"
	"fmt"

	"github.com/viethung213/gym-companion/internal/exercise/application/port"
	"github.com/viethung213/gym-companion/internal/exercise/domain"
	"github.com/viethung213/gym-companion/internal/shared/middleware"
)

type SubmitExerciseForApprovalCommand struct {
	ID string
}

type SubmitExerciseForApprovalHandler struct {
	repo  port.Repository
	clock port.Clock
	ids   port.IDGenerator
}

func NewSubmitExerciseForApprovalHandler(
	repo port.Repository,
	clock port.Clock,
	ids port.IDGenerator,
) *SubmitExerciseForApprovalHandler {
	return &SubmitExerciseForApprovalHandler{
		repo:  repo,
		clock: clock,
		ids:   ids,
	}
}

func (h *SubmitExerciseForApprovalHandler) Handle(
	ctx context.Context,
	cmd SubmitExerciseForApprovalCommand,
) (*domain.Exercise, error) {
	if _, err := middleware.RequireAdmin(ctx); err != nil {
		return nil, err
	}

	exercise, err := h.repo.FindByID(ctx, cmd.ID)
	if err != nil {
		return nil, fmt.Errorf("find exercise: %w", err)
	}

	now := h.clock.Now()
	if submitErr := exercise.SubmitForApproval(now); submitErr != nil {
		return nil, submitErr
	}

	var event *domain.Event
	event, err = newEvent(h.ids, domain.EventTypeExerciseSubmittedForApproval, exercise, now)
	if err != nil {
		return nil, err
	}

	if err = h.repo.Save(ctx, exercise, event); err != nil {
		return nil, fmt.Errorf("save exercise: %w", err)
	}

	return exercise, nil
}
