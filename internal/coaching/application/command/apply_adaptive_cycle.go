package command

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"time"

	"github.com/viethung213/gym-companion/internal/coaching/application/port"
	domainevent "github.com/viethung213/gym-companion/internal/coaching/domain/event"
	"github.com/viethung213/gym-companion/internal/coaching/domain/guardrail"
	"github.com/viethung213/gym-companion/internal/coaching/domain/roadmap"
	"github.com/viethung213/gym-companion/internal/coaching/domain/service"
)

// ApplyAdaptiveCycleCommand triggers UC-04.1 (BR-AC-04) evaluation.
type ApplyAdaptiveCycleCommand struct {
	UserID  string
	Metrics []service.WeeklyMetrics
	// ReviewedWeekIDs are the weeks this adaptation answers. They are stamped
	// reviewed in the same save as the rewrite, and if one already is (another
	// run adapted to it), nothing is applied again.
	ReviewedWeekIDs []string
}

// ApplyAdaptiveCycleResult echoes the decision applied.
type ApplyAdaptiveCycleResult struct {
	Decision service.AdaptationDecision
	Roadmap  *roadmap.Roadmap
	// AlreadyApplied is set when the weeks were reviewed by another run.
	AlreadyApplied bool
}

// ApplyAdaptiveCycleHandler orchestrates BR-AC-04 Trigger A.
type ApplyAdaptiveCycleHandler struct {
	tx       port.TransactionManager
	repo     port.RoadmapRepository
	agent    port.CoachAgent
	guard    *guardrail.Engine
	profiles port.UserProfileReader
	outbox   port.OutboxWriter
	clock    port.Clock
	engine   *service.AdaptiveCoachEngine
}

// NewApplyAdaptiveCycleHandler wires the handler.
func NewApplyAdaptiveCycleHandler(
	tx port.TransactionManager,
	repo port.RoadmapRepository,
	agent port.CoachAgent,
	guard *guardrail.Engine,
	profiles port.UserProfileReader,
	outbox port.OutboxWriter,
	clock port.Clock,
	engine *service.AdaptiveCoachEngine,
) *ApplyAdaptiveCycleHandler {
	if engine == nil {
		engine = service.NewAdaptiveCoachEngine()
	}

	return &ApplyAdaptiveCycleHandler{
		tx:       tx,
		repo:     repo,
		agent:    agent,
		guard:    guard,
		profiles: profiles,
		outbox:   outbox,
		clock:    clock,
		engine:   engine,
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

	// Read the schedule in the athlete's current zone, as the agent does.
	profile, err := h.profiles.GetProfile(ctx, cmd.UserID)
	if err != nil {
		return nil, fmt.Errorf("load profile for adaptive cycle: %w", err)
	}
	rm.SetTimezone(profile.Zone().String())

	// Checked before the model is paid for, and again after it answers.
	if anyReviewed(rm, cmd.ReviewedWeekIDs) {
		return &ApplyAdaptiveCycleResult{Decision: decision, Roadmap: rm, AlreadyApplied: true}, nil
	}

	// Same set the agent revises (adk pendingNode): PENDING sessions not yet started.
	upcoming := rm.UpcomingPendingSessions(h.clock.Now())
	if len(upcoming) == 0 {
		return &ApplyAdaptiveCycleResult{Decision: decision, Roadmap: rm}, nil
	}
	// The volume a deload cuts from, read before the model rewrites it.
	setsBefore := make(map[string]int, len(upcoming))
	for _, s := range upcoming {
		setsBefore[s.ID()] = s.MainSets()
	}

	drafts, err := h.agent.Adapt(ctx, cmd.UserID, &decision)
	if err != nil {
		return nil, fmt.Errorf("agent adapt: %w", err)
	}

	if rm, err = reloadForRewrite(ctx, h.repo, rm, profile.Zone().String()); err != nil {
		return nil, err
	}
	if anyReviewed(rm, cmd.ReviewedWeekIDs) {
		return &ApplyAdaptiveCycleResult{Decision: decision, Roadmap: rm, AlreadyApplied: true}, nil
	}

	// Generation takes a while; sessions that started meanwhile are left alone.
	now := h.clock.Now()
	pending := rm.UpcomingPendingSessions(now)

	// An adaptation only rewrites prescriptions; it never moves a session.
	aligned, ok := alignDrafts(pending, drafts)
	if !ok {
		return nil, fmt.Errorf("agent moved sessions instead of adapting the %d pending ones", len(pending))
	}
	applied, err := rewriteInPlace(pending, aligned, now)
	if err != nil {
		return nil, err
	}
	if applied == 0 {
		return nil, fmt.Errorf("no session was adapted out of %d pending", len(pending))
	}

	if decision.Kind == service.AdaptationTriggerDeload {
		if unloadErr := unloadVolume(ctx, rm, setsBefore, decision.Params.VolumePctChange, now); unloadErr != nil {
			return nil, unloadErr
		}
	}
	if fitErr := fitDeloadVolume(ctx, rm, now); fitErr != nil {
		return nil, fitErr
	}
	injuries := profile.Athlete().Injuries
	if result := h.guard.Check(ctx, rm, injuries, now); result.Status != guardrail.StatusApproved {
		return nil, fmt.Errorf("guardrail rejected adaptive cycle: %+v", result.Violations)
	}

	err = h.tx.WithTransaction(ctx, func(txCtx context.Context) error {
		rm.Touch(now)
		for _, id := range cmd.ReviewedWeekIDs {
			if markErr := rm.MarkReviewed(id, now); markErr != nil {
				return markErr
			}
		}

		if saveErr := h.repo.Save(txCtx, rm); saveErr != nil {
			return saveErr
		}

		evt := &domainevent.RoadmapAdjusted{
			RoadmapID: rm.ID(),

			UserID: rm.UserID(),

			Reason: "adaptive_cycle:" + decision.Reason,

			AdjustedAt: now,
		}

		return h.outbox.Enqueue(txCtx, rm.UserID(), evt)
	})

	if err != nil {
		return nil, err
	}

	return &ApplyAdaptiveCycleResult{Decision: decision, Roadmap: rm}, nil
}

// anyReviewed reports whether one of the weeks is already stamped reviewed.
func anyReviewed(rm *roadmap.Roadmap, weekIDs []string) bool {
	for _, w := range rm.Weeks() {
		if w.Reviewed() && slices.Contains(weekIDs, w.ID()) {
			return true
		}
	}
	return false
}

// unloadVolume enforces the deload's volume cut instead of trusting the model
// to: the prompt asks for it, but the model kept 2 sets per line live.
func unloadVolume(ctx context.Context, rm *roadmap.Roadmap, before map[string]int, pctChange float64, now time.Time) error {
	if pctChange >= 0 {
		return nil
	}
	cut, err := rm.UnloadUpcoming(before, 1+pctChange, now)
	if err != nil {
		return err
	}
	if len(cut) > 0 {
		slog.InfoContext(ctx, "Deload volume cut enforced (BR-AC-04)",
			slog.String("roadmap_id", rm.ID()), slog.Int("sessions", len(cut)))
	}
	return nil
}
