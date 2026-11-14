package worker

import (
	"context"
	"fmt"
	"log/slog"
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

	slog.InfoContext(ctx, "Starting Exercise Outbox background worker", slog.Duration("polling_interval", w.interval))

	for {
		select {
		case <-ctx.Done():
			slog.InfoContext(ctx, "Stopping Exercise Outbox background worker due to context cancellation")
			return
		case <-ticker.C:
			if err := w.processOutbox(ctx); err != nil {
				slog.ErrorContext(ctx, "Exercise Outbox worker processing error", slog.String("error", err.Error()))
			}
		}
	}
}

func (w *OutboxWorker) processOutbox(ctx context.Context) error {
	return w.outboxRepo.ProcessBatch(ctx, 500, func(publishCtx context.Context, events []*port.OutboxRecord) error {
		slog.DebugContext(publishCtx, "Exercise Outbox worker found unpublished events", slog.Int("count", len(events)))
		if err := w.publisher.PublishBatch(publishCtx, events); err != nil {
			return fmt.Errorf("publish batch failed: %w", err)
		}
		slog.DebugContext(publishCtx, "Exercise Outbox worker successfully published batch", slog.Int("count", len(events)))
		return nil
	})
}
