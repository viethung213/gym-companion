package worker

import (
	"context"
	"fmt"
	"log"
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

	log.Printf("[Nutrition OutboxWorker] Started background worker (interval: %v)...", w.interval)

	for {
		select {
		case <-ctx.Done():
			log.Println("[Nutrition OutboxWorker] Stopping worker due to context cancellation.")
			return nil
		case <-ticker.C:
			if err := w.processOutbox(ctx); err != nil {
				log.Printf("[Nutrition OutboxWorker] Processing error: %v", err)
			}
		}
	}
}

func (w *OutboxWorker) processOutbox(ctx context.Context) error {
	return w.outboxRepo.ProcessBatch(ctx, 100, func(publishCtx context.Context, records []port.OutboxRecord) error {
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
							Status:       "FAILED",
							ErrorMessage: err.Error(),
						})
					}
				}
				return fmt.Errorf("publish outbox batch to kafka: %w", err)
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
					Status:       "PROCESSED",
					ErrorMessage: "",
				})
			}
		}

		log.Printf("[Nutrition OutboxWorker] Successfully published batch of %d events.", len(records))
		return nil
	})
}
