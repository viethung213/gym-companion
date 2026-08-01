//go:build unit

package worker_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/viethung213/gym-companion/internal/profile/application/port"
	"github.com/viethung213/gym-companion/internal/profile/infrastructure/worker"
)

type mockOutboxRepoForWorker struct {
	unpublished []*port.OutboxRecord
	published   []string
	failFetch   bool
	failMark    bool
}

func (m *mockOutboxRepoForWorker) Save(ctx context.Context, record *port.OutboxRecord) error {
	m.unpublished = append(m.unpublished, record)
	return nil
}

func (m *mockOutboxRepoForWorker) FetchUnpublished(ctx context.Context, limit int) ([]*port.OutboxRecord, error) {
	if m.failFetch {
		return nil, errors.New("db error fetch")
	}
	return m.unpublished, nil
}

func (m *mockOutboxRepoForWorker) MarkAsPublished(ctx context.Context, ids []string) error {
	if m.failMark {
		return errors.New("db error mark")
	}
	m.published = append(m.published, ids...)
	m.unpublished = nil
	return nil
}

func (m *mockOutboxRepoForWorker) ClaimBatch(ctx context.Context, limit int, _ time.Duration) ([]*port.OutboxRecord, error) {
	return m.FetchUnpublished(ctx, limit)
}

func (m *mockOutboxRepoForWorker) ProcessBatch(
	ctx context.Context,
	limit int,
	publishFn func(ctx context.Context, records []*port.OutboxRecord) error,
) error {
	unpub, err := m.FetchUnpublished(ctx, limit)
	if err != nil {
		return err
	}
	if len(unpub) == 0 {
		return nil
	}
	if err := publishFn(ctx, unpub); err != nil {
		return err
	}
	ids := make([]string, len(unpub))
	for i, r := range unpub {
		ids[i] = r.ID
	}
	return m.MarkAsPublished(ctx, ids)
}

type mockBrokerPublisher struct {
	publishedRecords []*port.OutboxRecord
	failPub          bool
}

func (m *mockBrokerPublisher) PublishBatch(ctx context.Context, records []*port.OutboxRecord) error {
	if m.failPub {
		return errors.New("kafka error pub")
	}
	m.publishedRecords = append(m.publishedRecords, records...)
	return nil
}

func TestOutboxWorker_ProcessUnpublished(t *testing.T) {
	outboxRepo := &mockOutboxRepoForWorker{
		unpublished: []*port.OutboxRecord{
			{
				ID:           "out-1",
				EventID:      "evt-1",
				EventType:    "contracts.supporting.profile.v1.event.ProfileCompleted",
				Payload:      []byte(`{"user_id":"u-1"}`),
				PartitionKey: "u-1",
			},
		},
	}
	publisher := &mockBrokerPublisher{}

	w := worker.NewOutboxWorker(outboxRepo, publisher, 50*time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Millisecond)
	defer cancel()

	w.Start(ctx)

	assert.NotEmpty(t, publisher.publishedRecords)
	assert.Contains(t, outboxRepo.published, "out-1")
}

func TestOutboxWorker_Errors(t *testing.T) {
	outboxRepo := &mockOutboxRepoForWorker{
		unpublished: []*port.OutboxRecord{
			{ID: "out-err-1", EventID: "evt-err-1", PartitionKey: "u-err"},
		},
	}
	publisher := &mockBrokerPublisher{}

	w := worker.NewOutboxWorker(outboxRepo, publisher, 10*time.Millisecond)

	// 1. Fail Fetch
	outboxRepo.failFetch = true
	ctx1, cancel1 := context.WithTimeout(context.Background(), 25*time.Millisecond)
	defer cancel1()
	w.Start(ctx1)
	outboxRepo.failFetch = false

	// 2. Fail Publish
	publisher.failPub = true
	ctx2, cancel2 := context.WithTimeout(context.Background(), 25*time.Millisecond)
	defer cancel2()
	w.Start(ctx2)
	publisher.failPub = false

	// 3. Fail Mark
	outboxRepo.failMark = true
	ctx3, cancel3 := context.WithTimeout(context.Background(), 25*time.Millisecond)
	defer cancel3()
	w.Start(ctx3)
	outboxRepo.failMark = false
}
