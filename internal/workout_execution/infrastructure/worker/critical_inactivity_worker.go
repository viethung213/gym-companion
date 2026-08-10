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
			func() {
				defer func() {
					if r := recover(); r != nil {
						log.Printf("[WorkoutExecution] PANIC RECOVERED in Critical Inactivity worker tick: %v", r)
					}
				}()
				if err := w.processCriticalInactiveSessions(ctx); err != nil {
					log.Printf("[WorkoutExecution] Critical Inactivity worker error: %v", err)
				}
			}()
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
		persists, lastCriticalAt := hasCriticalErrorPersistence(session, w.inactivityThreshold, now)
		if !persists {
			continue
		}

		if err := session.MarkCriticalInactivity(now, lastCriticalAt); err != nil {
			log.Printf("[WorkoutExecution] Could not mark session %q as critically inactive: %v",
				session.ID(), err)
			continue
		}

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
			log.Printf("[WorkoutExecution] Failed to save critically inactive session %s: %v", session.ID(), txErr)
		}
	}

	return nil
}

// hasCriticalErrorPersistence checks if a session has a single critical error code that persisted
// or repeated continuously (at least 3 occurrences spanning at least 60% of threshold) in the recent window.
func hasCriticalErrorPersistence(session *aggregate.WorkoutSession, threshold time.Duration, now time.Time) (bool, time.Time) {
	windowStart := now.Add(-threshold)
	minSpan := threshold * 6 / 10 // e.g. 3 minutes for a 5-minute threshold

	// Group recent critical errors by ErrorCode
	errorsByCode := make(map[string][]time.Time)
	var latestOverallCritical time.Time

	errs := session.Errors()
	for i := range errs {
		e := &errs[i]
		if policy.IsCritical(e) {
			if e.Timestamp.After(latestOverallCritical) {
				latestOverallCritical = e.Timestamp
			}
			if !e.Timestamp.Before(windowStart) && !e.Timestamp.After(now) {
				code := e.ErrorCode
				if code == "" {
					code = "CRITICAL_POSTURE_ERROR"
				}
				errorsByCode[code] = append(errorsByCode[code], e.Timestamp)
			}
		}
	}

	for _, timestamps := range errorsByCode {
		if len(timestamps) >= 3 {
			var first, last time.Time
			for _, ts := range timestamps {
				if first.IsZero() || ts.Before(first) {
					first = ts
				}
				if last.IsZero() || ts.After(last) {
					last = ts
				}
			}
			if last.Sub(first) >= minSpan {
				return true, last
			}
		}
	}

	return false, latestOverallCritical
}

// findLastCriticalErrorTime returns the timestamp of the most recent critical
// error in the session's error list, or the zero time if none found.
func findLastCriticalErrorTime(session *aggregate.WorkoutSession) time.Time {
	var last time.Time
	errs := session.Errors()
	for i := range errs {
		e := &errs[i]
		if policy.IsCritical(e) && e.Timestamp.After(last) {
			last = e.Timestamp
		}
	}
	return last
}
