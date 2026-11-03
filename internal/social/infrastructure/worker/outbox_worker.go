package worker

import (
	"context"
	"log"
	"time"

	"github.com/viethung213/gym-companion/internal/social/application/port"
)

type OutboxWorker struct {
	outboxRepo port.OutboxRepository
	publisher  port.BrokerPublisher
	interval   time.Duration
}

func NewOutboxWorker(
	outboxRepo port.OutboxRepository,
	publisher port.BrokerPublisher,
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
	return w.outboxRepo.ProcessBatch(ctx, 50, func(publishCtx context.Context, records []*port.OutboxRecord) error {
		return w.publisher.PublishBatch(publishCtx, records)
	})
}
