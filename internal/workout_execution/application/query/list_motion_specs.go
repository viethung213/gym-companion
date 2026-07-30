package query

import (
	"context"
	"fmt"

	"github.com/viethung213/gym-companion/internal/workout_execution/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/workout_execution/domain/repository"
)

// ListMotionSpecificationsQuery contains parameters for listing motion specs.

type ListMotionSpecificationsQuery struct {
	Limit int

	Offset int
}

// ListMotionSpecificationsQueryResult contains paginated motion specs.

type ListMotionSpecificationsQueryResult struct {
	Items []*aggregate.MotionSpecification

	TotalCount int
}

// ListMotionSpecificationsQueryHandler handles querying paginated MotionSpecifications.

type ListMotionSpecificationsQueryHandler struct {
	motionRepo repository.MotionSpecificationRepository
}

// NewListMotionSpecificationsQueryHandler constructs the handler.

func NewListMotionSpecificationsQueryHandler(

	motionRepo repository.MotionSpecificationRepository,

) *ListMotionSpecificationsQueryHandler {

	return &ListMotionSpecificationsQueryHandler{

		motionRepo: motionRepo,
	}

}

// Handle executes the ListMotionSpecifications query.

func (h *ListMotionSpecificationsQueryHandler) Handle(

	ctx context.Context,

	q ListMotionSpecificationsQuery,

) (*ListMotionSpecificationsQueryResult, error) {

	limit := q.Limit

	if limit <= 0 {

		limit = 20

	}

	offset := q.Offset

	if offset < 0 {

		offset = 0

	}

	items, total, err := h.motionRepo.List(ctx, limit, offset)

	if err != nil {

		return nil, fmt.Errorf("list motion specs query: %w", err)

	}

	return &ListMotionSpecificationsQueryResult{

		Items: items,

		TotalCount: total,
	}, nil

}
