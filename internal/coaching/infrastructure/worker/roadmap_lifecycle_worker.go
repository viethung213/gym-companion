package worker

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/viethung213/gym-companion/internal/coaching/application/command"
	"github.com/viethung213/gym-companion/internal/coaching/application/port"
	"github.com/viethung213/gym-companion/internal/coaching/domain/roadmap"
)

const (
	// generationTimeout bounds one model-backed job (a week review with its
	// adaptation, or planning a block), retries included.
	generationTimeout = 5 * time.Minute

	// rolloverTimeout bounds one roll-over, which only touches the database.
	rolloverTimeout = 30 * time.Second

	// maxConcurrentGenerations caps the model calls in flight, so a busy tick
	// stays within the provider's rate limits while users are not served one
	// by one.
	maxConcurrentGenerations = 3

	// planRetryDelay spaces out retries of a block that failed to plan: the
	// final week leaves room for a few attempts a day.
	planRetryDelay = 6 * time.Hour

	// reviewRetryDelay spaces out retries of a failed adaptation, which the
	// review retries only during its 12h window.
	reviewRetryDelay = 2 * time.Hour
)

// The handlers the worker drives, as interfaces so the loop tests without them.
type (
	rollOver interface {
		Handle(ctx context.Context, userID string) (*command.RollOverResult, error)
	}
	weekReviewer interface {
		Handle(ctx context.Context, roadmapID string) (*command.ReviewWeeksResult, error)
	}
	blockPlanner interface {
		Handle(ctx context.Context, roadmapID string) (*roadmap.Roadmap, error)
	}
	roadmapInitiator interface {
		Handle(ctx context.Context, cmd command.InitiateRoadmapCommand) (*command.InitiateRoadmapResult, error)
	}
)

// RoadmapLifecycleWorker keeps every athlete's roadmap moving. Each tick, in
// order:
//
//  1. roll over: complete the roadmaps that ended and activate their planned
//     successors (database only, so it runs first and nobody waits on a model);
//  2. review: close finished weeks and adapt the rest of the block (BR-AC-04);
//  3. plan: draft the next block of roadmaps in their final week;
//  4. start fresh: a new roadmap for athletes whose block ended unplanned,
//     when the roll-over says so (they trained in it and it just ended).
//
// Steps 2 to 4 call the model and run up to maxConcurrentGenerations
// roadmaps at once. The steps stay sequential, so a roadmap is never reviewed
// and planned at the same time.
type RoadmapLifecycleWorker struct {
	queue    port.RoadmapLifecycleStore
	rollOver rollOver
	reviewer weekReviewer
	planner  blockPlanner
	// initiator, when set, starts a fresh roadmap for an athlete whose block
	// ended with no successor planned (generation kept failing, or the plan was
	// discarded by a late profile change) and RollOverResult.StartFresh is set.
	initiator roadmapInitiator
	interval  time.Duration
	// reviewGate and planGate space out retries of failed model-backed jobs.
	reviewGate *retryGate
	planGate   *retryGate
	stopOnce   sync.Once
	stop       chan struct{}
}

// NewRoadmapLifecycleWorker constructs the worker.
func NewRoadmapLifecycleWorker(
	queue port.RoadmapLifecycleStore,
	rollOver rollOver,
	reviewer weekReviewer,
	planner blockPlanner,
	interval time.Duration,
) *RoadmapLifecycleWorker {
	if interval <= 0 {
		interval = 30 * time.Minute
	}
	return &RoadmapLifecycleWorker{
		queue: queue, rollOver: rollOver, reviewer: reviewer, planner: planner,
		interval: interval, stop: make(chan struct{}),
		reviewGate: newRetryGate(reviewRetryDelay), planGate: newRetryGate(planRetryDelay),
	}
}

// WithInitiator enables starting a fresh roadmap after a block ends unplanned.
func (w *RoadmapLifecycleWorker) WithInitiator(i roadmapInitiator) *RoadmapLifecycleWorker {
	w.initiator = i
	return w
}

// Start runs a tick now and then on every interval, until Stop or ctx ends.
func (w *RoadmapLifecycleWorker) Start(ctx context.Context) {
	slog.InfoContext(ctx, "RoadmapLifecycleWorker started", slog.Duration("interval", w.interval))
	go func() {
		ticker := time.NewTicker(w.interval)
		defer ticker.Stop()
		for {
			w.RunOnce(ctx, time.Now())
			select {
			case <-ticker.C:
			case <-w.stop:
				return
			case <-ctx.Done():
				return
			}
		}
	}()
}

// Stop ends the worker loop.
func (w *RoadmapLifecycleWorker) Stop() { w.stopOnce.Do(func() { close(w.stop) }) }

