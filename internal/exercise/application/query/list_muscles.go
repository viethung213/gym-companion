package query

import (
	"context"
	"fmt"

	"github.com/viethung213/gym-companion/internal/exercise/application/port"
	"github.com/viethung213/gym-companion/internal/shared/middleware"
)

type ListMusclesQuery struct {
	BodyPartID string
	Limit      int
	Offset     int
}

type ListMusclesHandler struct {
	repo port.Repository
}

func NewListMusclesHandler(repo port.Repository) *ListMusclesHandler {
	return &ListMusclesHandler{repo: repo}
}

func (h *ListMusclesHandler) Handle(ctx context.Context, q ListMusclesQuery) ([]port.Muscle, int, error) {
	if _, err := middleware.RequireAuthenticated(ctx); err != nil {
		return nil, 0, err
	}

	limit := q.Limit
	if limit <= 0 || limit > 100 {
		limit = 50
	}

	ms, total, err := h.repo.ListMuscles(ctx, q.BodyPartID, limit, q.Offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list muscles: %w", err)
	}

	return ms, total, nil
}
