package worker

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/social/application/port"
	"github.com/viethung213/gym-companion/internal/social/infrastructure/kafka"
)

type mockOutboxRepo struct {
	claimFn func(ctx context.Context, limit int, lockDuration time.Duration) ([]*port.OutboxRecord, error)
}

func (m *mockOutboxRepo) Save(ctx context.Context, record *port.OutboxRecord) error {
	return nil
}

func (m *mockOutboxRepo) FetchUnpublished(ctx context.Context, limit int) ([]*port.OutboxRecord, error) {
	return nil, nil
}

func (m *mockOutboxRepo) ClaimBatch(ctx context.Context, limit int, lockDuration time.Duration) ([]*port.OutboxRecord, error) {
	if m.claimFn != nil {
		return m.claimFn(ctx, limit, lockDuration)
	}
	return nil, nil
}

func (m *mockOutboxRepo) MarkAsPublished(ctx context.Context, ids []string) error {
	return nil
}

func TestOutboxWorker_StartCancellation(t *testing.T) {
	repo := &mockOutboxRepo{}
	pub := kafka.NewPublisher(nil)
	w := NewOutboxWorker(repo, pub, 10*time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	done := make(chan struct{})
	go func() {
		w.Start(ctx)
		close(done)
	}()

	select {
	case <-done:
		// success
	case <-time.After(500 * time.Millisecond):
		t.Fatalf("worker did not terminate upon context cancellation")
	}
}

func TestOutboxWorker_ProcessOutbox(t *testing.T) {
	t.Run("empty batch returns nil", func(t *testing.T) {
		repo := &mockOutboxRepo{
			claimFn: func(ctx context.Context, limit int, lockDuration time.Duration) ([]*port.OutboxRecord, error) {
				return []*port.OutboxRecord{}, nil
			},
		}
		w := NewOutboxWorker(repo, kafka.NewPublisher(nil), time.Second)
		err := w.processOutbox(context.Background())
		if err != nil {
			t.Fatalf("expected nil, got %v", err)
		}
	})

	t.Run("claim batch error propagated", func(t *testing.T) {
		expectedErr := errors.New("db lock failure")
		repo := &mockOutboxRepo{
			claimFn: func(ctx context.Context, limit int, lockDuration time.Duration) ([]*port.OutboxRecord, error) {
				return nil, expectedErr
			},
		}
		w := NewOutboxWorker(repo, kafka.NewPublisher(nil), time.Second)
		err := w.processOutbox(context.Background())
		if !errors.Is(err, expectedErr) {
			t.Fatalf("got err %v, want %v", err, expectedErr)
		}
	})
}
