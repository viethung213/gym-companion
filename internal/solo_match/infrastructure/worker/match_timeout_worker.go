package worker

import (
	"context"
	"log"
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

	log.Printf("[SoloMatch] Starting Match Timeout worker (interval: %v)...", w.interval)

	for {
		select {
		case <-ctx.Done():
			log.Println("[SoloMatch] Stopping Match Timeout worker.")
			return
		case <-ticker.C:
			func() {
				defer func() {
					if r := recover(); r != nil {
						log.Printf("[SoloMatch] PANIC RECOVERED in Match Timeout worker tick: %v", r)
					}
				}()
				if err := w.handler.Handle(ctx, command.HandleMatchTimeoutCommand{}); err != nil {
					log.Printf("[SoloMatch] Timeout worker sweep error: %v", err)
				}
			}()
		}
	}
}
