package query

import (
	"context"
	"fmt"

	"github.com/viethung213/gym-companion/internal/workout_execution/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/workout_execution/domain/repository"
)

type SearchMotionSpecificationsQuery struct {
	Keyword string
	Limit   int
	Offset  int
}

type SearchMotionSpecificationsQueryResult struct {
	Items      []*aggregate.MotionSpecification
	TotalCount int
}

type SearchMotionSpecificationsQueryHandler struct {
	motionRepo repository.MotionSpecificationRepository
}

func NewSearchMotionSpecificationsQueryHandler(
	motionRepo repository.MotionSpecificationRepository,
) *SearchMotionSpecificationsQueryHandler {
	return &SearchMotionSpecificationsQueryHandler{
		motionRepo: motionRepo,
	}
}

func (h *SearchMotionSpecificationsQueryHandler) Handle(
	ctx context.Context,
	q SearchMotionSpecificationsQuery,
) (*SearchMotionSpecificationsQueryResult, error) {
	limit := q.Limit
	if limit <= 0 {
		limit = 20
	}

	offset := q.Offset
	if offset < 0 {
		offset = 0
	}

	items, total, err := h.motionRepo.Search(ctx, q.Keyword, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("search motion specs query: %w", err)
	}

	return &SearchMotionSpecificationsQueryResult{
		Items:      items,
		TotalCount: total,
	}, nil
}
