package query

import (
	"context"
	"fmt"

	"github.com/viethung213/gym-companion/internal/exercise/application/port"
	"github.com/viethung213/gym-companion/internal/exercise/domain"
)

type SearchExercisesQuery struct {
	Filters *port.SearchFilters
}

type SearchExercisesResult struct {
	Exercises  []*domain.Exercise
	TotalCount int32
}

type SearchExercisesHandler struct {
	repo port.Repository
}

func NewSearchExercisesHandler(repo port.Repository) *SearchExercisesHandler {
	return &SearchExercisesHandler{
		repo: repo,
	}
}

func (h *SearchExercisesHandler) Handle(
	ctx context.Context,
	q SearchExercisesQuery,
) (*SearchExercisesResult, error) {
	exercises, totalCount, err := h.repo.SearchActive(ctx, q.Filters)
	if err != nil {
		return nil, fmt.Errorf("search exercises: %w", err)
	}

	return &SearchExercisesResult{
		Exercises:  exercises,
		TotalCount: totalCount,
	}, nil
}
