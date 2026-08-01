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

// RegenerateScheduleCommand is the input to UC-02.3.
type RegenerateScheduleCommand struct {
	UserID string
	Reason string // "profile_updated", "injury_reported", "injury_recovered", ...
}

// RegenerateScheduleResult echoes the updated roadmap.
type RegenerateScheduleResult struct {
	Roadmap *roadmap.Roadmap
}

// RegenerateScheduleHandler orchestrates UC-02.3 (FR-AC-06).
type RegenerateScheduleHandler struct {
	tx     port.TransactionManager
	repo   port.RoadmapRepository
	agent  port.CoachAgent
	guard  *guardrail.Engine
	outbox port.OutboxWriter
	clock  port.Clock
}

// NewRegenerateScheduleHandler wires the handler.
func NewRegenerateScheduleHandler(
	tx port.TransactionManager,
	repo port.RoadmapRepository,
	agent port.CoachAgent,
	guard *guardrail.Engine,
	outbox port.OutboxWriter,
	clock port.Clock,
) *RegenerateScheduleHandler {
	return &RegenerateScheduleHandler{
		tx:     tx,
		repo:   repo,
		agent:  agent,
		guard:  guard,
		outbox: outbox,
		clock:  clock,
	}
}

// Handle executes UC-02.3: rewrite only PENDING sessions from today onward (D3).
func (h *RegenerateScheduleHandler) Handle(ctx context.Context, cmd RegenerateScheduleCommand) (*RegenerateScheduleResult, error) {
	if cmd.UserID == "" {
		return nil, errors.New("user_id required")
	}

	now := h.clock.Now()

	// Load current active roadmap.

	rm, err := h.repo.FindActiveByUser(ctx, cmd.UserID)

	if err != nil {
		return nil, err
	}

	pending := rm.PendingSessionsFrom(now)

	if len(pending) == 0 {
		// Nothing to regenerate.

		return &RegenerateScheduleResult{Roadmap: rm}, nil
	}

	ids := make([]string, 0, len(pending))

	for _, s := range pending {
		ids = append(ids, s.ID())
	}

	drafts, err := h.agent.RegeneratePending(ctx, cmd.UserID, ids)

	if err != nil {
		return nil, fmt.Errorf("agent regenerate: %w", err)
	}

	// The agent returns one draft per pending session, in the same order.

	if len(drafts) != len(pending) {
		return nil, fmt.Errorf("agent returned %d drafts for %d pending sessions", len(drafts), len(pending))
	}

	applied := 0

	for i, draft := range drafts {
		if draft == nil {
			continue
		}

		s := pending[i]

		if s.Status() != roadmap.SessionPlanStatusPending {
			continue // Someone else finalized it — respect D3.
		}

		if rwErr := s.RewritePrescription(draft.Prescription, draft.TargetMuscleGroups, draft.Reasoning, now); rwErr != nil {
			return nil, fmt.Errorf("rewrite session %s: %w", s.ID(), rwErr)
		}

		applied++
	}

	if applied == 0 {
		return nil, fmt.Errorf("no session was rewritten out of %d pending", len(pending))
	}

	if result := h.guard.Check(rm); result.Status != guardrail.StatusApproved {
		return nil, fmt.Errorf("guardrail rejected regen: %+v", result.Violations)
	}

	err = h.tx.WithTransaction(ctx, func(txCtx context.Context) error {
		rm.Touch(now)

		if saveErr := h.repo.Save(txCtx, rm); saveErr != nil {
			return saveErr
		}

		evt := &domainevent.RoadmapAdjusted{
			RoadmapID: rm.ID(),

			UserID: rm.UserID(),

			Reason: cmd.Reason,

			AdjustedAt: now,
		}

		return h.outbox.Enqueue(txCtx, rm.UserID(), evt)
	})

	if err != nil {
		return nil, err
	}

	return &RegenerateScheduleResult{Roadmap: rm}, nil
}
