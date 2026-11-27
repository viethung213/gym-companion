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

// profileCommitSlack covers the gap between a profile change's timestamp and
// its commit: the event is stamped inside the profile transaction, so a plan
// started just after the stamp may still have read the profile before the
// commit. It stays below the outbox-and-Kafka hop that separates onboarding
// from the roadmap it plans, so that roadmap still counts as reflecting it.
const profileCommitSlack = 500 * time.Millisecond

// plannedAfter reports whether a roadmap planned at plannedAt already read a
// profile changed at changedAt. A doubt regenerates: one more generation is
// cheaper than an injury left out of the plan.
func plannedAfter(plannedAt, changedAt time.Time) bool {
	return !plannedAt.Before(changedAt.Add(profileCommitSlack))
}

// RegenerateScheduleCommand is the input to UC-02.3.
type RegenerateScheduleCommand struct {
	UserID string
	Reason string // "profile_updated", "injury_reported", "injury_recovered", ...
	// ChangedAt is when the profile change happened. A roadmap planned after it
	// already reflects it, so nothing is regenerated. Zero (an event without a
	// usable time) always regenerates and discards the planned next block.
	ChangedAt time.Time
}

// RegenerateScheduleResult echoes the updated roadmap.
type RegenerateScheduleResult struct {
	Roadmap *roadmap.Roadmap
	// Deferred is set when the schedule could not be regenerated now (the
	// model unavailable, a draft the guardrail refused): the change is kept on
	// the roadmap and the lifecycle worker retries it (RetryDue).
	Deferred bool
}

// errRegenerationFailed marks a failure of the generation itself, worth a
// later retry; a missing roadmap or a storage error is not.
var errRegenerationFailed = errors.New("regeneration failed")

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
// A failed generation does not lose the change: it is kept on the roadmap,
// reported as Deferred, and retried by the lifecycle worker (RetryDue).
func (h *RegenerateScheduleHandler) Handle(ctx context.Context, cmd RegenerateScheduleCommand) (*RegenerateScheduleResult, error) {
	if cmd.UserID == "" {
		return nil, errors.New("user_id required")
	}
	res, err := h.regenerate(ctx, cmd)
	if !errors.Is(err, errRegenerationFailed) {
		return res, err
	}
	if deferErr := h.deferRegeneration(ctx, cmd); deferErr != nil {
		return nil, errors.Join(err, deferErr)
	}
	slog.WarnContext(ctx, "Schedule regeneration deferred; the lifecycle worker retries it",
		slog.String("user_id", cmd.UserID), slog.String("reason", cmd.Reason), slog.Any("error", err))
	return &RegenerateScheduleResult{Deferred: true}, nil
}

// RetryDue retries the profile change waiting on the athlete's active
// roadmap. A change waiting longer than roadmap.RegenerationGiveUpAfter is
// dropped: the athlete has trained on the old schedule since, and the next
// profile change or block plans from the current profile anyway.
func (h *RegenerateScheduleHandler) RetryDue(ctx context.Context, userID string) (*RegenerateScheduleResult, error) {
	rm, err := h.repo.FindActiveByUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	due, err := rm.PendingRegeneration()
	if errors.Is(err, roadmap.ErrNoRegenerationDue) {
		return &RegenerateScheduleResult{Roadmap: rm}, nil
	}
	if due.RegenerationExpired(h.clock.Now()) {
		slog.WarnContext(ctx, "Dropping a profile change the schedule could not absorb in time",
			slog.String("user_id", userID), slog.String("reason", due.Reason), slog.Time("changed_at", due.ChangedAt))
		if clearErr := h.clearRegenerationDue(ctx, rm); clearErr != nil {
			return nil, clearErr
		}
		return &RegenerateScheduleResult{Roadmap: rm}, nil
	}
	return h.Handle(ctx, RegenerateScheduleCommand{UserID: userID, Reason: due.Reason, ChangedAt: due.ChangedAt})
}

