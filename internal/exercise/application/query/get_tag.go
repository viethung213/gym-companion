package query

import (
	"context"
	"fmt"
	"strings"

	"github.com/viethung213/gym-companion/internal/exercise/application/port"
	"github.com/viethung213/gym-companion/internal/exercise/domain"
	"github.com/viethung213/gym-companion/internal/shared/middleware"
)

type GetTagQuery struct {
	ID string
}

type GetTagHandler struct {
	repo port.Repository
}

func NewGetTagHandler(repo port.Repository) *GetTagHandler {
	return &GetTagHandler{repo: repo}
}

func (h *GetTagHandler) Handle(ctx context.Context, q GetTagQuery) (*port.Tag, error) {
	if _, err := middleware.RequireAuthenticated(ctx); err != nil {
		return nil, err
	}

	id := strings.TrimSpace(q.ID)
	if id == "" {
		return nil, domain.ErrTagNotFound
	}

	t, err := h.repo.GetTag(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get tag: %w", err)
	}

	return t, nil
}
