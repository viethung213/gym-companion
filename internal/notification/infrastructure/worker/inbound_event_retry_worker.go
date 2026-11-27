package worker

import (
	"context"
	"log/slog"
	"time"

	"github.com/viethung213/gym-companion/internal/notification/application/port"
)

// MessageDispatcher processes raw CloudEvent JSON payloads across all supported notification channels.
type MessageDispatcher interface {
	ProcessMessage(ctx context.Context, msgValue []byte) error
}

type InboundEventRetryWorker struct {
	outboxLogRepo port.OutboxLogRepository
	dispatcher    MessageDispatcher
	interval      time.Duration
}

func NewInboundEventRetryWorker(
	outboxLogRepo port.OutboxLogRepository,
	dispatcher MessageDispatcher,
	interval time.Duration,
) *InboundEventRetryWorker {
	if interval <= 0 {
		interval = 10 * time.Second
	}
	return &InboundEventRetryWorker{
		outboxLogRepo: outboxLogRepo,
		dispatcher:    dispatcher,
		interval:      interval,
	}
}

func (w *InboundEventRetryWorker) Start(ctx context.Context) error {
	defer func() {
		if r := recover(); r != nil {
			slog.ErrorContext(ctx, "Panic recovered in InboundEventRetryWorker", slog.Any("panic", r))
		}
	}()

	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	slog.InfoContext(ctx, "Started Notification InboundEventRetryWorker", slog.Duration("interval", w.interval))

	for {
		select {
		case <-ctx.Done():
			slog.InfoContext(ctx, "Stopping Notification InboundEventRetryWorker due to context cancellation")
			return nil
		case <-ticker.C:
			if err := w.processFailedLogs(ctx); err != nil {
				slog.ErrorContext(ctx, "Notification InboundEventRetryWorker processing error", slog.String("error", err.Error()))
			}
		}
	}
}

func (w *InboundEventRetryWorker) processFailedLogs(ctx context.Context) error {
	if w.outboxLogRepo == nil || w.dispatcher == nil {
		return nil
	}

	failedLogs, err := w.outboxLogRepo.FetchFailedLogs(ctx, 50)
	if err != nil {
		return err
	}

	if len(failedLogs) == 0 {
		return nil
	}

	slog.InfoContext(ctx, "Found failed inbound events to retry", slog.Int("count", len(failedLogs)))

	for _, logRec := range failedLogs {
		sendErr := w.dispatcher.ProcessMessage(ctx, logRec.Payload)
		if sendErr != nil {
			nextCount := logRec.RetryCount + 1
			switch {
			case nextCount >= 3:
				slog.ErrorContext(ctx, "Inbound event retry exhausted max attempts (3), moving to DEAD_LETTER",
					slog.String("log_id", logRec.ID),
					slog.String("event_id", logRec.EventID),
					slog.Int("attempts", nextCount),
					slog.String("error", sendErr.Error()),
				)
				_ = w.outboxLogRepo.UpdateRetryLog(ctx, logRec.ID, nextCount, time.Now().UTC(), "DEAD_LETTER", sendErr.Error())
			case nextCount == 1:
				nextRetry := time.Now().UTC().Add(10 * time.Second)
				slog.WarnContext(ctx, "Inbound event retry failed (attempt 1), scheduling next retry in 10s",
					slog.String("log_id", logRec.ID),
					slog.String("error", sendErr.Error()),
				)
				_ = w.outboxLogRepo.UpdateRetryLog(ctx, logRec.ID, nextCount, nextRetry, "FAILED", sendErr.Error())
			default:
				nextRetry := time.Now().UTC().Add(30 * time.Second)
				slog.WarnContext(ctx, "Inbound event retry failed (attempt 2), scheduling next retry in 30s",
					slog.String("log_id", logRec.ID),
					slog.String("error", sendErr.Error()),
				)
				_ = w.outboxLogRepo.UpdateRetryLog(ctx, logRec.ID, nextCount, nextRetry, "FAILED", sendErr.Error())
			}
		} else {
			slog.InfoContext(ctx, "Retry SUCCESS for inbound log", slog.String("log_id", logRec.ID), slog.String("event_id", logRec.EventID))
			_ = w.outboxLogRepo.UpdateLogStatus(ctx, logRec.ID, "SUCCESS", "")
		}
	}

	return nil
}
