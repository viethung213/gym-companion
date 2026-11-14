package worker

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/viethung213/gym-companion/internal/nutrition/application/port"
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

	slog.InfoContext(ctx, "Started Nutrition OutboxWorker", slog.Duration("interval", w.interval))

	for {
		select {
		case <-ctx.Done():
			slog.InfoContext(ctx, "Stopping Nutrition OutboxWorker due to context cancellation")
			return nil
		case <-ticker.C:
			if err := w.processOutbox(ctx); err != nil {
				slog.ErrorContext(ctx, "Nutrition OutboxWorker processing error", slog.String("error", err.Error()))
			}
		}
	}
}

func (w *OutboxWorker) processOutbox(ctx context.Context) error {
	return w.outboxRepo.ProcessBatch(ctx, 100, func(publishCtx context.Context, records []port.OutboxRecord) error {
		if w.publisher != nil {
			if err := w.publisher.PublishBatch(publishCtx, records); err != nil {
				for i := range records {
					rec := &records[i]
					if w.outboxLogRepo != nil {
						_ = w.outboxLogRepo.SaveLog(publishCtx, &port.OutboxLogRecord{
							ID:           uuid.New().String(),
							EventID:      rec.EventID,
							EventType:    rec.EventType,
							Payload:      rec.Payload,
							PartitionKey: rec.PartitionKey,
							Status:       "FAILED",
							ErrorMessage: err.Error(),
						})
					}
				}
				return fmt.Errorf("publish outbox batch to kafka: %w", err)
			}
		}

		for i := range records {
			rec := &records[i]
			if w.outboxLogRepo != nil {
				_ = w.outboxLogRepo.SaveLog(publishCtx, &port.OutboxLogRecord{
					ID:           uuid.New().String(),
					EventID:      rec.EventID,
					EventType:    rec.EventType,
					Payload:      rec.Payload,
					PartitionKey: rec.PartitionKey,
					Status:       "PROCESSED",
					ErrorMessage: "",
				})
			}
		}

		slog.DebugContext(publishCtx, "Nutrition OutboxWorker successfully published batch", slog.Int("count", len(records)))
		return nil
	})
}
