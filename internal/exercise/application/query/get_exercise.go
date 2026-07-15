package query

import (
	"context"
	"fmt"

	"github.com/viethung213/gym-companion/internal/exercise/application/port"
	"github.com/viethung213/gym-companion/internal/exercise/domain"
	"github.com/viethung213/gym-companion/internal/shared/middleware"
)

type GetExerciseQuery struct {
	ID string
}

type GetExerciseHandler struct {
	repo port.Repository
}

func NewGetExerciseHandler(repo port.Repository) *GetExerciseHandler {
	return &GetExerciseHandler{
		repo: repo,
	}
}

func (h *GetExerciseHandler) Handle(
	ctx context.Context,
	q GetExerciseQuery,
) (*domain.Exercise, error) {
	actor, err := middleware.RequireAuthenticated(ctx)
	if err != nil {
		return nil, err
	}

	exercise, err := h.repo.FindByID(ctx, q.ID)
	if err != nil {
		return nil, fmt.Errorf("find exercise: %w", err)
	}

	if exercise.Info().Status != domain.StatusActive && !actor.IsAdmin() {
		return nil, domain.ErrExerciseNotFound
	}

	return exercise, nil
}
