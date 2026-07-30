// Package command holds application command handlers for the Coaching context.
package command

import (
	"context"
	"errors"
	"fmt"

	"github.com/viethung213/gym-companion/internal/coaching/application/port"
	domainevent "github.com/viethung213/gym-companion/internal/coaching/domain/event"
	"github.com/viethung213/gym-companion/internal/coaching/domain/guardrail"
	"github.com/viethung213/gym-companion/internal/coaching/domain/roadmap"
)

// InitiateRoadmapCommand is the input to UC-02.1.
type InitiateRoadmapCommand struct {
	UserID string
}

// InitiateRoadmapResult wraps the persisted roadmap snapshot.
type InitiateRoadmapResult struct {
	Roadmap *roadmap.Roadmap
}

// InitiateRoadmapHandler orchestrates: ContextBuilder → CoachAgent → Guard →
// Save (Roadmap + Outbox event) in a single transaction.
type InitiateRoadmapHandler struct {
	tx     port.TransactionManager
	repo   port.RoadmapRepository
	agent  port.CoachAgent
	guard  *guardrail.Engine
	outbox port.OutboxWriter
	clock  port.Clock
}

// NewInitiateRoadmapHandler wires the handler.
func NewInitiateRoadmapHandler(
	tx port.TransactionManager,
	repo port.RoadmapRepository,
	agent port.CoachAgent,
	guard *guardrail.Engine,
	outbox port.OutboxWriter,
	clock port.Clock,
) *InitiateRoadmapHandler {
	return &InitiateRoadmapHandler{
		tx:     tx,
		repo:   repo,
		agent:  agent,
		guard:  guard,
		outbox: outbox,
		clock:  clock,
	}
}

// Handle executes UC-02.1.
func (h *InitiateRoadmapHandler) Handle(ctx context.Context, cmd InitiateRoadmapCommand) (*InitiateRoadmapResult, error) {
	if cmd.UserID == "" {
		return nil, errors.New("user_id is required")
	}

	// Reject if an ACTIVE roadmap already exists (BR-AC constraint enforced also
	// at DB level via ux_roadmaps_user_active).

	existing, err := h.repo.FindActiveByUser(ctx, cmd.UserID)

	if err != nil && !errors.Is(err, roadmap.ErrRoadmapNotFound) {
		return nil, fmt.Errorf("check active roadmap: %w", err)
	}

	if existing != nil {
		return nil, roadmap.ErrActiveRoadmapExists
	}

	now := h.clock.Now()

	draft, err := h.agent.GenerateRoadmap(ctx, cmd.UserID)

	if err != nil {
		return nil, fmt.Errorf("agent generate: %w", err)
	}

	if valErr := draft.ValidateFullStructure(); valErr != nil {
		return nil, fmt.Errorf("draft structure: %w", valErr)
	}

	if result := h.guard.Check(draft); result.Status != guardrail.StatusApproved {
		return nil, fmt.Errorf("guardrail rejected: %+v", result.Violations)
	}

	var out *InitiateRoadmapResult

	err = h.tx.WithTransaction(ctx, func(txCtx context.Context) error {
		if saveErr := h.repo.Save(txCtx, draft); saveErr != nil {
			return saveErr
		}

		evt := &domainevent.RoadmapInitiated{
			RoadmapID:   draft.ID(),
			UserID:      draft.UserID(),
			InitiatedAt: now,
		}

		if enqErr := h.outbox.Enqueue(txCtx, draft.UserID(), evt); enqErr != nil {
			return enqErr
		}

		out = &InitiateRoadmapResult{Roadmap: draft}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return out, nil
}
