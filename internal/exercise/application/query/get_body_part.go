package query

import (
	"context"
	"fmt"
	"strings"

	"github.com/viethung213/gym-companion/internal/exercise/application/port"
	"github.com/viethung213/gym-companion/internal/exercise/domain"
	"github.com/viethung213/gym-companion/internal/shared/middleware"
)

type GetBodyPartQuery struct {
	ID string
}

type GetBodyPartHandler struct {
	repo port.Repository
}

func NewGetBodyPartHandler(repo port.Repository) *GetBodyPartHandler {
	return &GetBodyPartHandler{repo: repo}
}

func (h *GetBodyPartHandler) Handle(ctx context.Context, q GetBodyPartQuery) (*port.BodyPart, error) {
	if _, err := middleware.RequireAuthenticated(ctx); err != nil {
		return nil, err
	}

	id := strings.TrimSpace(q.ID)
	if id == "" {
		return nil, domain.ErrBodyPartNotFound
	}

	bp, err := h.repo.GetBodyPart(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get body part: %w", err)
	}

	return bp, nil
}
