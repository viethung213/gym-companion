package worker

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/viethung213/gym-companion/internal/coaching/application/port"
)

const advisoryLockID int64 = 77553311

type OutboxWorker struct {
	repository port.OutboxRepository
	publisher  port.EventPublisher
	interval   time.Duration
}

func NewOutboxWorker(
	repository port.OutboxRepository,
	publisher port.EventPublisher,
	interval time.Duration,
) *OutboxWorker {
	return &OutboxWorker{repository: repository, publisher: publisher, interval: interval}
}

func (w *OutboxWorker) Start(ctx context.Context) {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()
	log.Printf("Starting Coaching Outbox worker (polling interval: %v)", w.interval)
	for {
		select {
		case <-ctx.Done():
			log.Println("Stopping Coaching Outbox worker")
			return
		case <-ticker.C:
			if err := w.process(ctx); err != nil {
				log.Printf("Coaching Outbox worker error: %v", err)
			}
		}
	}
}

func (w *OutboxWorker) process(ctx context.Context) error {
	var records []*port.OutboxRecord
	if err := w.repository.ExecuteInLock(
		ctx,
		advisoryLockID,
		func(lockContext context.Context) error {
			var err error
			records, err = w.repository.FetchUnpublished(lockContext, 500)
			return err
		},
	); err != nil {
		return err
	}
	if len(records) == 0 {
		return nil
	}
	if err := w.publisher.PublishBatch(ctx, records); err != nil {
		return fmt.Errorf("publish outbox batch: %w", err)
	}
	ids := make([]string, len(records))
	for index, record := range records {
		ids[index] = record.ID
	}
	if err := w.repository.ExecuteInLock(
		ctx,
		advisoryLockID,
		func(lockContext context.Context) error {
			return w.repository.MarkPublished(lockContext, ids)
		},
	); err != nil {
		return fmt.Errorf("mark outbox batch published: %w", err)
	}
	log.Printf("Coaching Outbox worker published %d events", len(records))
	return nil
}
