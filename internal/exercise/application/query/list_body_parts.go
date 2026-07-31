package query

import (
	"context"
	"fmt"

	"github.com/viethung213/gym-companion/internal/exercise/application/port"
	"github.com/viethung213/gym-companion/internal/shared/middleware"
)

type ListBodyPartsQuery struct {
	Limit  int
	Offset int
}

type ListBodyPartsHandler struct {
	repo port.Repository
}

func NewListBodyPartsHandler(repo port.Repository) *ListBodyPartsHandler {
	return &ListBodyPartsHandler{repo: repo}
}

func (h *ListBodyPartsHandler) Handle(ctx context.Context, q ListBodyPartsQuery) ([]port.BodyPart, int, error) {
	if _, err := middleware.RequireAuthenticated(ctx); err != nil {
		return nil, 0, err
	}

	limit := q.Limit
	if limit <= 0 || limit > 100 {
		limit = 50
	}

	bps, total, err := h.repo.ListBodyParts(ctx, limit, q.Offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list body parts: %w", err)
	}

	return bps, total, nil
}
