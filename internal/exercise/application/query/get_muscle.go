package query

import (
	"context"
	"fmt"
	"strings"

	"github.com/viethung213/gym-companion/internal/exercise/application/port"
	"github.com/viethung213/gym-companion/internal/exercise/domain"
	"github.com/viethung213/gym-companion/internal/shared/middleware"
)

type GetMuscleQuery struct {
	ID string
}

type GetMuscleHandler struct {
	repo port.Repository
}

func NewGetMuscleHandler(repo port.Repository) *GetMuscleHandler {
	return &GetMuscleHandler{repo: repo}
}

func (h *GetMuscleHandler) Handle(ctx context.Context, q GetMuscleQuery) (*port.Muscle, error) {
	if _, err := middleware.RequireAuthenticated(ctx); err != nil {
		return nil, err
	}

	id := strings.TrimSpace(q.ID)
	if id == "" {
		return nil, domain.ErrMuscleNotFound
	}

	m, err := h.repo.GetMuscle(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get muscle: %w", err)
	}

	return m, nil
}
