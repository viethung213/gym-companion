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
	var events []*port.OutboxRecord

	// Fetch unpublished events (max 500) inside transaction with Advisory Lock (ID: 55667788)
	err := w.outboxRepo.ExecuteInLock(ctx, 55667788, func(txCtx context.Context) error {
		var err error
		events, err = w.outboxRepo.FetchUnpublished(txCtx, 500)
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return err
	}

	if len(events) == 0 {
		return nil
	}

	log.Printf("Exercise Outbox worker: found %d unpublished events to process.", len(events))

	// Publish to Kafka (OUTSIDE DB transaction)
	err = w.publisher.PublishBatch(ctx, events)
	if err != nil {
		return fmt.Errorf("publish batch failed: %w", err)
	}

	// Mark as published in another transaction
	ids := make([]string, len(events))
	for i, ev := range events {
		ids[i] = ev.ID
	}
	err = w.outboxRepo.ExecuteInLock(ctx, 55667788, func(txCtx context.Context) error {
		return w.outboxRepo.MarkPublished(txCtx, ids)
	})
	if err != nil {
		return fmt.Errorf("mark batch published failed: %w", err)
	}

	log.Printf("Exercise Outbox worker: successfully published batch of %d events.", len(events))
	return nil
}
