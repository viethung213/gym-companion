package command

import (
	"context"
	"fmt"
	"strings"

	"github.com/viethung213/gym-companion/internal/exercise/application/port"
	"github.com/viethung213/gym-companion/internal/exercise/domain"
	"github.com/viethung213/gym-companion/internal/shared/middleware"
)

type DeleteMuscleCommand struct {
	ID string
}

type DeleteMuscleHandler struct {
	repo port.Repository
}

func NewDeleteMuscleHandler(repo port.Repository) *DeleteMuscleHandler {
	return &DeleteMuscleHandler{repo: repo}
}

func (h *DeleteMuscleHandler) Handle(ctx context.Context, cmd *DeleteMuscleCommand) error {
	if _, err := middleware.RequireAdmin(ctx); err != nil {
		return err
	}

	id := strings.TrimSpace(cmd.ID)
	if id == "" {
		return domain.ErrInvalidMuscle
	}

	if err := h.repo.DeleteMuscle(ctx, id); err != nil {
		return fmt.Errorf("delete muscle: %w", err)
	}

	return nil
}
