package event_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/auth/application/port"
	domainEvent "github.com/viethung213/gym-companion/internal/auth/domain/event"
	authEvent "github.com/viethung213/gym-companion/internal/auth/infrastructure/event"
)

type mockOutboxRepository struct {
	savedEvents []savedEventRecord
}

type savedEventRecord struct {
	EventID      string
	EventType    string
	Payload      []byte
	PartitionKey string
}

func (m *mockOutboxRepository) SaveEvent(ctx context.Context, eventID string, eventType string, payload []byte, partitionKey string) error {
	m.savedEvents = append(m.savedEvents, savedEventRecord{
		EventID:      eventID,
		EventType:    eventType,
		Payload:      payload,
		PartitionKey: partitionKey,
	})
	return nil
}

func (m *mockOutboxRepository) FetchUnpublished(ctx context.Context, limit int) ([]*port.OutboxRecord, error) {
	return nil, nil
}

func (m *mockOutboxRepository) ClaimBatch(ctx context.Context, limit int, lockDuration time.Duration) ([]*port.OutboxRecord, error) {
	return nil, nil
}

func (m *mockOutboxRepository) MarkPublished(ctx context.Context, ids []string) error {
	return nil
}

func (m *mockOutboxRepository) ProcessBatch(ctx context.Context, limit int, publishFn func(ctx context.Context, records []*port.OutboxRecord) error) error {
	return nil
}

func TestOutboxWriter_Write_OTPSent_Email(t *testing.T) {
	repo := &mockOutboxRepository{}
	writer := authEvent.NewOutboxWriter(repo)

	now := time.Now().UTC()
	otpEv := domainEvent.OTPSentEvent{
		Identifier:       "user@example.com",
		Code:             "654321",
		ExpiresInSeconds: 180,
		SentAt:           now,
	}

	err := writer.Write(context.Background(), otpEv)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(repo.savedEvents) != 1 {
		t.Fatalf("expected 1 saved event, got %d", len(repo.savedEvents))
	}

	saved := repo.savedEvents[0]
	expectedEventType := "contracts.generic.notification.v1.event.HighPriorityNotificationRequested"
	if saved.EventType != expectedEventType {
		t.Errorf("got event type %s, want %s", saved.EventType, expectedEventType)
	}
	if saved.PartitionKey != "user@example.com" {
		t.Errorf("got partition key %s, want user@example.com", saved.PartitionKey)
	}

	var cloudEvent struct {
		ID   string          `json:"id"`
		Type string          `json:"type"`
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(saved.Payload, &cloudEvent); err != nil {
		t.Fatalf("failed to unmarshal cloudevent: %v", err)
	}

	var payload struct {
		Channels []string          `json:"channels"`
		Data     map[string]string `json:"data"`
		Title    string            `json:"title"`
	}
	if err := json.Unmarshal(cloudEvent.Data, &payload); err != nil {
		t.Fatalf("failed to unmarshal event data: %v", err)
	}

	if len(payload.Channels) != 1 || payload.Channels[0] != "EMAIL" {
		t.Errorf("expected channel EMAIL, got %v", payload.Channels)
	}
	if payload.Data["code"] != "654321" {
		t.Errorf("expected code 654321, got %s", payload.Data["code"])
	}
	if payload.Data["email"] != "user@example.com" {
		t.Errorf("expected email user@example.com, got %s", payload.Data["email"])
	}
}

func TestOutboxWriter_Write_OTPSent_SMS(t *testing.T) {
	repo := &mockOutboxRepository{}
	writer := authEvent.NewOutboxWriter(repo)

	now := time.Now().UTC()
	otpEv := domainEvent.OTPSentEvent{
		Identifier:       "+84912345678",
		Code:             "112233",
		ExpiresInSeconds: 120,
		SentAt:           now,
	}

	err := writer.Write(context.Background(), otpEv)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(repo.savedEvents) != 1 {
		t.Fatalf("expected 1 saved event, got %d", len(repo.savedEvents))
	}

	saved := repo.savedEvents[0]
	var cloudEvent struct {
		Data json.RawMessage `json:"data"`
	}
	_ = json.Unmarshal(saved.Payload, &cloudEvent)

	var payload struct {
		Channels []string          `json:"channels"`
		Data     map[string]string `json:"data"`
	}
	_ = json.Unmarshal(cloudEvent.Data, &payload)

	if len(payload.Channels) != 1 || payload.Channels[0] != "SMS" {
		t.Errorf("expected channel SMS, got %v", payload.Channels)
	}
	if payload.Data["phone"] != "+84912345678" {
		t.Errorf("expected phone +84912345678, got %s", payload.Data["phone"])
	}
}