// regenerate rewrites the pending sessions. The agent either rewrites them in
// place or — when the athlete's weekly slots changed — returns sessions on the
// new slots, which replace them. Drafts that match the pending sessions (day,
// slot, kind) are rewrites; anything else is a reschedule. Failures of the
// generation itself wrap errRegenerationFailed.
func (h *RegenerateScheduleHandler) regenerate(ctx context.Context, cmd RegenerateScheduleCommand) (*RegenerateScheduleResult, error) {

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
	if !cmd.ChangedAt.IsZero() && plannedAfter(rm.Info().CreatedAt, cmd.ChangedAt) {
		slog.InfoContext(ctx, "Profile change predates the active roadmap; already reflected, not regenerating",
			slog.String("user_id", cmd.UserID), slog.String("reason", cmd.Reason))
		return &RegenerateScheduleResult{Roadmap: rm}, h.clearRegenerationDue(ctx, rm)
	}

	// The schedule is read in the athlete's current zone, the same zone the
	// agent plans in, so both see the same upcoming sessions.
	profile, err := h.profiles.GetProfile(ctx, cmd.UserID)
	if err != nil {
		return nil, fmt.Errorf("load profile for regenerate: %w", err)
	}
	rm.SetTimezone(profile.Zone().String())

	before := rm.UpcomingPendingSessions(h.clock.Now())
	if len(before) == 0 { // nothing left to regenerate
		return &RegenerateScheduleResult{Roadmap: rm}, h.clearRegenerationDue(ctx, rm)
	}

	ids := make([]string, 0, len(before))
	for _, s := range before {
		ids = append(ids, s.ID())
	}

	drafts, err := h.agent.RegeneratePending(ctx, cmd.UserID, ids)
	if err != nil {
		return nil, fmt.Errorf("%w: agent regenerate: %w", errRegenerationFailed, err)
	}

	if rm, err = reloadForRewrite(ctx, h.repo, rm, profile.Zone().String()); err != nil {
		return nil, err
	}

	// Generation takes a while: a session may have started meanwhile, and a
	// started session is never touched.
	now := h.clock.Now()
	pending := rm.UpcomingPendingSessions(now)
	drafts = dropFinished(before, pending, drafts)

	if aligned, ok := alignDrafts(pending, drafts); ok {
		applied, rwErr := rewriteInPlace(pending, aligned, now)
		if rwErr != nil {
			return nil, rwErr
		}
		if applied == 0 {
			return nil, fmt.Errorf("%w: no session was rewritten out of %d pending", errRegenerationFailed, len(pending))
		}
	} else if rsErr := rm.ReschedulePending(now, drafts, h.ids.NewID); rsErr != nil {
		return nil, fmt.Errorf("%w: reschedule pending sessions: %w", errRegenerationFailed, rsErr)
	}

	if fitErr := fitDeloadVolume(ctx, rm, now); fitErr != nil {
		return nil, fitErr
	}
	athlete := profile.Athlete()
	if result := h.guard.Check(ctx, rm, athlete.Injuries, now); result.Status != guardrail.StatusApproved {
		return nil, fmt.Errorf("%w: guardrail rejected regen: %+v", errRegenerationFailed, result.Violations)
	}

	err = h.tx.WithTransaction(ctx, func(txCtx context.Context) error {
		rm.ClearRegenerationDue()
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

// deferRegeneration keeps the change on the athlete's active roadmap for the
// lifecycle worker. It reads the roadmap again: the generation that failed may
// have taken minutes, and the save must start from the stored version. An
// unknown change time is taken as now, as Handle does.
func (h *RegenerateScheduleHandler) deferRegeneration(ctx context.Context, cmd RegenerateScheduleCommand) error {
	changedAt := cmd.ChangedAt
	if changedAt.IsZero() {
		changedAt = h.clock.Now()
	}
	return h.tx.WithTransaction(ctx, func(txCtx context.Context) error {
		rm, err := h.repo.FindActiveByUser(txCtx, cmd.UserID)
		if err != nil {
			return fmt.Errorf("load roadmap to defer regeneration: %w", err)
		}
		rm.MarkRegenerationDue(changedAt, cmd.Reason)
		rm.Touch(h.clock.Now())
		return h.repo.Save(txCtx, rm)
	})
}

// clearRegenerationDue drops a waiting change the roadmap no longer needs (it
// already reflects it, or has nothing left to rewrite). Nothing is saved when
// no change was waiting.
func (h *RegenerateScheduleHandler) clearRegenerationDue(ctx context.Context, rm *roadmap.Roadmap) error {
	if _, err := rm.PendingRegeneration(); errors.Is(err, roadmap.ErrNoRegenerationDue) {
		return nil
	}
	rm.ClearRegenerationDue()
	rm.Touch(h.clock.Now())
	return h.tx.WithTransaction(ctx, func(txCtx context.Context) error {
		return h.repo.Save(txCtx, rm)
	})
}

// discardStalePlan deletes the user's planned next block when it was drafted
// before the profile change: it may hold an exercise a new injury rules out or
// sit on slots the athlete gave up. The next block is then planned again.
func (h *RegenerateScheduleHandler) discardStalePlan(ctx context.Context, cmd RegenerateScheduleCommand) error {
	if h.rotation == nil {
		return nil
	}
	changedAt := cmd.ChangedAt
	if changedAt.IsZero() {
		// Unknown time: assume the change is newer than any plan, as the
		// active roadmap does (zero always regenerates).
		changedAt = h.clock.Now()
	}
	planned, err := h.rotation.FindPlannedByUser(ctx, cmd.UserID)
	if errors.Is(err, roadmap.ErrRoadmapNotFound) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("find planned roadmap: %w", err)
	}
	if plannedAfter(planned.Info().CreatedAt, changedAt) {
		return nil
	}
	slog.InfoContext(ctx, "Profile changed after the next block was planned; discarding it to plan again",
		slog.String("user_id", cmd.UserID), slog.String("planned_roadmap_id", planned.ID()), slog.String("reason", cmd.Reason))
	return h.rotation.DeletePlannedRoadmap(ctx, planned.ID())
}
