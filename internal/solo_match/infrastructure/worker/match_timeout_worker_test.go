package worker_test

import (
	"context"
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/solo_match/application/command"
	"github.com/viethung213/gym-companion/internal/solo_match/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/solo_match/infrastructure/worker"
)

type mockTimeoutRepo struct{}

func (m *mockTimeoutRepo) Save(_ context.Context, _ *aggregate.SoloMatch) error { return nil }
func (m *mockTimeoutRepo) FindByID(_ context.Context, _ string) (*aggregate.SoloMatch, error) {
	return nil, nil
}
func (m *mockTimeoutRepo) FindActiveByUserID(_ context.Context, _ string) (*aggregate.SoloMatch, error) {
	return nil, nil
}
func (m *mockTimeoutRepo) FindUnreadyExpiredMatches(_ context.Context, _ time.Time, _ int) ([]*aggregate.SoloMatch, error) {
	return nil, nil
}
func (m *mockTimeoutRepo) FindExpiredInProgressMatches(_ context.Context, _ time.Time, _ int) ([]*aggregate.SoloMatch, error) {
	return nil, nil
}
func (m *mockTimeoutRepo) ListByUserID(_ context.Context, _ string, _, _ int) ([]*aggregate.SoloMatch, error) {
	return nil, nil
}

type mockEventPub struct{}

func (m *mockEventPub) PublishEvents(_ context.Context, _ []any) error { return nil }

type mockTxManager struct{}

func (t *mockTxManager) WithTransaction(ctx context.Context, fn func(txCtx context.Context) error) error {
	return fn(ctx)
}

func TestMatchTimeoutWorker_Lifecycle(t *testing.T) {
	handler := command.NewHandleMatchTimeoutHandler(
		&mockTimeoutRepo{},
		&mockEventPub{},
		&mockTxManager{},
	)

	w := worker.NewMatchTimeoutWorker(handler, 10*time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})

	go func() {
		w.Start(ctx)
		close(done)
	}()

	time.Sleep(30 * time.Millisecond)
	cancel()

	select {
	case <-done:
		// Worker gracefully stopped
	case <-time.After(1 * time.Second):
		t.Fatal("worker did not stop in time after context cancellation")
	}
}
