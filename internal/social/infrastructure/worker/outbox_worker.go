package worker

import (
	"context"
	"log"
	"time"

	"github.com/viethung213/gym-companion/internal/social/application/port"
	"github.com/viethung213/gym-companion/internal/social/infrastructure/kafka"
)

type OutboxWorker struct {
	outboxRepo port.OutboxRepository
	publisher  *kafka.Publisher
	interval   time.Duration
}

func NewOutboxWorker(
	outboxRepo port.OutboxRepository,
	publisher *kafka.Publisher,
	interval time.Duration,
) *OutboxWorker {
	return &OutboxWorker{
		outboxRepo: outboxRepo,
		publisher:  publisher,
		interval:   interval,
	}
}

func (w *OutboxWorker) Start(ctx context.Context) {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	log.Printf("Starting Social Outbox background worker (interval: %v)...", w.interval)

	for {
		select {
		case <-ctx.Done():
			log.Println("Stopping Social Outbox background worker due to context cancellation.")
			return
		case <-ticker.C:
			if err := w.processOutbox(ctx); err != nil {
				log.Printf("Social outbox worker processing error: %v", err)
			}
		}
	}
}

func (w *OutboxWorker) processOutbox(ctx context.Context) error {
	records, err := w.outboxRepo.ClaimBatch(ctx, 50, 10*time.Second)
	if err != nil {
		return err
	}
	if len(records) == 0 {
		return nil
	}

	if err := w.publisher.PublishBatch(ctx, records); err != nil {
		return err
	}

	ids := make([]string, len(records))
	for i, r := range records {
		ids[i] = r.ID
	}

	return w.outboxRepo.MarkAsPublished(ctx, ids)
}
