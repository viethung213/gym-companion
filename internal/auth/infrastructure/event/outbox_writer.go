// Package event provides CloudEvent serialization and outbox recording adapters
// for all domain events produced by the auth bounded context.
package event

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/viethung213/gym-companion/internal/auth/application/port"
	domainEvent "github.com/viethung213/gym-companion/internal/auth/domain/event"
)

// OutboxWriter implements port.EventRecorder by serializing domain events
// into CloudEvent envelopes and persisting them to the outbox table.
type OutboxWriter struct {
	outboxRepo port.OutboxRepository
}

// Compile-time interface check
var _ port.EventRecorder = (*OutboxWriter)(nil)

// NewOutboxWriter creates a new OutboxWriter instance.
func NewOutboxWriter(outboxRepo port.OutboxRepository) *OutboxWriter {
	return &OutboxWriter{outboxRepo: outboxRepo}
}

// RecordEvents records a slice of domain events into the outbox repository.
func (p *OutboxWriter) RecordEvents(ctx context.Context, events []any) error {
	for _, ev := range events {
		dev, ok := ev.(domainEvent.DomainEvent)
		if !ok {
			return fmt.Errorf("unsupported domain event: %T", ev)
		}
		if err := p.Write(ctx, dev); err != nil {
			return err
		}
	}
	return nil
}

// Write serializes and stores the domain event as a CloudEvent into the outbox repository.
func (p *OutboxWriter) Write(ctx context.Context, ev domainEvent.DomainEvent) error {
	rec, err := PackCloudEvent(ev)
	if err != nil {
		return err
	}

	record := &port.OutboxRecord{
		ID:            uuid.New().String(),
		EventID:       rec.EventID,
		AggregateType: rec.AggregateType,
		AggregateID:   rec.AggregateID,
		EventType:     rec.EventType,
		Payload:       rec.Payload,
		PartitionKey:  rec.PartitionKey,
		Published:     false,
		CreatedAt:     rec.OccurredAt,
	}

	return p.outboxRepo.Save(ctx, record)
}
