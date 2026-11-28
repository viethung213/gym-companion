package worker

import (
	"context"
	"log/slog"
	"time"

	"github.com/viethung213/gym-companion/internal/gamification/application/command"
)

const (
	defaultLeaderboardReconcileInterval = 15 * time.Minute
	defaultLeaderboardRebuildTimeout    = 10 * time.Minute
)

// LeaderboardRebuilder rebuilds the leaderboard read model.
type LeaderboardRebuilder interface {
	Handle(
		ctx context.Context,
		cmd command.RebuildLeaderboardCommand,
	) (command.RebuildLeaderboardResult, error)
}

// LeaderboardReconcileWorker keeps the leaderboard read model converged with the source of
// truth: a full reset on startup (until one succeeds), a reconcile every interval, and an early
// run on request. Each run is bounded by runTimeout so a hung dependency cannot stall the worker.
type LeaderboardReconcileWorker struct {
	rebuilder  LeaderboardRebuilder
	interval   time.Duration
	runTimeout time.Duration
	trigger    chan struct{}
}

// NewLeaderboardReconcileWorker constructs the worker; interval <= 0 uses 15 minutes and
// runTimeout <= 0 uses 10 minutes.
func NewLeaderboardReconcileWorker(
	rebuilder LeaderboardRebuilder,
	interval time.Duration,
	runTimeout time.Duration,
) *LeaderboardReconcileWorker {
	if interval <= 0 {
		interval = defaultLeaderboardReconcileInterval
	}
	if runTimeout <= 0 {
		runTimeout = defaultLeaderboardRebuildTimeout
	}
	return &LeaderboardReconcileWorker{
		rebuilder:  rebuilder,
		interval:   interval,
		runTimeout: runTimeout,
		trigger:    make(chan struct{}, 1),
	}
}

// RequestRebuild asks for an early run without blocking; requests coalesce while one is pending.
func (w *LeaderboardReconcileWorker) RequestRebuild() {
	select {
	case w.trigger <- struct{}{}:
	default:
	}
}

// Start runs the worker until ctx is cancelled.
func (w *LeaderboardReconcileWorker) Start(ctx context.Context) {
	slog.InfoContext(ctx, "Starting Gamification leaderboard reconcile worker",
		slog.String("module", "gamification"), slog.Duration("interval", w.interval))

	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	resetPending := w.run(ctx, true)
	for {
		select {
		case <-ctx.Done():
			slog.InfoContext(ctx, "Stopping Gamification leaderboard reconcile worker",
				slog.String("module", "gamification"))
			return
		case <-ticker.C:
			resetPending = w.run(ctx, resetPending)
		case <-w.trigger:
			resetPending = w.run(ctx, resetPending)
		}
	}
}

// run performs one rebuild and reports whether a reset is still pending afterwards.
func (w *LeaderboardReconcileWorker) run(ctx context.Context, reset bool) (resetPending bool) {
	resetPending = reset
	defer func() {
		if r := recover(); r != nil {
			slog.ErrorContext(ctx, "PANIC RECOVERED in Gamification leaderboard reconcile worker",
				slog.String("module", "gamification"), slog.Any("panic", r))
		}
	}()

	runCtx, cancel := context.WithTimeout(ctx, w.runTimeout)
	defer cancel()

	start := time.Now()
	res, err := w.rebuilder.Handle(runCtx, command.RebuildLeaderboardCommand{Reset: reset})
	switch {
	case err != nil:
		if ctx.Err() == nil {
			slog.ErrorContext(ctx, "Leaderboard reconcile failed",
				slog.String("module", "gamification"),
				slog.Bool("reset", reset),
				slog.String("error", err.Error()))
		}
		return resetPending
	case res.Skipped:
		slog.InfoContext(ctx, "Leaderboard reconcile skipped: another instance holds the rebuild lock",
			slog.String("module", "gamification"), slog.Bool("reset", reset))
		return resetPending
	}

	slog.InfoContext(ctx, "Leaderboard reconcile completed",
		slog.String("module", "gamification"),
		slog.Bool("reset", reset),
		slog.Int("projected", res.Projected),
		slog.Duration("took", time.Since(start)))
	w.discardPendingRequest()
	return false
}

// discardPendingRequest drops a request raised while the run that just succeeded was in progress
// (reads find the board not ready until it is marked ready), so it does not start a redundant
// full rebuild. A board that is still not ready raises a new request on the next read.
func (w *LeaderboardReconcileWorker) discardPendingRequest() {
	select {
	case <-w.trigger:
	default:
	}
}
