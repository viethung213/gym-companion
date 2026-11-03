package event

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/viethung213/gym-companion/internal/social/application/port"
	domainEvent "github.com/viethung213/gym-companion/internal/social/domain/event"
)

type OutboxWriter struct {
	outboxRepo port.OutboxRepository
}

var _ port.EventRecorder = (*OutboxWriter)(nil)

func NewOutboxWriter(outboxRepo port.OutboxRepository) *OutboxWriter {
	return &OutboxWriter{outboxRepo: outboxRepo}
}

// RecordEvents saves domain events as CloudEvent outbox records in the database.
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

type UserFollowedEvent = domainEvent.UserFollowedEvent
type PostCreatedEvent = domainEvent.PostCreatedEvent
type PostReactedEvent = domainEvent.PostReactedEvent
type PostCommentedEvent = domainEvent.PostCommentedEvent

func (w *OutboxWriter) recordSingleEvent(ctx context.Context, ev any) error {
	records, err := PackNotifications(ev)
	if err != nil {
		return err
	}

	for i := range records {
		rec := &records[i]
		outboxRec := &port.OutboxRecord{
			ID:            uuid.New().String(),
			EventID:       rec.EventID,
			AggregateType: rec.AggregateType,
			AggregateID:   rec.AggregateID,
			EventType:     rec.EventType,
			Payload:       rec.Payload,
			PartitionKey:  rec.PartitionKey,
			CreatedAt:     rec.OccurredAt,
			Published:     false,
		}

		if err := w.outboxRepo.Save(ctx, outboxRec); err != nil {
			return fmt.Errorf("save outbox record: %w", err)
		}
	}

	return nil
}
