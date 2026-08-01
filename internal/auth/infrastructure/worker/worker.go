package worker

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/viethung213/gym-companion/internal/auth/application/port"
)

// OutboxWorker regularly polls the database outbox table and publishes events to Kafka.
type OutboxWorker struct {
	outboxRepo port.OutboxRepository
	publisher  port.BrokerPublisher
	interval   time.Duration
}

// NewOutboxWorker constructs a new OutboxWorker instance.
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

// Start spawns the background worker process. It stops when the context is cancelled.
func (w *OutboxWorker) Start(ctx context.Context) {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	log.Printf("Starting Outbox background worker (polling interval: %v)...", w.interval)

	for {
		select {
		case <-ctx.Done():
			log.Println("Stopping Outbox background worker due to context cancellation.")
			return
		case <-ticker.C:
			if err := w.processOutbox(ctx); err != nil {
				log.Printf("Outbox worker processing error: %v", err)
			}
		}
	}
}

func (w *OutboxWorker) processOutbox(ctx context.Context) error {
	return w.outboxRepo.ProcessBatch(ctx, 500, func(publishCtx context.Context, events []*port.OutboxRecord) error {
		log.Printf("Outbox worker: found %d unpublished events to process.", len(events))
		if err := w.publisher.PublishBatch(publishCtx, events); err != nil {
			return fmt.Errorf("publish batch failed: %w", err)
		}
		log.Printf("Outbox worker: successfully published batch of %d events.", len(events))
		return nil
	})
}
