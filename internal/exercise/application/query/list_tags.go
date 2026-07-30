package query

import (
	"context"
	"fmt"

	"github.com/viethung213/gym-companion/internal/exercise/application/port"
	"github.com/viethung213/gym-companion/internal/shared/middleware"
)

type ListTagsQuery struct {
	Limit  int
	Offset int
}

type ListTagsHandler struct {
	repo port.Repository
}

func NewListTagsHandler(repo port.Repository) *ListTagsHandler {
	return &ListTagsHandler{repo: repo}
}

func (h *ListTagsHandler) Handle(ctx context.Context, q ListTagsQuery) ([]port.Tag, int, error) {
	if _, err := middleware.RequireAuthenticated(ctx); err != nil {
		return nil, 0, err
	}

	limit := q.Limit
	if limit <= 0 || limit > 100 {
		limit = 50
	}

	ts, total, err := h.repo.ListTags(ctx, limit, q.Offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list tags: %w", err)
	}

	return ts, total, nil
}
