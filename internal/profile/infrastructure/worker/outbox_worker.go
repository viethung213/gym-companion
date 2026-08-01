package worker

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/viethung213/gym-companion/internal/profile/application/port"
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

	log.Printf("Starting Profile Outbox background worker (interval: %v)...", w.interval)

	for {
		select {
		case <-ctx.Done():
			log.Println("Stopping Profile Outbox background worker due to context cancellation.")
			return
		case <-ticker.C:
			if err := w.processOutbox(ctx); err != nil {
				log.Printf("Profile outbox worker processing error: %v", err)
			}
		}
	}
}

func (w *OutboxWorker) processOutbox(ctx context.Context) error {
	return w.outboxRepo.ProcessBatch(ctx, 100, func(publishCtx context.Context, events []*port.OutboxRecord) error {
		if err := w.publisher.PublishBatch(publishCtx, events); err != nil {
			return fmt.Errorf("publish outbox batch to kafka: %w", err)
		}
		log.Printf("Profile Outbox worker: successfully published batch of %d events.", len(events))
		return nil
	})
}
