package command

import (
	"context"
	"errors"
	"fmt"

	"github.com/viethung213/gym-companion/internal/coaching/agent"
	"github.com/viethung213/gym-companion/internal/coaching/agent/contextbuilder"
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
	tx      port.TransactionManager
	repo    port.RoadmapRepository
	agent   agent.CoachAgent
	builder *contextbuilder.Builder
	guard   *guardrail.Engine
	outbox  port.OutboxWriter
	clock   port.Clock
}

// NewRegenerateScheduleHandler wires the handler.
func NewRegenerateScheduleHandler(
	tx port.TransactionManager,
	repo port.RoadmapRepository,
	agent agent.CoachAgent,
	builder *contextbuilder.Builder,
	guard *guardrail.Engine,
	outbox port.OutboxWriter,
	clock port.Clock,
) *RegenerateScheduleHandler {
	return &RegenerateScheduleHandler{tx: tx, repo: repo, agent: agent, builder: builder, guard: guard, outbox: outbox, clock: clock}
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

	cc, err := h.builder.Build(ctx, agent.FlowRegenerate, cmd.UserID, rm, now)
	if err != nil {
		return nil, err
	}

	ids := make([]string, 0, len(pending))
	for _, s := range pending {
		ids = append(ids, s.ID())
	}

	drafts, err := h.agent.RegeneratePending(ctx, cc, ids, nil)
	if err != nil {
		return nil, fmt.Errorf("agent regenerate: %w", err)
	}

	// Apply drafts in-place onto the aggregate.
	for _, draft := range drafts {
		if draft == nil {
			continue
		}
		s, ok := rm.FindSession(draft.SessionPlanID)
		if !ok {
			continue // Session may have been completed between load and generate — skip.
		}
		if s.Status() != roadmap.SessionPlanStatusPending {
			continue // Someone else finalized it — respect D3.
		}
		if rwErr := s.RewritePrescription(draft.Prescription, draft.TargetMuscleGroups, draft.Reasoning, now); rwErr != nil {
			return nil, fmt.Errorf("rewrite session %s: %w", draft.SessionPlanID, rwErr)
		}
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
			RoadmapID:  rm.ID(),
			UserID:     rm.UserID(),
			Reason:     cmd.Reason,
			AdjustedAt: now,
		}
		return h.outbox.Enqueue(txCtx, rm.UserID(), evt)
	})
	if err != nil {
		return nil, err
	}
	return &RegenerateScheduleResult{Roadmap: rm}, nil
}
