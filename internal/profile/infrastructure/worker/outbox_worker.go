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
	events, err := w.outboxRepo.FetchUnpublished(ctx, 100)
	if err != nil {
		return fmt.Errorf("fetch unpublished events: %w", err)
	}

	if len(events) == 0 {
		return nil
	}

	if err := w.publisher.PublishBatch(ctx, events); err != nil {
		return fmt.Errorf("publish outbox batch to kafka: %w", err)
	}

	ids := make([]string, len(events))
	for i, ev := range events {
		ids[i] = ev.ID
	}

	if err := w.outboxRepo.MarkAsPublished(ctx, ids); err != nil {
		return fmt.Errorf("mark outbox events as published: %w", err)
	}

	log.Printf("Profile Outbox worker: successfully published batch of %d events.", len(events))
	return nil
}
