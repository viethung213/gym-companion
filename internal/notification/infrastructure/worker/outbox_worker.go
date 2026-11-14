package worker

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/viethung213/gym-companion/internal/notification/application/port"
)

type OutboxWorker struct {
	outboxRepo    port.OutboxRepository
	outboxLogRepo port.OutboxLogRepository
	publisher     port.BrokerPublisher
	interval      time.Duration
}

func NewOutboxWorker(
	outboxRepo port.OutboxRepository,
	outboxLogRepo port.OutboxLogRepository,
	publisher port.BrokerPublisher,
	interval time.Duration,
) *OutboxWorker {
	if interval <= 0 {
		interval = 2 * time.Second
	}
	return &OutboxWorker{
		outboxRepo:    outboxRepo,
		outboxLogRepo: outboxLogRepo,
		publisher:     publisher,
		interval:      interval,
	}
}

func (w *OutboxWorker) Start(ctx context.Context) error {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	slog.InfoContext(ctx, "Started Notification OutboxWorker", slog.Duration("interval", w.interval))

	for {
		select {
		case <-ctx.Done():
			slog.InfoContext(ctx, "Stopping Notification OutboxWorker due to context cancellation")
			return nil
		case <-ticker.C:
			if err := w.processOutbox(ctx); err != nil {
				slog.ErrorContext(ctx, "Notification OutboxWorker processing error", slog.String("error", err.Error()))
			}
		}
	}
}

func (w *OutboxWorker) processOutbox(ctx context.Context) error {
	if w.outboxRepo == nil {
		return nil
	}

	return w.outboxRepo.ProcessBatch(ctx, 100, func(publishCtx context.Context, records []*port.OutboxRecord) error {
		if w.publisher != nil {
			if err := w.publisher.PublishBatch(publishCtx, records); err != nil {
				for _, rec := range records {
					if w.outboxLogRepo != nil {
						_ = w.outboxLogRepo.SaveLog(publishCtx, &port.OutboxLogRecord{
							ID:           uuid.New().String(),
							EventID:      rec.EventID,
							EventType:    rec.EventType,
							Payload:      rec.Payload,
							PartitionKey: rec.PartitionKey,
							ProcessedAt:  time.Now().UTC(),
							Status:       "FAILED",
							ErrorMessage: err.Error(),
						})
					}
				}
				return fmt.Errorf("publish notification outbox batch to kafka: %w", err)
			}
		}

		for _, rec := range records {
			if w.outboxLogRepo != nil {
				_ = w.outboxLogRepo.SaveLog(publishCtx, &port.OutboxLogRecord{
					ID:           uuid.New().String(),
					EventID:      rec.EventID,
					EventType:    rec.EventType,
					Payload:      rec.Payload,
					PartitionKey: rec.PartitionKey,
					ProcessedAt:  time.Now().UTC(),
					Status:       "PROCESSED",
					ErrorMessage: "",
				})
			}
		}

		slog.DebugContext(publishCtx, "Notification OutboxWorker successfully published batch", slog.Int("count", len(records)))
		return nil
	})
}
