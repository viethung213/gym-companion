package worker

import (
	"context"
	"log"
	"time"

	"github.com/viethung213/gym-companion/internal/workout_execution/application/port"
	"github.com/viethung213/gym-companion/internal/workout_execution/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/workout_execution/domain/policy"
	"github.com/viethung213/gym-companion/internal/workout_execution/domain/repository"
)

const defaultCriticalInactivityThreshold = 5 * time.Minute
const defaultCriticalInactivityInterval = 1 * time.Minute

// CriticalInactivityWorker periodically scans IN_PROGRESS sessions that had a
// critical posture error and received no further interaction for the configured
// inactivity threshold, then marks them as ANOMALOUS and publishes a domain event.
type CriticalInactivityWorker struct {
	sessionRepo         repository.WorkoutSessionRepository
	outbox              port.OutboxWriter
	txManager           port.TxManager
	interval            time.Duration
	inactivityThreshold time.Duration
}

// NewCriticalInactivityWorker constructs a CriticalInactivityWorker.
// interval is how often the scan runs; inactivityThreshold is how long without
// interaction (after a critical error) before the session is marked ANOMALOUS.
// Zero or negative values fall back to sensible defaults.
func NewCriticalInactivityWorker(
	sessionRepo repository.WorkoutSessionRepository,
	outbox port.OutboxWriter,
	txManager port.TxManager,
	interval time.Duration,
	inactivityThreshold time.Duration,
) *CriticalInactivityWorker {
	if interval <= 0 {
		interval = defaultCriticalInactivityInterval
	}
	if inactivityThreshold <= 0 {
		inactivityThreshold = defaultCriticalInactivityThreshold
	}
	return &CriticalInactivityWorker{
		sessionRepo:         sessionRepo,
		outbox:              outbox,
		txManager:           txManager,
		interval:            interval,
		inactivityThreshold: inactivityThreshold,
	}
}

// Start spawns the background worker process. It blocks until ctx is cancelled.
func (w *CriticalInactivityWorker) Start(ctx context.Context) {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	log.Printf(
		"[WorkoutExecution] Starting Critical Inactivity worker (interval: %v, threshold: %v)...",
		w.interval, w.inactivityThreshold,
	)

	for {
		select {
		case <-ctx.Done():
			log.Println("[WorkoutExecution] Stopping Critical Inactivity worker.")
			return
		case <-ticker.C:
			if err := w.processCriticalInactiveSessions(ctx); err != nil {
				log.Printf("[WorkoutExecution] Critical Inactivity worker error: %v", err)
			}
		}
	}
}

func (w *CriticalInactivityWorker) processCriticalInactiveSessions(ctx context.Context) error {
	sessions, err := w.sessionRepo.FindSessionsWithCriticalInactivity(ctx, w.inactivityThreshold)
	if err != nil || len(sessions) == 0 {
		return err
	}

	log.Printf("[WorkoutExecution] Found %d sessions with critical inactivity.", len(sessions))
	now := time.Now().UTC()

	for _, session := range sessions {
		lastCriticalAt := findLastCriticalErrorTime(session)
		if err := session.MarkCriticalInactivity(now, lastCriticalAt); err != nil {
			log.Printf("[WorkoutExecution] Could not mark session %q as critically inactive: %v",
				session.ID(), err)
			continue
		}

		_ = w.txManager.WithTransaction(ctx, func(txCtx context.Context) error {
			if err := w.sessionRepo.Save(txCtx, session); err != nil {
				return err
			}
			events := session.PopEvents()
			if len(events) > 0 && w.outbox != nil {
				return w.outbox.WriteEvents(txCtx, "WorkoutSession", session.ID(), events)
			}
			return nil
		})
	}

	return nil
}

// findLastCriticalErrorTime returns the timestamp of the most recent critical
// error in the session's error list, or the zero time if none found.
func findLastCriticalErrorTime(session *aggregate.WorkoutSession) time.Time {
	var last time.Time
	for _, e := range session.Errors() {
		if policy.IsCritical(e) && e.Timestamp.After(last) {
			last = e.Timestamp
		}
	}
	return last
}
