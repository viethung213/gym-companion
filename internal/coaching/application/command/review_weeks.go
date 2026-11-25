package command

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/viethung213/gym-companion/internal/coaching/application/port"
	"github.com/viethung213/gym-companion/internal/coaching/domain/roadmap"
	"github.com/viethung213/gym-companion/internal/coaching/domain/service"
)

const (
	// reviewGrace delays a week's review, so a workout finished just before
	// midnight and reported late still counts as done rather than skipped.
	reviewGrace = 6 * time.Hour

	// adaptRetryWindow is how long after a week ended a failed adaptation is
	// retried (the worker comes back every 30 minutes). Past it the week is
	// stamped reviewed anyway: the next week is well under way by then.
	adaptRetryWindow = 12 * time.Hour

	// highActualRPE is the effort from which a session counts as very hard
	// (BR-AC-04: three such sessions in a week trigger a deload).
	highActualRPE = 9.0
)

// ReviewWeeksResult reports what a review found and decided.
type ReviewWeeksResult struct {
	// DueWeeks are the week numbers that had ended and were reviewed.
	DueWeeks []int32
	// Metrics are the BR-AC-04 inputs of every ended week, oldest first.
	Metrics []service.WeeklyMetrics
	// Adaptation is the adaptive cycle's outcome, nil when it did not run.
	Adaptation *ApplyAdaptiveCycleResult
	// Deferred is set when the adaptation failed and will be retried.
	Deferred bool
}

// ReviewWeeksHandler closes finished training weeks and feeds their results
// to the adaptive cycle (BR-AC-04, Trigger A):
//
//  1. A week is due once it ended (midnight of its end date in the roadmap's
//     zone) plus reviewGrace. Its sessions still PENDING become SKIPPED.
//  2. Each ended week yields WeeklyMetrics from its scheduled TRAINING
//     sessions: completion rate, mean ΔRPE, very hard sessions.
//  3. AdaptiveCoachEngine decides from those metrics (more load, deload,
//     shorter sessions or nothing) and ApplyAdaptiveCycle rewrites the
//     remaining sessions accordingly.
//  4. Only then is the week stamped reviewed. A failed adaptation (the model
//     unavailable) leaves it open, so the next run retries it, up to
//     adaptRetryWindow after the week ended.
type ReviewWeeksHandler struct {
	tx    port.TransactionManager
	repo  port.RoadmapRepository
	adapt *ApplyAdaptiveCycleHandler
	clock port.Clock
}

// NewReviewWeeksHandler wires the handler. A nil adapt reviews weeks without
// adapting.
func NewReviewWeeksHandler(
	tx port.TransactionManager,
	repo port.RoadmapRepository,
	adapt *ApplyAdaptiveCycleHandler,
	clock port.Clock,
) *ReviewWeeksHandler {
	return &ReviewWeeksHandler{tx: tx, repo: repo, adapt: adapt, clock: clock}
}

// Handle reviews one roadmap.
func (h *ReviewWeeksHandler) Handle(ctx context.Context, roadmapID string) (*ReviewWeeksResult, error) {
	now := h.clock.Now()
	res := &ReviewWeeksResult{}

	rm, due, err := h.skipMissed(ctx, roadmapID, now)
	if err != nil || len(due) == 0 {
		return res, err
	}
	for _, w := range due {
		res.DueWeeks = append(res.DueWeeks, w.WeekNumber())
	}
	res.Metrics = endedWeekMetrics(rm, due)

	adaptErr := h.runAdaptation(ctx, rm, due, res, now)
	if adaptErr != nil {
		lastEnded := due[len(due)-1].EndsAt(rm.Location())
		if now.Sub(lastEnded) < adaptRetryWindow {
			res.Deferred = true
			return res, adaptErr
		}
		slog.WarnContext(ctx, "Giving up adapting after the retry window; closing the week",
			slog.String("roadmap_id", roadmapID), slog.Any("error", adaptErr))
	}

	if err := h.markReviewed(ctx, roadmapID, due, now); err != nil {
		return res, err
	}
	return res, adaptErr
}

// skipMissed loads the roadmap and turns the missed sessions of its due weeks
// into SKIPPED, committing that on its own.
func (h *ReviewWeeksHandler) skipMissed(
	ctx context.Context, roadmapID string, now time.Time,
) (rm *roadmap.Roadmap, due []*roadmap.WeekPlan, err error) {
	err = h.tx.WithTransaction(ctx, func(txCtx context.Context) error {
		loaded, findErr := h.repo.FindByID(txCtx, roadmapID)
		if findErr != nil {
			return fmt.Errorf("load roadmap %s: %w", roadmapID, findErr)
		}
		if loaded.Status() != roadmap.StatusActive {
			return nil
		}
		due = loaded.WeeksDueForReview(now.Add(-reviewGrace))
		for _, w := range due {
			if skipErr := loaded.SkipMissedSessions(w.ID()); skipErr != nil {
				return skipErr
			}
		}
		if len(due) == 0 {
			return nil
		}
		rm = loaded
		return h.repo.Save(txCtx, loaded)
	})
	return rm, due, err
}

