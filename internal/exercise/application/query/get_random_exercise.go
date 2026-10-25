package query

import (
	"context"
	"fmt"

	"github.com/viethung213/gym-companion/internal/exercise/application/port"
	"github.com/viethung213/gym-companion/internal/exercise/domain"
)

type GetRandomExerciseQuery struct{}

type GetRandomExerciseHandler struct {
	repo port.Repository
}

func NewGetRandomExerciseHandler(repo port.Repository) *GetRandomExerciseHandler {
	return &GetRandomExerciseHandler{
		repo: repo,
	}
}

func (h *GetRandomExerciseHandler) Handle(
	ctx context.Context,
	_ GetRandomExerciseQuery,
) (*domain.Exercise, error) {
	exercise, err := h.repo.GetRandomAISupported(ctx)
	if err != nil {
		return nil, fmt.Errorf("get random ai exercise: %w", err)
	}

	return exercise, nil
}
