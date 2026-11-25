package command

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/viethung213/gym-companion/internal/coaching/application/port"
	domainevent "github.com/viethung213/gym-companion/internal/coaching/domain/event"
	"github.com/viethung213/gym-companion/internal/coaching/domain/guardrail"
	"github.com/viethung213/gym-companion/internal/coaching/domain/roadmap"
)

// RegenerateScheduleCommand is the input to UC-02.3.
type RegenerateScheduleCommand struct {
	UserID string
	Reason string // "profile_updated", "injury_reported", "injury_recovered", ...
	// ChangedAt is when the profile change happened. A roadmap planned after it
	// already reflects it, so nothing is regenerated. Zero always regenerates.
	ChangedAt time.Time
}

// RegenerateScheduleResult echoes the updated roadmap.
type RegenerateScheduleResult struct {
	Roadmap *roadmap.Roadmap
}

// RegenerateScheduleHandler orchestrates UC-02.3 (FR-AC-06).
type RegenerateScheduleHandler struct {
	tx       port.TransactionManager
	repo     port.RoadmapRepository
	agent    port.CoachAgent
	guard    *guardrail.Engine
	profiles port.UserProfileReader
	outbox   port.OutboxWriter
	clock    port.Clock
	ids      port.IDGenerator
	// rotation, when set, discards the next block planned from a profile that
	// has since changed, so it is planned again from the current one.
	rotation port.RoadmapRotation
}

// WithPlannedBlocks enables discarding a stale planned block on profile changes.
func (h *RegenerateScheduleHandler) WithPlannedBlocks(rotation port.RoadmapRotation) *RegenerateScheduleHandler {
	h.rotation = rotation
	return h
}

// NewRegenerateScheduleHandler wires the handler.
func NewRegenerateScheduleHandler(
	tx port.TransactionManager,
	repo port.RoadmapRepository,
	agent port.CoachAgent,
	guard *guardrail.Engine,
	profiles port.UserProfileReader,
	outbox port.OutboxWriter,
	clock port.Clock,
	ids port.IDGenerator,
) *RegenerateScheduleHandler {
	return &RegenerateScheduleHandler{
		tx:       tx,
		repo:     repo,
		agent:    agent,
		guard:    guard,
		profiles: profiles,
		outbox:   outbox,
		clock:    clock,
		ids:      ids,
	}
}

// Handle executes UC-02.3 on the PENDING sessions that have not started (D3).
//
// The agent either rewrites those sessions in place or — when the athlete's
// weekly slots changed — returns sessions on the new slots, which replace
// them. Drafts that match the pending sessions (day, slot, kind) are rewrites;
// anything else is a reschedule.
func (h *RegenerateScheduleHandler) Handle(ctx context.Context, cmd RegenerateScheduleCommand) (*RegenerateScheduleResult, error) {
	if cmd.UserID == "" {
		return nil, errors.New("user_id required")
	}

	rm, err := h.repo.FindActiveByUser(ctx, cmd.UserID)
	if err != nil {
		return nil, err
	}

	if discardErr := h.discardStalePlan(ctx, cmd); discardErr != nil {
		return nil, discardErr
	}

	// Onboarding emits the injuries with ProfileCompleted: the roadmap planned
	// from that profile already holds them, and rewriting it would cost a
	// second generation for nothing.
	if !cmd.ChangedAt.IsZero() && !rm.Info().CreatedAt.Before(cmd.ChangedAt) {
		slog.InfoContext(ctx, "Profile change predates the active roadmap; already reflected, not regenerating",
			slog.String("user_id", cmd.UserID), slog.String("reason", cmd.Reason))
		return &RegenerateScheduleResult{Roadmap: rm}, nil
	}

	// The schedule is read in the athlete's current zone, the same zone the
	// agent plans in, so both see the same upcoming sessions.
	profile, err := h.profiles.GetProfile(ctx, cmd.UserID)
	if err != nil {
		return nil, fmt.Errorf("load profile for regenerate: %w", err)
	}
	rm.SetTimezone(profile.Zone().String())

	pending := rm.UpcomingPendingSessions(h.clock.Now())
	if len(pending) == 0 {
		return &RegenerateScheduleResult{Roadmap: rm}, nil // nothing left to regenerate
	}

	ids := make([]string, 0, len(pending))
	for _, s := range pending {
		ids = append(ids, s.ID())
	}

	drafts, err := h.agent.RegeneratePending(ctx, cmd.UserID, ids)
	if err != nil {
		return nil, fmt.Errorf("agent regenerate: %w", err)
	}

	if rm, err = reloadForRewrite(ctx, h.repo, rm, profile.Zone().String()); err != nil {
		return nil, err
	}

	// Generation takes a while: a session may have started meanwhile, and a
	// started session is never touched.
	now := h.clock.Now()
	pending = rm.UpcomingPendingSessions(now)

	if aligned, ok := alignDrafts(pending, drafts); ok {
		applied, rwErr := rewriteInPlace(pending, aligned, now)
		if rwErr != nil {
			return nil, rwErr
		}
		if applied == 0 {
			return nil, fmt.Errorf("no session was rewritten out of %d pending", len(pending))
		}
	} else if rsErr := rm.ReschedulePending(now, drafts, h.ids.NewID); rsErr != nil {
		return nil, fmt.Errorf("reschedule pending sessions: %w", rsErr)
	}

	if fitErr := fitDeloadVolume(ctx, rm, now); fitErr != nil {
		return nil, fitErr
	}
	athlete := profile.Athlete()
	if result := h.guard.Check(ctx, rm, athlete.Injuries, now); result.Status != guardrail.StatusApproved {
		return nil, fmt.Errorf("guardrail rejected regen: %+v", result.Violations)
	}

	err = h.tx.WithTransaction(ctx, func(txCtx context.Context) error {
		rm.Touch(now)
		if saveErr := h.repo.Save(txCtx, rm); saveErr != nil {
			return saveErr
		}
		return h.outbox.Enqueue(txCtx, rm.UserID(), &domainevent.RoadmapAdjusted{
			RoadmapID:  rm.ID(),
			UserID:     rm.UserID(),
			Reason:     cmd.Reason,
			AdjustedAt: now,
		})
	})
	if err != nil {
		return nil, err
	}

	return &RegenerateScheduleResult{Roadmap: rm}, nil
}

// discardStalePlan deletes the user's planned next block when it was drafted
// before the profile change: it may hold an exercise a new injury rules out or
// sit on slots the athlete gave up. The next block is then planned again.
func (h *RegenerateScheduleHandler) discardStalePlan(ctx context.Context, cmd RegenerateScheduleCommand) error {
	if h.rotation == nil || cmd.ChangedAt.IsZero() {
		return nil
	}
	planned, err := h.rotation.FindPlannedByUser(ctx, cmd.UserID)
	if errors.Is(err, roadmap.ErrRoadmapNotFound) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("find planned roadmap: %w", err)
	}
	if !planned.Info().CreatedAt.Before(cmd.ChangedAt) {
		return nil
	}
	slog.InfoContext(ctx, "Profile changed after the next block was planned; discarding it to plan again",
		slog.String("user_id", cmd.UserID), slog.String("planned_roadmap_id", planned.ID()), slog.String("reason", cmd.Reason))
	return h.rotation.DeletePlannedRoadmap(ctx, planned.ID())
}