// runAdaptation runs the adaptive cycle on the weeks being reviewed while
// sessions remain to rewrite; the adaptation stamps them reviewed itself.
// The decision is about the newest week; when that week had no scheduled
// training (only recovery or ad-hoc work) there is nothing new to adapt to, and
// the last metrics would be an older week already adapted to.
func (h *ReviewWeeksHandler) runAdaptation(
	ctx context.Context, rm *roadmap.Roadmap, due []*roadmap.WeekPlan, res *ReviewWeeksResult, now time.Time,
) error {
	if h.adapt == nil || len(res.Metrics) == 0 || len(rm.UpcomingPendingSessions(now)) == 0 {
		return nil
	}
	newest := due[len(due)-1]
	if res.Metrics[len(res.Metrics)-1].WeekNumber != newest.WeekNumber() {
		return nil
	}
	weekIDs := make([]string, 0, len(due))
	for _, w := range due {
		weekIDs = append(weekIDs, w.ID())
	}
	adaptation, err := h.adapt.Handle(ctx, ApplyAdaptiveCycleCommand{
		UserID: rm.UserID(), Metrics: res.Metrics, ReviewedWeekIDs: weekIDs,
	})
	if err != nil {
		return fmt.Errorf("adapt roadmap %s after week review: %w", rm.ID(), err)
	}
	res.Adaptation = adaptation
	return nil
}

// markReviewed stamps the due weeks on a fresh load, keeping what the
// adaptation saved in between.
func (h *ReviewWeeksHandler) markReviewed(ctx context.Context, roadmapID string, due []*roadmap.WeekPlan, now time.Time) error {
	return h.tx.WithTransaction(ctx, func(txCtx context.Context) error {
		rm, err := h.repo.FindByID(txCtx, roadmapID)
		if err != nil {
			return fmt.Errorf("reload roadmap %s: %w", roadmapID, err)
		}
		for _, w := range due {
			if err := rm.MarkReviewed(w.ID(), now); err != nil {
				return err
			}
		}
		return h.repo.Save(txCtx, rm)
	})
}

// errNoReviewableSessions marks a week with no scheduled training to measure.
var errNoReviewableSessions = errors.New("week has no scheduled training session")

// endedWeekMetrics returns the metrics of every ended week (already reviewed
// or due now) with scheduled training, oldest first.
func endedWeekMetrics(rm *roadmap.Roadmap, due []*roadmap.WeekPlan) []service.WeeklyMetrics {
	dueIDs := make(map[string]bool, len(due))
	for _, w := range due {
		dueIDs[w.ID()] = true
	}
	var out []service.WeeklyMetrics
	for _, w := range rm.Weeks() {
		if !w.Reviewed() && !dueIDs[w.ID()] {
			continue
		}
		if m, err := weekMetrics(rm, w); err == nil {
			out = append(out, m)
		}
	}
	return out
}

// weekMetrics measures a week's scheduled TRAINING sessions. Recovery and
// user-built sessions are not the program and are left out. A session not
// completed (skipped, aborted) counts as 0% done; ΔRPE and effort come from
// the completed ones that reported an RPE, effort being the session's target
// RPE plus its ΔRPE.
func weekMetrics(rm *roadmap.Roadmap, w *roadmap.WeekPlan) (service.WeeklyMetrics, error) {
	m := service.WeeklyMetrics{WeekNumber: w.WeekNumber()}
	var scrSum, deltaSum float64
	rated := 0 // completed sessions that reported their effort

	for _, d := range w.Days() {
		for _, s := range d.Sessions() {
			info := s.Info()
			if info.Source != roadmap.SessionPlanSourceScheduled || s.Kind() != roadmap.SessionKindTraining {
				continue
			}
			m.TotalSessions++
			if info.Status != roadmap.SessionPlanStatusCompleted || info.SessionSCR == nil {
				continue
			}
			scrSum += float64(*info.SessionSCR)

			if info.SessionDeltaRPE == nil {
				continue // done, but the effort was not reported
			}
			rated++
			delta := float64(*info.SessionDeltaRPE)
			deltaSum += delta

			actual := float64(sessionTargetRPE(rm, s)) + delta
			m.MaxActualRPE = max(m.MaxActualRPE, actual)
			if actual >= highActualRPE {
				m.HighRPESessions++
			}
		}
	}

	if m.TotalSessions == 0 {
		return m, errNoReviewableSessions
	}
	m.SCR = scrSum / float64(m.TotalSessions)
	if rated > 0 {
		m.AvgDeltaRPE = deltaSum / float64(rated)
	} else {
		m.EffortUnreported = true
	}
	return m, nil
}
