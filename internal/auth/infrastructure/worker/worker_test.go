//go:build unit

package worker

import (
	"context"
	"errors"
	"testing"

	"github.com/viethung213/gym-companion/internal/auth/application/port"
)

type mockOutboxRepository struct {
	events       []*port.OutboxRecord
	publishedMap map[string]bool
}

func (m *mockOutboxRepository) SaveEvent(ctx context.Context, eventID string, eventType string, payload []byte, partitionKey string) error {
	e := &port.OutboxRecord{
		ID:           eventID, // simplify mock using eventID as record ID
		EventID:      eventID,
		EventType:    eventType,
		Payload:      payload,
		PartitionKey: partitionKey,
	}
	m.events = append(m.events, e)
	return nil
}

func (m *mockOutboxRepository) FetchUnpublished(ctx context.Context, limit int) ([]*port.OutboxRecord, error) {
	var unpublished []*port.OutboxRecord
	for _, e := range m.events {
		if !m.publishedMap[e.ID] {
			unpublished = append(unpublished, e)
			if len(unpublished) >= limit {
				break
			}
		}
	}
	return unpublished, nil
}

func (m *mockOutboxRepository) MarkPublished(ctx context.Context, ids []string) error {
	for _, id := range ids {
		found := false
		for _, e := range m.events {
			if e.ID == id {
				m.publishedMap[id] = true
				found = true
				break
			}
		}
		if !found {
			return errors.New("not found: " + id)
		}
	}
	return nil
}

func (m *mockOutboxRepository) ExecuteInLock(ctx context.Context, lockID int64, fn func(ctx context.Context) error) error {
	return fn(ctx)
}

type mockEventPublisher struct {
	published []struct {
		topic string
		key   string
		value []byte
	}
}

func (p *mockEventPublisher) PublishBatch(ctx context.Context, records []*port.OutboxRecord) error {
	for _, r := range records {
		p.published = append(p.published, struct {
			topic string
			key   string
			value []byte
		}{topic: r.EventType, key: r.PartitionKey, value: r.Payload})
	}
	return nil
}

func TestOutboxWorker_ProcessOutbox(t *testing.T) {
	ctx := context.Background()

	// 1. Arrange
	record1 := &port.OutboxRecord{ID: "id-1", EventID: "ev-id-1", EventType: "user.registered", Payload: []byte(`{"userId":"user-1"}`), PartitionKey: "user-1"}
	record2 := &port.OutboxRecord{ID: "id-2", EventID: "ev-id-2", EventType: "user.registered", Payload: []byte(`{"userId":"user-2"}`), PartitionKey: "user-2"}

	repo := &mockOutboxRepository{
		events:       []*port.OutboxRecord{record1, record2},
		publishedMap: make(map[string]bool),
	}
	pub := &mockEventPublisher{}
	worker := NewOutboxWorker(repo, pub, 1)

	// 2. Act
	err := worker.processOutbox(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// 3. Assert
	if len(pub.published) != 2 {
		t.Errorf("expected 2 published messages, got %d", len(pub.published))
	}

	if pub.published[0].topic != "user.registered" || pub.published[0].key != "user-1" {
		t.Errorf("unexpected message 1 contents: %+v", pub.published[0])
	}

	// Verify they are now marked as published
	unpublished, err := repo.FetchUnpublished(ctx, 10)
	if err != nil {
		t.Fatalf("fetch unpublished error: %v", err)
	}
	if len(unpublished) != 0 {
		t.Errorf("expected 0 unpublished events left, got %d", len(unpublished))
	}
}
