package event

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/viethung213/gym-companion/internal/workout_execution/application/port"
)

// EventNameProvider interface to extract event name string.
type EventNameProvider interface {
	EventName() string
}

// OutboxWriter implements port.OutboxWriter.
type OutboxWriter struct {
	outboxRepo port.OutboxRepository
}

var _ port.OutboxWriter = (*OutboxWriter)(nil)

// NewOutboxWriter constructs OutboxWriter.
func NewOutboxWriter(outboxRepo port.OutboxRepository) *OutboxWriter {
	return &OutboxWriter{outboxRepo: outboxRepo}
}

// WriteEvents serializes domain events to CloudEvents and writes to outbox.
func (w *OutboxWriter) WriteEvents(ctx context.Context, aggregateType, aggregateID string, events []interface{}) error {
	for _, ev := range events {
		if err := w.writeSingleEvent(ctx, aggregateType, aggregateID, ev); err != nil {
			return err
		}
	}
	return nil
}

func (w *OutboxWriter) writeSingleEvent(ctx context.Context, aggregateType, aggregateID string, ev interface{}) error {
	eventType := "contracts.core.workout_execution.v1.event.UnknownEvent"
	if nameProvider, ok := ev.(EventNameProvider); ok {
		eventType = nameProvider.EventName()
	}

	payloadBytes, err := json.Marshal(ev)
	if err != nil {
		return fmt.Errorf("failed to marshal event payload: %w", err)
	}

	outboxID := uuid.NewString()
	eventID := uuid.NewString()
	now := time.Now().UTC().Format(time.RFC3339)

	cloudEvent := map[string]interface{}{
		"specversion":     "1.0",
		"id":              eventID,
		"source":          "services/workout-execution-service",
		"type":            eventType,
		"time":            now,
		"datacontenttype": "application/json",
		"data":            json.RawMessage(payloadBytes),
	}

	envelopeBytes, err := json.Marshal(cloudEvent)
	if err != nil {
		return fmt.Errorf("failed to marshal cloud event envelope: %w", err)
	}

	record := &port.OutboxRecord{
		ID:            outboxID,
		EventID:       eventID,
		AggregateType: aggregateType,
		AggregateID:   aggregateID,
		EventType:     eventType,
		Payload:       envelopeBytes,
		PartitionKey:  aggregateID,
		Published:     false,
		CreatedAt:     time.Now().UTC(),
	}

	if err := w.outboxRepo.Save(ctx, record); err != nil {
		return fmt.Errorf("failed to save outbox record: %w", err)
	}

	return nil
}
