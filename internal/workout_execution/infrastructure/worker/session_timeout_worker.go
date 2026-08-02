package worker

import (
	"context"
	"log"
	"time"

	"github.com/viethung213/gym-companion/internal/workout_execution/application/port"
	"github.com/viethung213/gym-companion/internal/workout_execution/domain/repository"
)

// SessionTimeoutWorker periodically scans for active sessions exceeding 240 minutes and auto-aborts them.
type SessionTimeoutWorker struct {
	sessionRepo repository.WorkoutSessionRepository
	outbox      port.OutboxWriter
	txManager   port.TxManager
	interval    time.Duration
}

// NewSessionTimeoutWorker constructs a new SessionTimeoutWorker.
func NewSessionTimeoutWorker(
	sessionRepo repository.WorkoutSessionRepository,
	outbox port.OutboxWriter,
	txManager port.TxManager,
	interval time.Duration,
) *SessionTimeoutWorker {
	if interval <= 0 {
		interval = 10 * time.Minute
	}
	return &SessionTimeoutWorker{
		sessionRepo: sessionRepo,
		outbox:      outbox,
		txManager:   txManager,
		interval:    interval,
	}
}

// Start spawns the background worker process.
func (w *SessionTimeoutWorker) Start(ctx context.Context) {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	log.Printf("[WorkoutExecution] Starting Session Timeout worker (interval: %v)...", w.interval)

	for {
		select {
		case <-ctx.Done():
			log.Println("[WorkoutExecution] Stopping Session Timeout worker.")
			return
		case <-ticker.C:
			if err := w.processTimedOutSessions(ctx); err != nil {
				log.Printf("[WorkoutExecution] Session Timeout worker error: %v", err)
			}
		}
	}
}

func (w *SessionTimeoutWorker) processTimedOutSessions(ctx context.Context) error {
	sessions, err := w.sessionRepo.FindTimedOutSessions(ctx, 240)
	if err != nil || len(sessions) == 0 {
		return err
	}

	log.Printf("[WorkoutExecution] Found %d sessions exceeding 240 minutes timeout.", len(sessions))
	now := time.Now().UTC()

	for _, session := range sessions {
		if session.CheckTimeoutAndAutoAbort(now) {
			saveFunc := func(txCtx context.Context) error {
				if err := w.sessionRepo.Save(txCtx, session); err != nil {
					return err
				}
				events := session.PopEvents()
				if len(events) > 0 && w.outbox != nil {
					return w.outbox.WriteEvents(txCtx, "WorkoutSession", session.ID(), events)
				}
				return nil
			}

			var txErr error
			if w.txManager != nil {
				txErr = w.txManager.WithTransaction(ctx, saveFunc)
			} else {
				txErr = saveFunc(ctx)
			}
			if txErr != nil {
				log.Printf("[WorkoutExecution] Failed to save timed out session %s: %v", session.ID(), txErr)
			}
		}
	}

	return nil
}
