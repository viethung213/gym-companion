package command

import (
	"context"
	"errors"
	"fmt"

	"github.com/viethung213/gym-companion/internal/coaching/application/contextbuilder"
	"github.com/viethung213/gym-companion/internal/coaching/application/port"
	domainevent "github.com/viethung213/gym-companion/internal/coaching/domain/event"
	"github.com/viethung213/gym-companion/internal/coaching/domain/roadmap"
	"github.com/viethung213/gym-companion/internal/coaching/domain/service"
	"github.com/viethung213/gym-companion/internal/coaching/infrastructure/guardrail"
)

// ApplyAdaptiveCycleCommand triggers UC-04.1 (BR-AC-04) evaluation.
type ApplyAdaptiveCycleCommand struct {
	UserID  string
	Metrics []service.WeeklyMetrics
}

// ApplyAdaptiveCycleResult echoes the decision applied.
type ApplyAdaptiveCycleResult struct {
	Decision service.AdaptationDecision
	Roadmap  *roadmap.Roadmap
}

// ApplyAdaptiveCycleHandler orchestrates BR-AC-04 Trigger A.
type ApplyAdaptiveCycleHandler struct {
	tx      port.TransactionManager
	repo    port.RoadmapRepository
	agent   port.CoachAgent
	builder *contextbuilder.Builder
	guard   *guardrail.Engine
	outbox  port.OutboxWriter
	clock   port.Clock
	engine  *service.AdaptiveCoachEngine
}

// NewApplyAdaptiveCycleHandler wires the handler.
func NewApplyAdaptiveCycleHandler(
	tx port.TransactionManager,
	repo port.RoadmapRepository,
	agent port.CoachAgent,
	builder *contextbuilder.Builder,
	guard *guardrail.Engine,
	outbox port.OutboxWriter,
	clock port.Clock,
	engine *service.AdaptiveCoachEngine,
) *ApplyAdaptiveCycleHandler {
	if engine == nil {
		engine = service.NewAdaptiveCoachEngine()
	}
	return &ApplyAdaptiveCycleHandler{
		tx: tx, repo: repo, agent: agent, builder: builder,
		guard: guard, outbox: outbox, clock: clock, engine: engine,
	}
}

// Handle evaluates BR-AC-04 and applies the result via CoachAgent.Adapt.
func (h *ApplyAdaptiveCycleHandler) Handle(ctx context.Context, cmd ApplyAdaptiveCycleCommand) (*ApplyAdaptiveCycleResult, error) {
	if cmd.UserID == "" {
		return nil, errors.New("user_id required")
	}
	decision := h.engine.EvaluateTriggerA(cmd.Metrics)
	if decision.Kind == service.AdaptationNoOp {
		return &ApplyAdaptiveCycleResult{Decision: decision}, nil
	}

	rm, err := h.repo.FindActiveByUser(ctx, cmd.UserID)
	if err != nil {
		return nil, err
	}
	now := h.clock.Now()
	cc, err := h.builder.Build(ctx, port.FlowAdaptiveCycle, cmd.UserID, rm, now)
	if err != nil {
		return nil, err
	}

	drafts, err := h.agent.Adapt(ctx, cc, decision.Reason, nil)
	if err != nil {
		return nil, fmt.Errorf("agent adapt: %w", err)
	}

	// Apply drafts in-place onto pending sessions (D3 invariant).
	for _, d := range drafts {
		s, ok := rm.FindSession(d.SessionPlanID)
		if !ok {
			continue
		}
		if s.Status() != roadmap.SessionPlanStatusPending {
			continue
		}
		if rwErr := s.RewritePrescription(d.Prescription, d.TargetMuscleGroups, d.Reasoning, now); rwErr != nil {
			return nil, rwErr
		}
	}

	if result := h.guard.Check(rm); result.Status != guardrail.StatusApproved {
		return nil, fmt.Errorf("guardrail rejected adaptive cycle: %+v", result.Violations)
	}

	err = h.tx.WithTransaction(ctx, func(txCtx context.Context) error {
		rm.Touch(now)
		if saveErr := h.repo.Save(txCtx, rm); saveErr != nil {
			return saveErr
		}
		evt := &domainevent.RoadmapAdjusted{
			RoadmapID:  rm.ID(),
			UserID:     rm.UserID(),
			Reason:     "adaptive_cycle:" + decision.Reason,
			AdjustedAt: now,
		}
		return h.outbox.Enqueue(txCtx, rm.UserID(), evt)
	})
	if err != nil {
		return nil, err
	}
	return &ApplyAdaptiveCycleResult{Decision: decision, Roadmap: rm}, nil
}
