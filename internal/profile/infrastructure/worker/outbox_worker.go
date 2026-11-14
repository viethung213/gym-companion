package worker

import (
	"context"
	"fmt"
	"log/slog"
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

	slog.InfoContext(ctx, "Starting Profile Outbox background worker", slog.Duration("interval", w.interval))

	for {
		select {
		case <-ctx.Done():
			slog.InfoContext(ctx, "Stopping Profile Outbox background worker due to context cancellation")
			return
		case <-ticker.C:
			if err := w.processOutbox(ctx); err != nil {
				slog.ErrorContext(ctx, "Profile outbox worker processing error", slog.String("error", err.Error()))
			}
		}
	}
}

func (w *OutboxWorker) processOutbox(ctx context.Context) error {
	return w.outboxRepo.ProcessBatch(ctx, 100, func(publishCtx context.Context, events []*port.OutboxRecord) error {
		if err := w.publisher.PublishBatch(publishCtx, events); err != nil {
			return fmt.Errorf("publish outbox batch to kafka: %w", err)
		}
		slog.DebugContext(publishCtx, "Profile Outbox worker successfully published batch", slog.Int("count", len(events)))
		return nil
	})
}
