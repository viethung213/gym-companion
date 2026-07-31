package command

import (
	"context"
	"fmt"
	"strings"

	"github.com/viethung213/gym-companion/internal/exercise/application/port"
	"github.com/viethung213/gym-companion/internal/exercise/domain"
	"github.com/viethung213/gym-companion/internal/shared/middleware"
)

type UpdateEquipmentCommand struct {
	ID   string
	Name string
}

type UpdateEquipmentHandler struct {
	repo port.Repository
}

func NewUpdateEquipmentHandler(repo port.Repository) *UpdateEquipmentHandler {
	return &UpdateEquipmentHandler{repo: repo}
}

func (h *UpdateEquipmentHandler) Handle(ctx context.Context, cmd *UpdateEquipmentCommand) (*port.Equipment, error) {
	if _, err := middleware.RequireAdmin(ctx); err != nil {
		return nil, err
	}

	id := strings.TrimSpace(cmd.ID)
	if id == "" {
		return nil, domain.ErrInvalidEquipment
	}

	name := strings.TrimSpace(cmd.Name)
	if name == "" {
		return nil, domain.ErrInvalidEquipment
	}

	eq := &port.Equipment{ID: id, Name: name}
	if err := h.repo.UpdateEquipment(ctx, eq); err != nil {
		return nil, fmt.Errorf("update equipment: %w", err)
	}

	return eq, nil
}
