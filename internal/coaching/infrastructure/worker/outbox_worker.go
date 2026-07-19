package worker

import (
	"context"
	"log"
	"time"

	"github.com/viethung213/gym-companion/internal/coaching/application/port"
)

const advisoryLockID int64 = 80809001 // Unique advisory lock ID for Coaching outbox worker

type OutboxWorker struct {
	repo      port.OutboxRepository
	publisher port.BrokerPublisher
	interval  time.Duration
}

func NewOutboxWorker(repo port.OutboxRepository, publisher port.BrokerPublisher, interval time.Duration) *OutboxWorker {
	return &OutboxWorker{
		repo:      repo,
		publisher: publisher,
		interval:  interval,
	}
}

func (w *OutboxWorker) Start(ctx context.Context) {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	log.Println("INFO: starting Coaching Outbox background worker")
	for {
		select {
		case <-ctx.Done():
			log.Println("INFO: stopping Coaching Outbox background worker")
			return
		case <-ticker.C:
			if err := w.processOutbox(ctx); err != nil {
				log.Printf("ERROR: process coaching outbox batch: %v", err)
			}
		}
	}
}

func (w *OutboxWorker) processOutbox(ctx context.Context) error {
	return w.repo.ExecuteInLock(ctx, advisoryLockID, func(txCtx context.Context) error {
		records, err := w.repo.FetchUnpublished(txCtx, 50)
		if err != nil {
			return err
		}
		if len(records) == 0 {
			return nil
		}

		if err := w.publisher.PublishBatch(txCtx, records); err != nil {
			return err
		}

		ids := make([]string, len(records))
		for i, r := range records {
			ids[i] = r.ID
		}

		return w.repo.MarkPublished(txCtx, ids)
	})
}
