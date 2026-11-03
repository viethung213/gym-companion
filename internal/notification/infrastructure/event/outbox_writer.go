package event

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/viethung213/gym-companion/internal/notification/application/port"
)

// OutboxWriter implements port.EventRecorder by serializing domain events
// into standardized CloudEvent 1.0 outbox records and persisting them in PostgreSQL.
type OutboxWriter struct {
	outboxRepo port.OutboxRepository
}

var _ port.EventRecorder = (*OutboxWriter)(nil)

// NewOutboxWriter creates a new OutboxWriter instance.
func NewOutboxWriter(outboxRepo port.OutboxRepository) *OutboxWriter {
	return &OutboxWriter{outboxRepo: outboxRepo}
}

// RecordEvents converts each event into a CloudEvent 1.0 outbox record and saves it in the database.
func (w *OutboxWriter) RecordEvents(ctx context.Context, events []any) error {
	for _, ev := range events {
		if err := w.recordSingleEvent(ctx, ev); err != nil {
			return err
		}
	}
	return nil
}

// PublishEvents delegates to RecordEvents for backward compatibility.
func (w *OutboxWriter) PublishEvents(ctx context.Context, events []any) error {
	return w.RecordEvents(ctx, events)
}

func (w *OutboxWriter) recordSingleEvent(ctx context.Context, ev any) error {
	rec, err := PackCloudEvent(ev)
	if err != nil {
		return err
	}

	outboxRec := &port.OutboxRecord{
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

	if err := w.outboxRepo.Save(ctx, outboxRec); err != nil {
		return fmt.Errorf("save notification outbox record: %w", err)
	}

	return nil
}
