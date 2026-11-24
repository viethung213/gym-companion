package query

import (
	"context"
	"fmt"

	"github.com/viethung213/gym-companion/internal/exercise/application/port"
	"github.com/viethung213/gym-companion/internal/exercise/domain"
	"github.com/viethung213/gym-companion/internal/shared/middleware"
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
	// Unpublished exercises (draft, pending, archived) are an admin view, as in
	// GetExercise; everyone else searches ACTIVE exercises only.
	filters := q.Filters
	if actor, _ := middleware.RequireAuthenticated(ctx); filters != nil && !actor.IsAdmin() {
		public := *filters
		public.Status = ""
		public.IncludeAllStatuses = false
		filters = &public
	}

	exercises, totalCount, err := h.repo.SearchActive(ctx, filters)
	if err != nil {
		return nil, fmt.Errorf("search exercises: %w", err)
	}

	return &SearchExercisesResult{
		Exercises:  exercises,
		TotalCount: totalCount,
	}, nil
}
