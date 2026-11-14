package worker

import (
	"context"
	"log/slog"
	"time"

	"github.com/viethung213/gym-companion/internal/solo_match/application/command"
)

// MatchTimeoutWorker periodically sweeps unready matches exceeding 60s check-in deadline (CancelOnNoShow).
type MatchTimeoutWorker struct {
	handler  *command.HandleMatchTimeoutHandler
	interval time.Duration
}

// NewMatchTimeoutWorker constructs a new MatchTimeoutWorker.
func NewMatchTimeoutWorker(
	handler *command.HandleMatchTimeoutHandler,
	interval time.Duration,
) *MatchTimeoutWorker {
	if interval <= 0 {
		interval = 15 * time.Second
	}
	return &MatchTimeoutWorker{
		handler:  handler,
		interval: interval,
	}
}

// Start begins the periodic timeout sweep loop until context is cancelled.
func (w *MatchTimeoutWorker) Start(ctx context.Context) {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	slog.InfoContext(ctx, "Starting SoloMatch Match Timeout worker", slog.Duration("interval", w.interval))

	for {
		select {
		case <-ctx.Done():
			slog.InfoContext(ctx, "Stopping SoloMatch Match Timeout worker")
			return
		case <-ticker.C:
			func() {
				defer func() {
					if r := recover(); r != nil {
						slog.ErrorContext(ctx, "PANIC RECOVERED in Match Timeout worker tick", slog.Any("panic", r))
					}
				}()
				if err := w.handler.Handle(ctx, command.HandleMatchTimeoutCommand{}); err != nil {
					slog.ErrorContext(ctx, "SoloMatch timeout worker sweep error", slog.String("error", err.Error()))
				}
			}()
		}
	}
}
