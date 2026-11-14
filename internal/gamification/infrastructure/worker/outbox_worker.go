package worker

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
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

	slog.InfoContext(ctx, "Starting Gamification Outbox worker", slog.Duration("polling_interval", w.interval))

	for {
		select {
		case <-ctx.Done():
			slog.InfoContext(ctx, "Stopping Gamification Outbox worker due to context cancellation")
			return
		case <-ticker.C:
			func() {
				defer func() {
					if r := recover(); r != nil {
						slog.ErrorContext(ctx, "PANIC RECOVERED in Gamification Outbox worker tick", slog.Any("panic", r))
					}
				}()
				if err := w.processOutbox(ctx); err != nil {
					slog.ErrorContext(ctx, "Gamification Outbox worker processing error", slog.String("error", err.Error()))
				}
			}()
		}
	}
}

func (w *OutboxWorker) processOutbox(ctx context.Context) error {
	if w.publisher == nil {
		return errors.New("outbox broker publisher is nil, cannot process outbox batch")
	}
	return w.outboxRepo.ProcessBatch(ctx, 100, func(publishCtx context.Context, events []*port.OutboxRecord) error {
		if err := w.publisher.PublishBatch(publishCtx, events); err != nil {
			return fmt.Errorf("publish batch failed: %w", err)
		}
		return nil
	})
}
