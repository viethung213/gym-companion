package worker_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/workout_execution/application/port"
	"github.com/viethung213/gym-companion/internal/workout_execution/infrastructure/worker"
)

type mockOutboxRepo struct {
	lockErr           error
	fetchErr          error
	markErr           error
	unpublishedEvents []*port.OutboxRecord
	markedIDs         []string
}

func (m *mockOutboxRepo) Save(ctx context.Context, record *port.OutboxRecord) error {
	return nil
}

func (m *mockOutboxRepo) FetchUnpublished(ctx context.Context, limit int) ([]*port.OutboxRecord, error) {
	if m.fetchErr != nil {
		return nil, m.fetchErr
	}
	return m.unpublishedEvents, nil
}

func (m *mockOutboxRepo) MarkPublished(ctx context.Context, ids []string) error {
	m.markedIDs = ids
	return m.markErr
}

func (m *mockOutboxRepo) ExecuteInLock(ctx context.Context, lockKey int64, fn func(ctx context.Context) error) error {
	if m.lockErr != nil {
		return m.lockErr
	}
	return fn(ctx)
}

type mockPublisher struct {
	publishErr error
	published  []*port.OutboxRecord
}

func (m *mockPublisher) PublishBatch(ctx context.Context, events []*port.OutboxRecord) error {
	if m.publishErr != nil {
		return m.publishErr
	}
	m.published = events
	return nil
}

func TestOutboxWorker(t *testing.T) {
	t.Run("NewOutboxWorker constructor defaults and custom interval", func(t *testing.T) {
		wDefault := worker.NewOutboxWorker(&mockOutboxRepo{}, nil, 0)
		if wDefault == nil {
			t.Fatal("got nil worker, want non-nil")
		}

		wCustom := worker.NewOutboxWorker(&mockOutboxRepo{}, &mockPublisher{}, 100*time.Millisecond)
		if wCustom == nil {
			t.Fatal("got nil worker, want non-nil")
		}
	})

	t.Run("Start worker context cancellation", func(t *testing.T) {
		w := worker.NewOutboxWorker(&mockOutboxRepo{}, nil, 10*time.Millisecond)
		ctx, cancel := context.WithCancel(context.Background())

		done := make(chan struct{})
		go func() {
			w.Start(ctx)
			close(done)
		}()

		// Wait briefly then cancel
		time.Sleep(25 * time.Millisecond)
		cancel()

		select {
		case <-done:
			// Success
		case <-time.After(1 * time.Second):
			t.Fatal("worker did not stop on context cancellation")
		}
	})

	t.Run("processOutbox lock error", func(t *testing.T) {
		repo := &mockOutboxRepo{lockErr: errors.New("lock failed")}
		w := worker.NewOutboxWorker(repo, nil, 10*time.Millisecond)

		ctx, cancel := context.WithCancel(context.Background())
		go func() {
			time.Sleep(15 * time.Millisecond)
			cancel()
		}()
		w.Start(ctx)
	})

	t.Run("processOutbox empty unpublished events", func(t *testing.T) {
		repo := &mockOutboxRepo{unpublishedEvents: []*port.OutboxRecord{}}
		w := worker.NewOutboxWorker(repo, nil, 10*time.Millisecond)

		ctx, cancel := context.WithCancel(context.Background())
		go func() {
			time.Sleep(15 * time.Millisecond)
			cancel()
		}()
		w.Start(ctx)
	})

	t.Run("processOutbox publish batch failure", func(t *testing.T) {
		events := []*port.OutboxRecord{
			{ID: "event-1", AggregateType: "WorkoutSession", Payload: []byte("{}")},
		}
		repo := &mockOutboxRepo{unpublishedEvents: events}
		pub := &mockPublisher{publishErr: errors.New("kafka error")}
		w := worker.NewOutboxWorker(repo, pub, 10*time.Millisecond)

		ctx, cancel := context.WithCancel(context.Background())
		go func() {
			time.Sleep(15 * time.Millisecond)
			cancel()
		}()
		w.Start(ctx)
	})

	t.Run("processOutbox mark published failure", func(t *testing.T) {
		events := []*port.OutboxRecord{
			{ID: "event-1", AggregateType: "WorkoutSession", Payload: []byte("{}")},
		}
		repo := &mockOutboxRepo{unpublishedEvents: events, markErr: errors.New("mark err")}
		pub := &mockPublisher{}
		w := worker.NewOutboxWorker(repo, pub, 10*time.Millisecond)

		ctx, cancel := context.WithCancel(context.Background())
		go func() {
			time.Sleep(15 * time.Millisecond)
			cancel()
		}()
		w.Start(ctx)
	})

	t.Run("processOutbox full success", func(t *testing.T) {
		events := []*port.OutboxRecord{
			{ID: "event-1", AggregateType: "WorkoutSession", Payload: []byte("{}")},
		}
		repo := &mockOutboxRepo{unpublishedEvents: events}
		pub := &mockPublisher{}
		w := worker.NewOutboxWorker(repo, pub, 10*time.Millisecond)

		ctx, cancel := context.WithCancel(context.Background())
		go func() {
			time.Sleep(20 * time.Millisecond)
			cancel()
		}()
		w.Start(ctx)

		if len(pub.published) == 0 {
			t.Errorf("got published count = 0, want > 0")
		}
	})
}
