package query

import (
	"context"
	"fmt"
	"strings"

	"github.com/viethung213/gym-companion/internal/exercise/application/port"
	"github.com/viethung213/gym-companion/internal/exercise/domain"
	"github.com/viethung213/gym-companion/internal/shared/middleware"
)

type GetEquipmentQuery struct {
	ID string
}

type GetEquipmentHandler struct {
	repo port.Repository
}

func NewGetEquipmentHandler(repo port.Repository) *GetEquipmentHandler {
	return &GetEquipmentHandler{repo: repo}
}

func (h *GetEquipmentHandler) Handle(ctx context.Context, q GetEquipmentQuery) (*port.Equipment, error) {
	if _, err := middleware.RequireAuthenticated(ctx); err != nil {
		return nil, err
	}

	id := strings.TrimSpace(q.ID)
	if id == "" {
		return nil, domain.ErrEquipmentNotFound
	}

	eq, err := h.repo.GetEquipment(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get equipment: %w", err)
	}

	return eq, nil
}
