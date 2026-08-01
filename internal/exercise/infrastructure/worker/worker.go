package worker

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/viethung213/gym-companion/internal/exercise/application/port"
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

	log.Printf("Starting Exercise Outbox background worker (polling interval: %v)...", w.interval)

	for {
		select {
		case <-ctx.Done():
			log.Println("Stopping Exercise Outbox background worker due to context cancellation.")
			return
		case <-ticker.C:
			if err := w.processOutbox(ctx); err != nil {
				log.Printf("Exercise Outbox worker processing error: %v", err)
			}
		}
	}
}

func (w *OutboxWorker) processOutbox(ctx context.Context) error {
	return w.outboxRepo.ProcessBatch(ctx, 500, func(publishCtx context.Context, events []*port.OutboxRecord) error {
		log.Printf("Exercise Outbox worker: found %d unpublished events to process.", len(events))
		if err := w.publisher.PublishBatch(publishCtx, events); err != nil {
			return fmt.Errorf("publish batch failed: %w", err)
		}
		log.Printf("Exercise Outbox worker: successfully published batch of %d events.", len(events))
		return nil
	})
}