// RunOnce performs one tick. Queries look a day ahead, since no zone is more
// than a day from UTC; each handler applies the exact cut-off in the
// roadmap's zone.
func (w *RoadmapLifecycleWorker) RunOnce(ctx context.Context, now time.Time) {
	horizon := now.AddDate(0, 0, 1)

	var unplanned []string // athletes to start a fresh roadmap for (RollOverResult.StartFresh)
	if ended, err := w.queue.ActiveRoadmapsEndingBy(ctx, horizon); err != nil {
		slog.ErrorContext(ctx, "Roadmap lifecycle: listing ended roadmaps failed", slog.Any("error", err))
	} else {
		for _, ref := range ended {
			if w.rollOverUser(ctx, ref.UserID) {
				unplanned = append(unplanned, ref.UserID)
			}
		}
	}

	if ids, err := w.queue.RoadmapIDsWithOpenWeekEndingBy(ctx, horizon); err != nil {
		slog.ErrorContext(ctx, "Roadmap lifecycle: listing weeks to review failed", slog.Any("error", err))
	} else {
		forEachBounded(ctx, w.reviewGate.due(ids, now), func(jobCtx context.Context, id string) {
			w.reviewGate.record(id, now, w.review(jobCtx, id))
		})
	}

	// A roadmap is in its final week from 7 days before it ends; the planner
	// checks the exact start and whether a successor is already planned.
	if ending, err := w.queue.ActiveRoadmapsEndingBy(ctx, horizon.AddDate(0, 0, roadmap.DaysPerWeek)); err != nil {
		slog.ErrorContext(ctx, "Roadmap lifecycle: listing roadmaps to plan ahead failed", slog.Any("error", err))
	} else {
		ids := make([]string, 0, len(ending))
		for _, ref := range ending {
			ids = append(ids, ref.RoadmapID)
		}
		forEachBounded(ctx, w.planGate.due(ids, now), func(jobCtx context.Context, id string) {
			w.planGate.record(id, now, w.plan(jobCtx, id))
		})
	}

	// Best effort: if this fails too, the app offers to start a roadmap when the
	// athlete opens it (no ACTIVE roadmap), and starting a workout does so itself.
	if w.initiator != nil {
		forEachBounded(ctx, unplanned, w.startFresh)
	}
}

// rollOverUser rolls the athlete over and reports whether they should get a
// fresh roadmap (RollOverResult.StartFresh).
func (w *RoadmapLifecycleWorker) rollOverUser(ctx context.Context, userID string) (startFresh bool) {
	jobCtx, cancel := context.WithTimeout(ctx, rolloverTimeout)
	defer cancel()
	res, err := w.rollOver.Handle(jobCtx, userID)
	switch {
	case err != nil:
		slog.ErrorContext(ctx, "Roadmap roll-over failed", slog.String("user_id", userID), slog.Any("error", err))
	case !res.RolledOver:
		// Listed a day ahead; not over yet in the athlete's zone.
	case res.Active == nil:
		slog.InfoContext(ctx, "Roadmap completed; no next block was planned",
			slog.String("user_id", userID), slog.Bool("start_fresh", res.StartFresh))
		return res.StartFresh
	default:
		slog.InfoContext(ctx, "Roadmap rolled over to the planned block",
			slog.String("user_id", userID), slog.String("roadmap_id", res.Active.ID()))
	}
	return false
}

func (w *RoadmapLifecycleWorker) startFresh(ctx context.Context, userID string) {
	res, err := w.initiator.Handle(ctx, command.InitiateRoadmapCommand{UserID: userID})
	if errors.Is(err, roadmap.ErrActiveRoadmapExists) {
		return // the athlete started one themselves meanwhile
	}
	if err != nil {
		slog.ErrorContext(ctx, "Starting a fresh roadmap after the block ended failed; the app offers it",
			slog.String("user_id", userID), slog.Any("error", err))
		return
	}
	slog.InfoContext(ctx, "Fresh roadmap started after the block ended",
		slog.String("user_id", userID), slog.String("roadmap_id", res.Roadmap.ID()))
}

// review closes the roadmap's finished weeks; it reports false on a failure.
func (w *RoadmapLifecycleWorker) review(ctx context.Context, roadmapID string) (ok bool) {
	res, err := w.reviewer.Handle(ctx, roadmapID)
	if err != nil {
		slog.ErrorContext(ctx, "Week review failed", slog.String("roadmap_id", roadmapID), slog.Any("error", err))
	}
	if res == nil || len(res.DueWeeks) == 0 {
		return err == nil
	}

	attrs := []any{slog.String("roadmap_id", roadmapID), slog.Any("weeks", res.DueWeeks), slog.Bool("adaptation_deferred", res.Deferred)}
	if n := len(res.Metrics); n > 0 {
		last := res.Metrics[n-1]
		attrs = append(attrs, slog.Float64("scr", last.SCR), slog.Float64("avg_delta_rpe", last.AvgDeltaRPE),
			slog.Int("high_rpe_sessions", last.HighRPESessions), slog.Int("sessions", last.TotalSessions))
	}
	if res.Adaptation != nil {
		attrs = append(attrs, slog.String("decision", res.Adaptation.Decision.Reason))
	}
	msg := "Training week reviewed"
	if res.Deferred {
		msg = "Training week review deferred: adaptation will be retried"
	}
	slog.InfoContext(ctx, msg, attrs...)
	return err == nil
}

// plan drafts the roadmap's next block; it reports false on a failure.
func (w *RoadmapLifecycleWorker) plan(ctx context.Context, roadmapID string) (ok bool) {
	planned, err := w.planner.Handle(ctx, roadmapID)
	switch {
	case err != nil:
		slog.ErrorContext(ctx, "Planning the next block failed; retried later",
			slog.String("roadmap_id", roadmapID), slog.Duration("retry_after", planRetryDelay), slog.Any("error", err))
	case planned != nil:
		slog.InfoContext(ctx, "Next block planned", slog.String("roadmap_id", roadmapID),
			slog.String("planned_roadmap_id", planned.ID()),
			slog.String("starts", planned.Info().StartDate.Format(time.DateOnly)))
	}
	return err == nil
}

// forEachBounded runs job for every id, at most maxConcurrentGenerations at a
// time, each under generationTimeout, and returns when all are done.
func forEachBounded(ctx context.Context, ids []string, job func(context.Context, string)) {
	slots := make(chan struct{}, maxConcurrentGenerations)
	var wg sync.WaitGroup
	for _, id := range ids {
		if ctx.Err() != nil {
			break
		}
		slots <- struct{}{}
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() { <-slots }()
			jobCtx, cancel := context.WithTimeout(ctx, generationTimeout)
			defer cancel()
			job(jobCtx, id)
		}()
	}
	wg.Wait()
}
