package event_test

import (
	"encoding/json"
	"testing"

	domainEvent "github.com/viethung213/gym-companion/internal/notification/domain/event"
	infraEvent "github.com/viethung213/gym-companion/internal/notification/infrastructure/event"
)

func TestPackCloudEvent_NotificationSent(t *testing.T) {
	ev := domainEvent.NewNotificationSentEvent(
		"notif-1",
		"user-1",
		"PUSH",
		"Hello",
		"World",
		map[string]string{"key": "value"},
	)

	rec, err := infraEvent.PackCloudEvent(ev)
	if err != nil {
		t.Fatalf("PackCloudEvent failed: %v", err)
	}

	if rec.EventID == "" {
		t.Errorf("expected non-empty eventID")
	}
	if rec.EventType != "contracts.generic.notification.v1.event.NotificationSent" {
		t.Errorf("got eventType %s, want NotificationSent", rec.EventType)
	}
	if rec.AggregateType != "Notification" || rec.AggregateID != "user-1" || rec.PartitionKey != "user-1" {
		t.Errorf("unexpected routing metadata: %s, %s, %s", rec.AggregateType, rec.AggregateID, rec.PartitionKey)
	}
	if rec.OccurredAt.IsZero() {
		t.Errorf("occurredAt is zero")
	}

	var envelope infraEvent.CloudEventEnvelope
	if err := json.Unmarshal(rec.Payload, &envelope); err != nil {
		t.Fatalf("unmarshal payload failed: %v", err)
	}
	if envelope.SpecVersion != "1.0" {
		t.Errorf("got specversion %s, want 1.0", envelope.SpecVersion)
	}
	if envelope.Source != "services/notification-service" {
		t.Errorf("got source %s, want services/notification-service", envelope.Source)
	}
}
