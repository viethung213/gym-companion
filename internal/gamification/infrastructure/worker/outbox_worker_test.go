package worker_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/gamification/application/port"
	"github.com/viethung213/gym-companion/internal/gamification/infrastructure/worker"
)

type mockOutboxRepo struct {
	records      []*port.OutboxRecord
	publishedIDs []string
	failedIDs    []string
	lastErr      error
	processErr   error
}

func (m *mockOutboxRepo) Save(ctx context.Context, record *port.OutboxRecord) error {
	m.records = append(m.records, record)
	return nil
}

func (m *mockOutboxRepo) ClaimBatch(
	ctx context.Context,
	limit int,
	lockDuration time.Duration,
) ([]*port.OutboxRecord, error) {
	return m.records, nil
}

func (m *mockOutboxRepo) MarkAsPublished(ctx context.Context, ids []string) error {
	m.publishedIDs = append(m.publishedIDs, ids...)
	return nil
}

func (m *mockOutboxRepo) MarkAsFailed(ctx context.Context, ids []string, lastErr error) error {
	m.failedIDs = append(m.failedIDs, ids...)
	m.lastErr = lastErr
	return nil
}

func (m *mockOutboxRepo) ProcessBatch(
	ctx context.Context,
	limit int,
	publishFn func(ctx context.Context, records []*port.OutboxRecord) error,
) error {
	if m.processErr != nil {
		return m.processErr
	}
	if len(m.records) == 0 {
		return nil
	}
	ids := make([]string, len(m.records))
	for i, r := range m.records {
		ids[i] = r.ID
	}
	if err := publishFn(ctx, m.records); err != nil {
		_ = m.MarkAsFailed(ctx, ids, err)
		return err
	}
	return m.MarkAsPublished(ctx, ids)
}

type mockPublisher struct {
	published []*port.OutboxRecord
	err       error
}

func (p *mockPublisher) PublishBatch(ctx context.Context, events []*port.OutboxRecord) error {
	if p.err != nil {
		return p.err
	}
	p.published = append(p.published, events...)
	return nil
}

func TestOutboxWorker_Constructor(t *testing.T) {
	w := worker.NewOutboxWorker(nil, nil, 0)
	if w == nil {
		t.Fatal("NewOutboxWorker() = nil, want non-nil")
	}
}

func TestOutboxWorker_StartStopsOnContextCancel(t *testing.T) {
	repo := &mockOutboxRepo{}
	pub := &mockPublisher{}
	w := worker.NewOutboxWorker(repo, pub, 10*time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		w.Start(ctx)
		close(done)
	}()

	cancel()

	select {
	case <-done:
		// success: worker exited
	case <-time.After(1 * time.Second):
		t.Fatal("worker did not exit within timeout after context cancellation")
	}
}

func TestOutboxWorker_ProcessOutbox_Success(t *testing.T) {
	rec := &port.OutboxRecord{
		ID:        "rec-1",
		EventID:   "evt-1",
		EventType: "contracts.supporting.gamification.v1.xpEarned",
	}
	repo := &mockOutboxRepo{records: []*port.OutboxRecord{rec}}
	pub := &mockPublisher{}

	w := worker.NewOutboxWorker(repo, pub, 10*time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	done := make(chan struct{})
	go func() {
		w.Start(ctx)
		close(done)
	}()

	<-done

	if got := len(pub.published); got == 0 {
		t.Errorf("publisher received %d events, want at least 1", got)
	}
	if got := len(repo.publishedIDs); got == 0 {
		t.Errorf("repo marked %d records published, want at least 1", got)
	}
}

func TestOutboxWorker_ProcessOutbox_NilPublisher(t *testing.T) {
	repo := &mockOutboxRepo{}
	w := worker.NewOutboxWorker(repo, nil, 10*time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	w.Start(ctx)
	// should not panic
}

func TestOutboxWorker_ProcessOutbox_PublishError(t *testing.T) {
	rec := &port.OutboxRecord{ID: "rec-1"}
	repo := &mockOutboxRepo{records: []*port.OutboxRecord{rec}}
	wantErr := errors.New("kafka broker unreachable")
	pub := &mockPublisher{err: wantErr}

	w := worker.NewOutboxWorker(repo, pub, 10*time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()

	w.Start(ctx)

	if got := len(repo.failedIDs); got == 0 {
		t.Errorf("expected failedIDs to be recorded when publish fails, got 0")
	}
	if !errors.Is(repo.lastErr, wantErr) {
		t.Errorf("lastErr = %v, want %v", repo.lastErr, wantErr)
	}
}
