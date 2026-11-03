package event

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	notificationv1event "github.com/viethung213/gym-companion/internal/gen/go/contracts/generic/notification/v1/event"
	domainEvent "github.com/viethung213/gym-companion/internal/notification/domain/event"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// CloudEventEnvelope represents the standard CloudEvents 1.0 JSON format envelope.
type CloudEventEnvelope struct {
	SpecVersion     string          `json:"specversion"`
	ID              string          `json:"id"`
	Source          string          `json:"source"`
	Type            string          `json:"type"`
	Time            string          `json:"time"`
	DataContentType string          `json:"datacontenttype"`
	Data            json.RawMessage `json:"data"`
}

// OutboxRecordData holds the packaged event metadata and payload ready for Outbox persistence.
type OutboxRecordData struct {
	EventID       string
	EventType     string
	AggregateType string
	AggregateID   string
	PartitionKey  string
	OccurredAt    time.Time
	Payload       []byte
}

// PackCloudEvent packs a notification domain event into CloudEvents 1.0 JSON format.
func PackCloudEvent(ev any) (*OutboxRecordData, error) {
	if ev == nil {
		return nil, errors.New("nil domain event")
	}

	var notifEv *domainEvent.NotificationSentEvent
	switch e := ev.(type) {
	case domainEvent.NotificationSentEvent:
		notifEv = &e
	case *domainEvent.NotificationSentEvent:
		notifEv = e
	default:
		return nil, fmt.Errorf("unsupported notification event type: %T", ev)
	}

	occurredAt := notifEv.SentAt
	if occurredAt.IsZero() {
		occurredAt = time.Now().UTC()
	}

	eventType := "contracts.generic.notification.v1.event.NotificationSent"
	payloadProto := &notificationv1event.NotificationSent{
		NotificationId: notifEv.NotificationID,
		UserId:         notifEv.UserID,
		Channel:        notifEv.Channel,
		Title:          notifEv.Title,
		Body:           notifEv.Body,
		SentAt:         timestamppb.New(occurredAt),
	}

	payloadBytes, err := protojson.Marshal(payloadProto)
	if err != nil {
		return nil, fmt.Errorf("marshal NotificationSent proto: %w", err)
	}

	eventID := uuid.New().String()
	cloudEvent := CloudEventEnvelope{
		SpecVersion:     "1.0",
		ID:              eventID,
		Source:          "services/notification-service",
		Type:            eventType,
		Time:            occurredAt.Format(time.RFC3339),
		DataContentType: "application/json",
		Data:            json.RawMessage(payloadBytes),
	}

	cloudEventBytes, err := json.Marshal(cloudEvent)
	if err != nil {
		return nil, fmt.Errorf("marshal cloudevent envelope: %w", err)
	}

	return &OutboxRecordData{
		EventID:       eventID,
		EventType:     eventType,
		AggregateType: "Notification",
		AggregateID:   notifEv.UserID,
		PartitionKey:  notifEv.UserID,
		OccurredAt:    occurredAt,
		Payload:       cloudEventBytes,
	}, nil
}
