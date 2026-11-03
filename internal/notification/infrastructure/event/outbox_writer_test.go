package event_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/notification/application/port"
	domainEvent "github.com/viethung213/gym-companion/internal/notification/domain/event"
	infraEvent "github.com/viethung213/gym-companion/internal/notification/infrastructure/event"
)

type mockOutboxRepository struct {
	records []*port.OutboxRecord
	fail    bool
}

func (m *mockOutboxRepository) Save(ctx context.Context, record *port.OutboxRecord) error {
	if m.fail {
		return context.DeadlineExceeded
	}
	m.records = append(m.records, record)
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

func TestOutboxWriter_RecordEvents(t *testing.T) {
	repo := &mockOutboxRepository{}
	writer := infraEvent.NewOutboxWriter(repo)
	ctx := context.Background()

	ev := domainEvent.NewNotificationSentEvent(
		"notif-100",
		"user-100",
		"PUSH",
		"Workout Reminder",
		"Time to train!",
		map[string]string{"workout_id": "w-1"},
	)

	err := writer.RecordEvents(ctx, []any{ev})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if got, want := len(repo.records), 1; got != want {
		t.Fatalf("records count got %d, want %d", got, want)
	}

	rec := repo.records[0]
	if got, want := rec.PartitionKey, "user-100"; got != want {
		t.Errorf("partitionKey got %s, want %s", got, want)
	}
	if got, want := rec.EventType, "contracts.generic.notification.v1.event.NotificationSent"; got != want {
		t.Errorf("eventType got %s, want %s", got, want)
	}

	var envelope map[string]interface{}
	if unmarshalErr := json.Unmarshal(rec.Payload, &envelope); unmarshalErr != nil {
		t.Fatalf("unmarshal cloudEvent payload: %v", unmarshalErr)
	}
	if got, want := envelope["specversion"], "1.0"; got != want {
		t.Errorf("specversion got %v, want %v", got, want)
	}
	if got, want := envelope["type"], rec.EventType; got != want {
		t.Errorf("envelope type got %v, want %v", got, want)
	}
}
