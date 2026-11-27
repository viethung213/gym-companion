package command

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/viethung213/gym-companion/internal/coaching/application/port"
	domainevent "github.com/viethung213/gym-companion/internal/coaching/domain/event"
	"github.com/viethung213/gym-companion/internal/coaching/domain/roadmap"
)

// RollOverHandler moves a user from an ended roadmap to its successor.
type RollOverHandler struct {
	tx       port.TransactionManager
	repo     port.RoadmapRepository
	rotation port.RoadmapRotation
	outbox   port.OutboxWriter
	clock    port.Clock
}

// NewRollOverHandler wires the handler.
func NewRollOverHandler(
	tx port.TransactionManager,
	repo port.RoadmapRepository,
	rotation port.RoadmapRotation,
	outbox port.OutboxWriter,
	clock port.Clock,
) *RollOverHandler {
	return &RollOverHandler{tx: tx, repo: repo, rotation: rotation, outbox: outbox, clock: clock}
}

// RollOverResult reports what a roll-over did.
type RollOverResult struct {
	// Active is the roadmap active afterwards: the current one when it has
	// not ended, its successor, or nil when none was planned (the athlete then
	// starts a new roadmap themselves).
	Active *roadmap.Roadmap
	// RolledOver is set when the current roadmap had ended and was completed.
	RolledOver bool
	// StartFresh is set when the athlete was left without a roadmap and should
	// get a new one: the block that ended was trained in and ended within
	// freshStartWindow. A block long over (an athlete gone for months) or never
	// trained in gets no new generation; the app offers one when they return.
	StartFresh bool
}

// freshStartWindow is how long after a block ended a new one is still started
// for the athlete without them asking.
const freshStartWindow = 7 * 24 * time.Hour

// Handle completes the user's ACTIVE roadmap once it has ended and activates
// the block planned to follow it.
//
// Closing the last week skips what was not trained without adapting anything:
// no session is left to rewrite. A workout reported late still completes its
// SKIPPED session.
func (h *RollOverHandler) Handle(ctx context.Context, userID string) (*RollOverResult, error) {
	res := &RollOverResult{}
	err := h.tx.WithTransaction(ctx, func(txCtx context.Context) error {
		current, err := h.repo.FindActiveByUser(txCtx, userID)
		if errors.Is(err, roadmap.ErrRoadmapNotFound) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("find active roadmap: %w", err)
		}

		now := h.clock.Now()
		if now.Before(current.EndsAt()) {
			res.Active = current
			return nil
		}
		if doneErr := completeRoadmap(current, now); doneErr != nil {
			return doneErr
		}
		res.RolledOver = true
		if saveErr := h.repo.Save(txCtx, current); saveErr != nil {
			return saveErr
		}

		next, err := h.rotation.FindPlannedByUser(txCtx, userID)
		if errors.Is(err, roadmap.ErrRoadmapNotFound) {
			res.StartFresh = startFresh(current, now)
			return nil
		}
		if err != nil {
			return fmt.Errorf("find planned roadmap: %w", err)
		}
		if !next.Follows(current) {
			res.StartFresh = startFresh(current, now)
			return h.rotation.DeletePlannedRoadmap(txCtx, next.ID())
		}
		if activateErr := next.Activate(now); activateErr != nil {
			return activateErr
		}
		if saveErr := h.repo.Save(txCtx, next); saveErr != nil {
			return saveErr
		}
		res.Active = next
		return h.outbox.Enqueue(txCtx, userID, &domainevent.RoadmapInitiated{
			RoadmapID: next.ID(), UserID: userID, InitiatedAt: now,
		})
	})
	if err != nil {
		return nil, fmt.Errorf("roll over roadmap for user %s: %w", userID, err)
	}
	return res, nil
}

// startFresh reports whether an athlete whose block ended unplanned gets a new
// roadmap without asking (see RollOverResult.StartFresh).
func startFresh(ended *roadmap.Roadmap, now time.Time) bool {
	return ended.Trained() && now.Sub(ended.EndsAt()) <= freshStartWindow
}

// completeRoadmap closes every week still open and marks the roadmap COMPLETED.
func completeRoadmap(rm *roadmap.Roadmap, now time.Time) error {
	for _, w := range rm.Weeks() {
		if w.Reviewed() {
			continue
		}
		if err := rm.SkipMissedSessions(w.ID()); err != nil {
			return err
		}
		if err := rm.MarkReviewed(w.ID(), now); err != nil {
			return err
		}
	}
	return rm.MarkCompleted(now)
}
