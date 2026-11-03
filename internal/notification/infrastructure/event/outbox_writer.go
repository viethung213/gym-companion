package event

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	notificationv1event "github.com/viethung213/gym-companion/internal/gen/go/contracts/generic/notification/v1/event"
	"github.com/viethung213/gym-companion/internal/notification/application/port"
	domainEvent "github.com/viethung213/gym-companion/internal/notification/domain/event"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/timestamppb"
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
	switch e := ev.(type) {
	case domainEvent.NotificationSentEvent:
		return w.recordNotificationSent(ctx, &e)
	case *domainEvent.NotificationSentEvent:
		return w.recordNotificationSent(ctx, e)
	default:
		return fmt.Errorf("unsupported notification event type: %T", ev)
	}
}

func (w *OutboxWriter) recordNotificationSent(ctx context.Context, ev *domainEvent.NotificationSentEvent) error {
	now := ev.SentAt
	if now.IsZero() {
		now = time.Now().UTC()
	}
	eventType := "contracts.generic.notification.v1.event.NotificationSent"

	payloadProto := &notificationv1event.NotificationSent{
		NotificationId: ev.NotificationID,
		UserId:         ev.UserID,
		Channel:        ev.Channel,
		Title:          ev.Title,
		Body:           ev.Body,
		SentAt:         timestamppb.New(now),
	}

	payloadBytes, err := protojson.Marshal(payloadProto)
	if err != nil {
		return fmt.Errorf("marshal NotificationSent proto payload: %w", err)
	}

	return w.writeCloudEvent(ctx, "Notification", ev.UserID, eventType, ev.UserID, now, payloadBytes)
}

func (w *OutboxWriter) writeCloudEvent(
	ctx context.Context,
	aggregateType, aggregateID, eventType, partitionKey string,
	occurredAt time.Time,
	payloadBytes []byte,
) error {
	eventID := uuid.New().String()

	cloudEvent := map[string]interface{}{
		"specversion":     "1.0",
		"id":              eventID,
		"source":          "services/notification-service",
		"type":            eventType,
		"time":            occurredAt.Format(time.RFC3339),
		"datacontenttype": "application/json",
		"data":            json.RawMessage(payloadBytes),
	}

	cloudEventBytes, err := json.Marshal(cloudEvent)
	if err != nil {
		return fmt.Errorf("marshal cloudevent envelope: %w", err)
	}

	outboxRec := &port.OutboxRecord{
		ID:            uuid.New().String(),
		EventID:       eventID,
		AggregateType: aggregateType,
		AggregateID:   aggregateID,
		EventType:     eventType,
		Payload:       cloudEventBytes,
		PartitionKey:  partitionKey,
		Published:     false,
		CreatedAt:     occurredAt,
	}

	if err := w.outboxRepo.Save(ctx, outboxRec); err != nil {
		return fmt.Errorf("save notification outbox record: %w", err)
	}

	return nil
}
