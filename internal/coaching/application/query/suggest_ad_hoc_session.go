package query

import (
	"context"
	"errors"

	"github.com/viethung213/gym-companion/internal/coaching/application/port"
)

// SuggestAdHocSessionQuery is the read-only input for Flow 5.
type SuggestAdHocSessionQuery struct {
	UserID string
	Hint   port.AdHocHint
}

// SuggestAdHocSessionHandler runs the ad-hoc session recommendation flow.
type SuggestAdHocSessionHandler struct {
	repo  port.RoadmapRepository
	agent port.CoachAgent
	clock port.Clock
}

// NewSuggestAdHocSessionHandler wires the handler.
func NewSuggestAdHocSessionHandler(
	repo port.RoadmapRepository,
	agent port.CoachAgent,
	clock port.Clock,
) *SuggestAdHocSessionHandler {
	return &SuggestAdHocSessionHandler{repo: repo, agent: agent, clock: clock}
}

// Handle returns a SuggestedSession for the user.
func (h *SuggestAdHocSessionHandler) Handle(ctx context.Context, q *SuggestAdHocSessionQuery) (port.SuggestedSession, error) {
	if q == nil || q.UserID == "" {
		return port.SuggestedSession{}, errors.New("user_id required")
	}

	return h.agent.SuggestAdHocSession(ctx, q.UserID, q.Hint)
}
