package worker

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/viethung213/gym-companion/internal/gamification/application/port"
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

	log.Printf("[Gamification] Starting Outbox worker (polling interval: %v)...", w.interval)

	for {
		select {
		case <-ctx.Done():
			log.Println("[Gamification] Stopping Outbox worker due to context cancellation.")
			return
		case <-ticker.C:
			func() {
				defer func() {
					if r := recover(); r != nil {
						log.Printf("[Gamification] PANIC RECOVERED in Outbox worker tick: %v", r)
					}
				}()
				if err := w.processOutbox(ctx); err != nil {
					log.Printf("[Gamification] Outbox worker processing error: %v", err)
				}
			}()
		}
	}
}

func (w *OutboxWorker) processOutbox(ctx context.Context) error {
	if w.publisher == nil {
		return fmt.Errorf("outbox broker publisher is nil, cannot process outbox batch")
	}
	return w.outboxRepo.ProcessBatch(ctx, 100, func(publishCtx context.Context, events []*port.OutboxRecord) error {
		if err := w.publisher.PublishBatch(publishCtx, events); err != nil {
			return fmt.Errorf("publish batch failed: %w", err)
		}
		return nil
	})
}
