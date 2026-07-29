// Package query holds read-only handlers.
package query

import (
	"context"
	"errors"

	"github.com/viethung213/gym-companion/internal/coaching/application/port"
	"github.com/viethung213/gym-companion/internal/coaching/domain/roadmap"
)

// GetActiveRoadmapQuery is the input to the corresponding query.

type GetActiveRoadmapQuery struct{ UserID string }

// GetRoadmapQuery is the input to GetRoadmap.

type GetRoadmapQuery struct {
	UserID string

	RoadmapID string
}

// ListRoadmapsQuery filters user roadmaps.

type ListRoadmapsQuery struct {
	UserID string

	Status roadmap.Status // optional

	Limit int

	Offset int
}

// GetSessionPlanQuery is the input to GetSessionPlan.

type GetSessionPlanQuery struct {
	UserID string

	SessionPlanID string
}

// Handlers exposes all read-side operations.

type Handlers struct {
	repo port.RoadmapRepository
}

// NewHandlers wires the query bundle.

func NewHandlers(repo port.RoadmapRepository) *Handlers {

	return &Handlers{repo: repo}

}

// GetActiveRoadmap returns the active roadmap for a user.

func (h *Handlers) GetActiveRoadmap(ctx context.Context, q GetActiveRoadmapQuery) (*roadmap.Roadmap, error) {

	if q.UserID == "" {

		return nil, errors.New("user_id required")

	}

	return h.repo.FindActiveByUser(ctx, q.UserID)

}

// GetRoadmap fetches a specific roadmap by id, verifying ownership.

func (h *Handlers) GetRoadmap(ctx context.Context, q GetRoadmapQuery) (*roadmap.Roadmap, error) {

	if q.UserID == "" || q.RoadmapID == "" {

		return nil, errors.New("user_id and roadmap_id required")

	}

	rm, err := h.repo.FindByID(ctx, q.RoadmapID)

	if err != nil {

		return nil, err

	}

	if rm.UserID() != q.UserID {

		return nil, roadmap.ErrRoadmapNotFound

	}

	return rm, nil

}

// ListRoadmaps returns paginated roadmaps.

func (h *Handlers) ListRoadmaps(ctx context.Context, q ListRoadmapsQuery) ([]*roadmap.Roadmap, error) {

	if q.UserID == "" {

		return nil, errors.New("user_id required")

	}

	return h.repo.ListByUser(ctx, q.UserID, q.Status, q.Limit, q.Offset)

}

// GetSessionPlan returns the session by id (with ownership check via parent roadmap).

func (h *Handlers) GetSessionPlan(ctx context.Context, q GetSessionPlanQuery) (*roadmap.SessionPlan, error) {

	if q.UserID == "" || q.SessionPlanID == "" {

		return nil, errors.New("user_id and session_plan_id required")

	}

	rm, err := h.repo.FindSessionByID(ctx, q.SessionPlanID)

	if err != nil {

		return nil, err

	}

	if rm.UserID() != q.UserID {

		return nil, roadmap.ErrRoadmapNotFound

	}

	s, ok := rm.FindSession(q.SessionPlanID)

	if !ok {

		return nil, roadmap.ErrSessionNotFound

	}

	return s, nil

}
