package event_test

import (
	"encoding/json"
	"testing"
	"time"

	domainEvent "github.com/viethung213/gym-companion/internal/auth/domain/event"
	infraEvent "github.com/viethung213/gym-companion/internal/auth/infrastructure/event"
)

func TestPackCloudEvent_UserRegistered(t *testing.T) {
	now := time.Now().UTC()
	ev := domainEvent.UserRegisteredEvent{
		UserID:       "usr-123",
		FullName:     "Tran Van B",
		Identifier:   "b@example.com",
		IdentityType: "EMAIL",
		RegisteredAt: now,
	}

	rec, err := infraEvent.PackCloudEvent(ev)
	if err != nil {
		t.Fatalf("PackCloudEvent failed: %v", err)
	}

	if rec.EventID == "" {
		t.Errorf("expected non-empty eventID")
	}
	if rec.EventType != "contracts.generic.auth.v1.userRegistered" {
		t.Errorf("got eventType %s, want userRegistered", rec.EventType)
	}
	if rec.AggregateType != "User" || rec.AggregateID != "usr-123" || rec.PartitionKey != "usr-123" {
		t.Errorf("unexpected routing metadata: %s, %s, %s", rec.AggregateType, rec.AggregateID, rec.PartitionKey)
	}
	if !rec.OccurredAt.Equal(now) {
		t.Errorf("got occurredAt %v, want %v", rec.OccurredAt, now)
	}

	var envelope infraEvent.CloudEventEnvelope
	if err := json.Unmarshal(rec.Payload, &envelope); err != nil {
		t.Fatalf("unmarshal payload failed: %v", err)
	}
	if envelope.SpecVersion != "1.0" {
		t.Errorf("got specversion %s, want 1.0", envelope.SpecVersion)
	}
	if envelope.Source != "services/auth-service" {
		t.Errorf("got source %s, want services/auth-service", envelope.Source)
	}
}
