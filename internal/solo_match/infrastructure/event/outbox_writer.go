package event

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/viethung213/gym-companion/internal/solo_match/application/port"
	domainEvent "github.com/viethung213/gym-companion/internal/solo_match/domain/event"
)

// OutboxWriter implements port.EventRecorder by transforming domain events
// into CloudEvents 1.0 JSON payloads and writing them to port.OutboxRepository.
type OutboxWriter struct {
	outboxRepo port.OutboxRepository
}

var _ port.EventRecorder = (*OutboxWriter)(nil)

// NewOutboxWriter creates a new OutboxWriter instance.
func NewOutboxWriter(outboxRepo port.OutboxRepository) *OutboxWriter {
	return &OutboxWriter{outboxRepo: outboxRepo}
}

// RecordEvents converts each event into a standardized CloudEvent outbox record and saves it.
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
	dev, ok := ev.(domainEvent.DomainEvent)
	if !ok {
		return fmt.Errorf("unsupported event type %T: must implement domainEvent.DomainEvent", ev)
	}

	eventID, eventType, payload, partitionKey, err := PackCloudEvent(dev)
	if err != nil {
		return fmt.Errorf("pack cloudevent for %s: %w", dev.EventName(), err)
	}

	aggregateType := "SoloMatch"
	var aggregateID string
	switch e := dev.(type) {
	case *domainEvent.SoloMatchStarted:
		aggregateID = e.MatchID
	case *domainEvent.SoloMatchCompleted:
		aggregateID = e.MatchID
	case *domainEvent.SoloMatchForfeited:
		aggregateID = e.MatchID
	case *domainEvent.SoloMatchCancelled:
		aggregateID = e.MatchID
	}
	if aggregateID == "" {
		aggregateID = partitionKey
	}

	createdAt := dev.OccurredAt()
	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	}

	record := &port.OutboxRecord{
		ID:            uuid.New().String(),
		EventID:       eventID,
		AggregateType: aggregateType,
		AggregateID:   aggregateID,
		EventType:     eventType,
		Payload:       payload,
		PartitionKey:  partitionKey,
		Published:     false,
		CreatedAt:     createdAt,
	}

	return w.outboxRepo.Save(ctx, record)
}
