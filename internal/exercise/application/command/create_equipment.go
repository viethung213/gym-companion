package command

import (
	"context"
	"fmt"
	"strings"

	"github.com/viethung213/gym-companion/internal/exercise/application/port"
	"github.com/viethung213/gym-companion/internal/exercise/domain"
	"github.com/viethung213/gym-companion/internal/shared/middleware"
)

type CreateEquipmentCommand struct {
	Name string
}

type CreateEquipmentHandler struct {
	repo port.Repository
	ids  port.IDGenerator
}

func NewCreateEquipmentHandler(repo port.Repository, ids port.IDGenerator) *CreateEquipmentHandler {
	return &CreateEquipmentHandler{repo: repo, ids: ids}
}

func (h *CreateEquipmentHandler) Handle(ctx context.Context, cmd *CreateEquipmentCommand) (*port.Equipment, error) {
	if _, err := middleware.RequireAdmin(ctx); err != nil {
		return nil, err
	}

	name := strings.TrimSpace(cmd.Name)
	if name == "" {
		return nil, domain.ErrInvalidEquipment
	}

	id, err := h.ids.NewID()
	if err != nil {
		return nil, fmt.Errorf("generate equipment id: %w", err)
	}

	eq := &port.Equipment{ID: id, Name: name}
	if err := h.repo.CreateEquipment(ctx, eq); err != nil {
		return nil, fmt.Errorf("create equipment: %w", err)
	}

	return eq, nil
}
