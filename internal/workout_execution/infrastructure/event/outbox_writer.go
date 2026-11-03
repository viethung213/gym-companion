package event

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/viethung213/gym-companion/internal/workout_execution/application/port"
)

// OutboxWriter implements port.EventRecorder.
type OutboxWriter struct {
	outboxRepo port.OutboxRepository
}

var _ port.EventRecorder = (*OutboxWriter)(nil)

// NewOutboxWriter constructs OutboxWriter.
func NewOutboxWriter(outboxRepo port.OutboxRepository) *OutboxWriter {
	return &OutboxWriter{outboxRepo: outboxRepo}
}

// RecordEvents serializes domain events to CloudEvents and writes to outbox.
func (w *OutboxWriter) RecordEvents(ctx context.Context, events []any) error {
	for _, ev := range events {
		if err := w.recordSingleEvent(ctx, ev); err != nil {
			return err
		}
	}
	return nil
}

// WriteEvents delegates to RecordEvents for backward compatibility.
func (w *OutboxWriter) WriteEvents(ctx context.Context, aggregateType, aggregateID string, events []interface{}) error {
	return w.RecordEvents(ctx, events)
}

func (w *OutboxWriter) recordSingleEvent(ctx context.Context, ev any) error {
	if ev == nil {
		return nil
	}

	rec, err := PackCloudEvent(ev)
	if err != nil {
		return fmt.Errorf("pack cloudevent: %w", err)
	}

	record := &port.OutboxRecord{
		ID:            uuid.NewString(),
		EventID:       rec.EventID,
		AggregateType: rec.AggregateType,
		AggregateID:   rec.AggregateID,
		EventType:     rec.EventType,
		Payload:       rec.Payload,
		PartitionKey:  rec.PartitionKey,
		Published:     false,
		CreatedAt:     rec.OccurredAt,
	}

	if err := w.outboxRepo.Save(ctx, record); err != nil {
		return fmt.Errorf("save outbox record: %w", err)
	}

	return nil
}
