package command

import (
	"context"
	"fmt"

	"github.com/viethung213/gym-companion/internal/exercise/application/port"
	"github.com/viethung213/gym-companion/internal/exercise/domain"
	"github.com/viethung213/gym-companion/internal/shared/middleware"
)

type ArchiveExerciseCommand struct {
	ID string
}

type ArchiveExerciseHandler struct {
	repo  port.Repository
	clock port.Clock
	ids   port.IDGenerator
}

func NewArchiveExerciseHandler(repo port.Repository, clock port.Clock, ids port.IDGenerator) *ArchiveExerciseHandler {
	return &ArchiveExerciseHandler{
		repo:  repo,
		clock: clock,
		ids:   ids,
	}
}

func (h *ArchiveExerciseHandler) Handle(ctx context.Context, cmd ArchiveExerciseCommand) error {
	if _, err := middleware.RequireAdmin(ctx); err != nil {
		return err
	}

	exercise, err := h.repo.FindByID(ctx, cmd.ID)
	if err != nil {
		return fmt.Errorf("find exercise: %w", err)
	}

	now := h.clock.Now()
	if err := exercise.Archive(now); err != nil {
		return err
	}

	event, err := newEvent(h.ids, domain.EventTypeExerciseArchived, exercise, now)
	if err != nil {
		return err
	}

	if err := h.repo.Save(ctx, exercise, event); err != nil {
		return fmt.Errorf("save exercise: %w", err)
	}

	return nil
}
