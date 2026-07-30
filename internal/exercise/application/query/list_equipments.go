package query

import (
	"context"
	"fmt"

	"github.com/viethung213/gym-companion/internal/exercise/application/port"
	"github.com/viethung213/gym-companion/internal/shared/middleware"
)

type ListEquipmentsQuery struct {
	Limit  int
	Offset int
}

type ListEquipmentsHandler struct {
	repo port.Repository
}

func NewListEquipmentsHandler(repo port.Repository) *ListEquipmentsHandler {
	return &ListEquipmentsHandler{repo: repo}
}

func (h *ListEquipmentsHandler) Handle(ctx context.Context, q ListEquipmentsQuery) ([]port.Equipment, int, error) {
	if _, err := middleware.RequireAuthenticated(ctx); err != nil {
		return nil, 0, err
	}

	limit := q.Limit
	if limit <= 0 || limit > 100 {
		limit = 50
	}

	eqs, total, err := h.repo.ListEquipments(ctx, limit, q.Offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list equipments: %w", err)
	}

	return eqs, total, nil
}
