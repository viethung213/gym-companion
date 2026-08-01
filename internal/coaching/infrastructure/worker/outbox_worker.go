// Package worker contains the coaching OutboxWorker that polls
// coaching.outbox and publishes events to the broker.
package worker

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/viethung213/gym-companion/internal/coaching/application/port"
)

// LockID is the Postgres advisory lock ID reserved for coaching outbox
// serialization. Distinct from workout_execution (99887766).
const LockID int64 = 88776655

// OutboxWorker polls coaching.outbox and publishes to Kafka.
type OutboxWorker struct {
	outbox    port.OutboxRepository
	publisher port.BrokerPublisher
	interval  time.Duration
	batchSize int
}

// NewOutboxWorker constructs the worker.
func NewOutboxWorker(outbox port.OutboxRepository, publisher port.BrokerPublisher, interval time.Duration) *OutboxWorker {
	if interval <= 0 {
		interval = 5 * time.Second
	}

	return &OutboxWorker{outbox: outbox, publisher: publisher, interval: interval, batchSize: 100}
}

// Start runs the worker until ctx is cancelled.
func (w *OutboxWorker) Start(ctx context.Context) {
	ticker := time.NewTicker(w.interval)

	defer ticker.Stop()

	log.Printf("[Coaching] Outbox worker started (interval=%v)", w.interval)

	for {
		select {
		case <-ctx.Done():
			log.Println("[Coaching] Outbox worker stopping (ctx cancelled)")

			return

		case <-ticker.C:
			if err := w.tick(ctx); err != nil {
				log.Printf("[Coaching] outbox tick error: %v", err)
			}
		}
	}
}

func (w *OutboxWorker) tick(ctx context.Context) error {
	var records []*port.OutboxRecord

	err := w.outbox.ExecuteInLock(ctx, LockID, func(txCtx context.Context) error {
		var e error

		records, e = w.outbox.FetchUnpublished(txCtx, w.batchSize)

		return e
	})

	if err != nil {
		return fmt.Errorf("fetch unpublished: %w", err)
	}

	if len(records) == 0 {
		return nil
	}

	if w.publisher != nil {
		if err := w.publisher.PublishBatch(ctx, records); err != nil {
			return fmt.Errorf("publish batch: %w", err)
		}
	}

	ids := make([]string, 0, len(records))

	for _, r := range records {
		ids = append(ids, r.ID)
	}

	return w.outbox.ExecuteInLock(ctx, LockID, func(txCtx context.Context) error {
		return w.outbox.MarkPublished(txCtx, ids)
	})
}
