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
	var events []*port.OutboxRecord

	// 1. Fetch unpublished events (max 500) inside a transaction with an Advisory Lock
	err := w.outboxRepo.ExecuteInLock(ctx, 11223344, func(txCtx context.Context) error {
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

	log.Printf("Outbox worker: found %d unpublished events to process.", len(events))

	// 2. Batch publish to Kafka (OUTSIDE DB transaction, using parent context)
	err = w.publisher.PublishBatch(ctx, events)
	if err != nil {
		return fmt.Errorf("publish batch failed: %w", err)
	}

	// 3. Batch mark as published in the database inside another transaction
	ids := make([]string, len(events))
	for i, ev := range events {
		ids[i] = ev.ID
	}
	err = w.outboxRepo.ExecuteInLock(ctx, 11223344, func(txCtx context.Context) error {
		return w.outboxRepo.MarkPublished(txCtx, ids)
	})
	if err != nil {
		return fmt.Errorf("mark batch published failed: %w", err)
	}

	log.Printf("Outbox worker: successfully published batch of %d events.", len(events))
	return nil
}
