package worker

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/viethung213/gym-companion/internal/solo_match/application/port"
)

// OutboxPublisherWorker periodically polls pending events from database outbox and pushes them to Kafka.
type OutboxPublisherWorker struct {
	outboxRepo port.OutboxRepository
	publisher  port.BrokerPublisher
	interval   time.Duration
	stopCh     chan struct{}
}

// NewOutboxPublisherWorker constructs a new OutboxPublisherWorker instance.
func NewOutboxPublisherWorker(outboxRepo port.OutboxRepository, publisher port.BrokerPublisher, interval time.Duration) *OutboxPublisherWorker {
	if interval <= 0 {
		interval = 2 * time.Second
	}
	return &OutboxPublisherWorker{
		outboxRepo: outboxRepo,
		publisher:  publisher,
		interval:   interval,
		stopCh:     make(chan struct{}),
	}
}

// Start begins the periodic polling sweep loop until the context or worker is stopped.
func (p *OutboxPublisherWorker) Start(ctx context.Context) {
	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()

	slog.InfoContext(ctx, "Starting SoloMatch Outbox Publisher", slog.Duration("polling_interval", p.interval))

	for {
		select {
		case <-ctx.Done():
			return
		case <-p.stopCh:
			return
		case <-ticker.C:
			func() {
				defer func() {
					if r := recover(); r != nil {
						slog.ErrorContext(ctx, "PANIC recovered in SoloMatch Outbox Publisher tick", slog.Any("panic", r))
					}
				}()
				if err := p.processPendingEvents(ctx); err != nil {
					slog.ErrorContext(ctx, "SoloMatch Outbox Publisher process error", slog.String("error", err.Error()))
				}
			}()
		}
	}
}

// Stop signals the worker to terminate gracefully.
func (p *OutboxPublisherWorker) Stop() {
	close(p.stopCh)
}

func (p *OutboxPublisherWorker) processPendingEvents(ctx context.Context) error {
	if p.publisher == nil {
		return nil
	}
	return p.outboxRepo.ProcessBatch(ctx, 50, func(publishCtx context.Context, records []*port.OutboxRecord) error {
		if err := p.publisher.PublishBatch(publishCtx, records); err != nil {
			return fmt.Errorf("publish solo_match outbox batch: %w", err)
		}
		return nil
	})
}
