package event

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/viethung213/gym-companion/internal/solo_match/application/port"
	domainEvent "github.com/viethung213/gym-companion/internal/solo_match/domain/event"
)

// OutboxWriter implements port.EventPublisher by transforming domain events
// into CloudEvents 1.0 JSON payloads and writing them to port.OutboxRepository.
type OutboxWriter struct {
	outboxRepo port.OutboxRepository
}

var _ port.EventPublisher = (*OutboxWriter)(nil)

// NewOutboxWriter creates a new OutboxWriter instance.
func NewOutboxWriter(outboxRepo port.OutboxRepository) *OutboxWriter {
	return &OutboxWriter{outboxRepo: outboxRepo}
}

// PublishEvents converts each event into a standardized CloudEvent outbox record and saves it.
func (w *OutboxWriter) PublishEvents(ctx context.Context, events []any) error {
	for _, ev := range events {
		if err := w.publishSingleEvent(ctx, ev); err != nil {
			return err
		}
	}
	return nil
}

func (w *OutboxWriter) publishSingleEvent(ctx context.Context, ev any) error {
	dev, ok := ev.(domainEvent.DomainEvent)
	if !ok {
		return nil
	}

	eventID, eventType, payload, partitionKey, err := PackCloudEvent(dev)
	if err != nil {
		return fmt.Errorf("pack cloudevent for %s: %w", dev.EventName(), err)
	}

	record := &port.OutboxRecord{
		ID:           uuid.New().String(),
		EventID:      eventID,
		EventType:    eventType,
		Payload:      payload,
		PartitionKey: partitionKey,
	}

	return w.outboxRepo.Save(ctx, record)
}
