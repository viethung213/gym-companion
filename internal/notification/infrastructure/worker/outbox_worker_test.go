package worker_test

import (
	"context"
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/notification/application/port"
	"github.com/viethung213/gym-companion/internal/notification/infrastructure/worker"
)

type mockOutboxRepo struct {
	records []*port.OutboxRecord
}

func (m *mockOutboxRepo) Save(ctx context.Context, record *port.OutboxRecord) error {
	m.records = append(m.records, record)
	return nil
}

func (m *mockOutboxRepo) FetchUnpublished(ctx context.Context, limit int) ([]*port.OutboxRecord, error) {
	return m.records, nil
}

func (m *mockOutboxRepo) ClaimBatch(ctx context.Context, limit int, lockDuration time.Duration) ([]*port.OutboxRecord, error) {
	return m.records, nil
}

func (m *mockOutboxRepo) MarkPublished(ctx context.Context, ids []string) error {
	return nil
}

func (m *mockOutboxRepo) ProcessBatch(ctx context.Context, limit int, publishFn func(ctx context.Context, records []*port.OutboxRecord) error) error {
	if len(m.records) > 0 {
		return publishFn(ctx, m.records)
	}
	return nil
}

type mockPublisher struct {
	publishedCount int
}

func (m *mockPublisher) PublishBatch(ctx context.Context, records []*port.OutboxRecord) error {
	m.publishedCount += len(records)
	return nil
}

func TestOutboxWorker(t *testing.T) {
	t.Parallel()

	outboxRepo := &mockOutboxRepo{
		records: []*port.OutboxRecord{
			{ID: "1", EventID: "e1", EventType: "test.event", Payload: []byte("{}"), PartitionKey: "u1"},
		},
	}
	publisher := &mockPublisher{}

	w := worker.NewOutboxWorker(outboxRepo, nil, publisher, 10*time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_ = w.Start(ctx)

	if publisher.publishedCount < 1 {
		t.Errorf("got publishedCount %d, want >= 1", publisher.publishedCount)
	}
}
