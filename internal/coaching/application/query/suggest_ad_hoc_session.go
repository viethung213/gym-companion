package query

import (
	"context"
	"errors"
	"time"

	"github.com/viethung213/gym-companion/internal/coaching/agent"
	"github.com/viethung213/gym-companion/internal/coaching/agent/contextbuilder"
	"github.com/viethung213/gym-companion/internal/coaching/application/port"
	"github.com/viethung213/gym-companion/internal/coaching/domain/roadmap"
)

// SuggestAdHocSessionQuery is the read-only input for Flow 5.
type SuggestAdHocSessionQuery struct {
	UserID string
	Hint   agent.AdHocHint
}

// SuggestAdHocSessionHandler runs the ad-hoc session recommendation flow.
// Read-only: no roadmap mutation, no outbox event, no transaction.
// The frontend decides what to do with the returned suggestion.
type SuggestAdHocSessionHandler struct {
	repo    port.RoadmapRepository
	agent   agent.CoachAgent
	builder *contextbuilder.Builder
	clock   port.Clock
}

// NewSuggestAdHocSessionHandler wires the handler.
func NewSuggestAdHocSessionHandler(
	repo port.RoadmapRepository,
	agent agent.CoachAgent,
	builder *contextbuilder.Builder,
	clock port.Clock,
) *SuggestAdHocSessionHandler {
	return &SuggestAdHocSessionHandler{repo: repo, agent: agent, builder: builder, clock: clock}
}

// Handle returns a SuggestedSession for the user. If the user has no active
// roadmap, the suggestion still works — it just uses Accumulation defaults.
func (h *SuggestAdHocSessionHandler) Handle(ctx context.Context, q *SuggestAdHocSessionQuery) (agent.SuggestedSession, error) {
	if q == nil || q.UserID == "" {
		return agent.SuggestedSession{}, errors.New("user_id required")
	}
	now := h.clock.Now()

	// Load active roadmap if any (drives phase / injury awareness).
	var active *roadmap.Roadmap
	rm, err := h.repo.FindActiveByUser(ctx, q.UserID)
	if err == nil {
		active = rm
	} else if !errors.Is(err, roadmap.ErrRoadmapNotFound) {
		return agent.SuggestedSession{}, err
	}

	cc, err := h.builder.Build(ctx, agent.FlowSuggestAdHocSession, q.UserID, active, now)
	if err != nil {
		return agent.SuggestedSession{}, err
	}

	// If the hint overrides equipment, patch the CoachContext copy.
	if len(q.Hint.AvailableEquipment) > 0 {
		cc.Profile.AvailableEquipment = q.Hint.AvailableEquipment
	}

	return h.agent.SuggestAdHocSession(ctx, cc, q.Hint)
}

// Compile-time no-op to hint the reader that this handler is read-only.
var _ = time.Now
