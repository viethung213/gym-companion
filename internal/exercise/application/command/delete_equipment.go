package command

import (
	"context"
	"fmt"
	"strings"

	"github.com/viethung213/gym-companion/internal/exercise/application/port"
	"github.com/viethung213/gym-companion/internal/exercise/domain"
	"github.com/viethung213/gym-companion/internal/shared/middleware"
)

type DeleteEquipmentCommand struct {
	ID string
}

type DeleteEquipmentHandler struct {
	repo port.Repository
}

func NewDeleteEquipmentHandler(repo port.Repository) *DeleteEquipmentHandler {
	return &DeleteEquipmentHandler{repo: repo}
}

func (h *DeleteEquipmentHandler) Handle(ctx context.Context, cmd *DeleteEquipmentCommand) error {
	if _, err := middleware.RequireAdmin(ctx); err != nil {
		return err
	}

	id := strings.TrimSpace(cmd.ID)
	if id == "" {
		return domain.ErrInvalidEquipment
	}

	if err := h.repo.DeleteEquipment(ctx, id); err != nil {
		return fmt.Errorf("delete equipment: %w", err)
	}

	return nil
}
