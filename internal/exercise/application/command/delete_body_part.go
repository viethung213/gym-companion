package command

import (
	"context"
	"fmt"
	"strings"

	"github.com/viethung213/gym-companion/internal/exercise/application/port"
	"github.com/viethung213/gym-companion/internal/exercise/domain"
	"github.com/viethung213/gym-companion/internal/shared/middleware"
)

type DeleteBodyPartCommand struct {
	ID string
}

type DeleteBodyPartHandler struct {
	repo port.Repository
}

func NewDeleteBodyPartHandler(repo port.Repository) *DeleteBodyPartHandler {
	return &DeleteBodyPartHandler{repo: repo}
}

func (h *DeleteBodyPartHandler) Handle(ctx context.Context, cmd *DeleteBodyPartCommand) error {
	if _, err := middleware.RequireAdmin(ctx); err != nil {
		return err
	}

	id := strings.TrimSpace(cmd.ID)
	if id == "" {
		return domain.ErrInvalidBodyPart
	}

	if err := h.repo.DeleteBodyPart(ctx, id); err != nil {
		return fmt.Errorf("delete body part: %w", err)
	}

	return nil
}
