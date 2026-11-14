package worker

import (
	"context"
	"log/slog"
	"time"

	"github.com/viethung213/gym-companion/internal/workout_execution/application/port"
	"github.com/viethung213/gym-companion/internal/workout_execution/domain/repository"
)

// SessionTimeoutWorker periodically scans for active sessions exceeding 240 minutes and auto-aborts them.
type SessionTimeoutWorker struct {
	sessionRepo   repository.WorkoutSessionRepository
	eventRecorder port.EventRecorder
	txManager     port.TxManager
	interval      time.Duration
}

// NewSessionTimeoutWorker constructs a new SessionTimeoutWorker.
func NewSessionTimeoutWorker(
	sessionRepo repository.WorkoutSessionRepository,
	eventRecorder port.EventRecorder,
	txManager port.TxManager,
	interval time.Duration,
) *SessionTimeoutWorker {
	if interval <= 0 {
		interval = 10 * time.Minute
	}
	return &SessionTimeoutWorker{
		sessionRepo:   sessionRepo,
		eventRecorder: eventRecorder,
		txManager:     txManager,
		interval:      interval,
	}
}

// Start spawns the background worker process.
func (w *SessionTimeoutWorker) Start(ctx context.Context) {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	slog.InfoContext(ctx, "Starting WorkoutExecution Session Timeout worker", slog.Duration("interval", w.interval))

	for {
		select {
		case <-ctx.Done():
			slog.InfoContext(ctx, "Stopping WorkoutExecution Session Timeout worker")
			return
		case <-ticker.C:
			func() {
				defer func() {
					if r := recover(); r != nil {
						slog.ErrorContext(ctx, "PANIC RECOVERED in Session Timeout worker tick", slog.Any("panic", r))
					}
				}()
				if err := w.processTimedOutSessions(ctx); err != nil {
					slog.ErrorContext(ctx, "Session Timeout worker error", slog.String("error", err.Error()))
				}
			}()
		}
	}
}

func (w *SessionTimeoutWorker) processTimedOutSessions(ctx context.Context) error {
	sessions, err := w.sessionRepo.FindTimedOutSessions(ctx, 240)
	if err != nil || len(sessions) == 0 {
		return err
	}

	slog.InfoContext(ctx, "Found sessions exceeding 240 minutes timeout", slog.Int("count", len(sessions)))
	now := time.Now().UTC()

	for _, session := range sessions {
		if session.CheckTimeoutAndAutoAbort(now) {
			saveFunc := func(txCtx context.Context) error {
				if err := w.sessionRepo.Save(txCtx, session); err != nil {
					return err
				}
				events := session.PopEvents()
				if len(events) > 0 && w.eventRecorder != nil {
					return w.eventRecorder.RecordEvents(txCtx, events)
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
				slog.ErrorContext(ctx, "Failed to save timed out session",
					slog.String("session_id", session.ID()),
					slog.String("error", txErr.Error()),
				)
			}
		}
	}

	return nil
}
