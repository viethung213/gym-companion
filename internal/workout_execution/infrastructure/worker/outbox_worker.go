package worker

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/viethung213/gym-companion/internal/workout_execution/application/port"
)

// OutboxWorker regularly polls the database outbox table and publishes events to message broker.
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
	if interval <= 0 {
		interval = 5 * time.Second
	}
	return &OutboxWorker{
		outboxRepo: outboxRepo,
		publisher:  publisher,
		interval:   interval,
	}
}

// Start runs the background worker until context is cancelled.
func (w *OutboxWorker) Start(ctx context.Context) {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	log.Printf("[WorkoutExecution] Starting Outbox worker (polling: %v)...", w.interval)

	for {
		select {
		case <-ctx.Done():
			log.Println("[WorkoutExecution] Stopping Outbox worker due to context cancellation.")
			return
		case <-ticker.C:
			if err := w.processOutbox(ctx); err != nil {
				log.Printf("[WorkoutExecution] Outbox worker processing error: %v", err)
			}
		}
	}
}

func (w *OutboxWorker) processOutbox(ctx context.Context) error {
	var events []*port.OutboxRecord

	// Lock key 99887766 for workout_execution outbox
	err := w.outboxRepo.ExecuteInLock(ctx, 99887766, func(txCtx context.Context) error {
		var err error
		events, err = w.outboxRepo.FetchUnpublished(txCtx, 100)
		return err
	})
	if err != nil {
		return err
	}

	if len(events) == 0 {
		return nil
	}

	if w.publisher != nil {
		if err := w.publisher.PublishBatch(ctx, events); err != nil {
			return fmt.Errorf("publish batch failed: %w", err)
		}
	}

	ids := make([]string, len(events))
	for i, ev := range events {
		ids[i] = ev.ID
	}

	return w.outboxRepo.ExecuteInLock(ctx, 99887766, func(txCtx context.Context) error {
		return w.outboxRepo.MarkPublished(txCtx, ids)
	})
}
