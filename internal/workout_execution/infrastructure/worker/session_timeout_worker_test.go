package worker_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/workout_execution/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/workout_execution/infrastructure/worker"
)

func TestSessionTimeoutWorker(t *testing.T) {
	t.Run("NewSessionTimeoutWorker constructor defaults and custom interval", func(t *testing.T) {
		wDefault := worker.NewSessionTimeoutWorker(&mockSessionRepo{}, nil, nil, 0)
		if wDefault == nil {
			t.Fatal("got nil worker, want non-nil")
		}

		wCustom := worker.NewSessionTimeoutWorker(&mockSessionRepo{}, &mockOutboxWriter{}, &mockTxManager{}, 100*time.Millisecond)
		if wCustom == nil {
			t.Fatal("got nil worker, want non-nil")
		}
	})

	t.Run("Start worker context cancellation", func(t *testing.T) {
		w := worker.NewSessionTimeoutWorker(&mockSessionRepo{}, nil, &mockTxManager{}, 10*time.Millisecond)
		ctx, cancel := context.WithCancel(context.Background())

		done := make(chan struct{})
		go func() {
			w.Start(ctx)
			close(done)
		}()

		time.Sleep(25 * time.Millisecond)
		cancel()

		select {
		case <-done:
			// Success
		case <-time.After(1 * time.Second):
			t.Fatal("worker did not stop on context cancellation")
		}
	})

	t.Run("processTimedOutSessions find error", func(t *testing.T) {
		repo := &mockSessionRepo{findErr: errors.New("db error")}
		w := worker.NewSessionTimeoutWorker(repo, nil, &mockTxManager{}, 10*time.Millisecond)

		ctx, cancel := context.WithCancel(context.Background())
		go func() {
			time.Sleep(15 * time.Millisecond)
			cancel()
		}()
		w.Start(ctx)
	})

	t.Run("processTimedOutSessions empty sessions", func(t *testing.T) {
		repo := &mockSessionRepo{}
		w := worker.NewSessionTimeoutWorker(repo, nil, &mockTxManager{}, 10*time.Millisecond)

		ctx, cancel := context.WithCancel(context.Background())
		go func() {
			time.Sleep(15 * time.Millisecond)
			cancel()
		}()
		w.Start(ctx)
	})

	t.Run("processTimedOutSessions successfully aborts timed out session", func(t *testing.T) {
		now := time.Now().UTC()
		started := now.Add(-250 * time.Minute)
		session := aggregate.ReconstituteWorkoutSession(
			"sess-timeout-1", "user-1", "plan-1",
			aggregate.StatusInProgress,
			nil, nil, nil, &started, nil,
			started, started,
		)

		repo := &mockSessionRepo{session: session}
		tx := &mockTxManager{}
		outbox := &mockOutboxWriter{}

		w := worker.NewSessionTimeoutWorker(repo, outbox, tx, 10*time.Millisecond)

		ctx, cancel := context.WithCancel(context.Background())
		go func() {
			time.Sleep(20 * time.Millisecond)
			cancel()
		}()
		w.Start(ctx)

		if session.Status() != aggregate.StatusAnomalous {
			t.Errorf("got session status = %v, want %v", session.Status(), aggregate.StatusAnomalous)
		}
	})

	t.Run("processTimedOutSessions transaction save error", func(t *testing.T) {
		now := time.Now().UTC()
		started := now.Add(-250 * time.Minute)
		session := aggregate.ReconstituteWorkoutSession(
			"sess-timeout-2", "user-1", "plan-1",
			aggregate.StatusInProgress,
			nil, nil, nil, &started, nil,
			started, started,
		)

		repo := &mockSessionRepo{session: session, saveErr: errors.New("save error")}
		tx := &mockTxManager{}
		outbox := &mockOutboxWriter{}

		w := worker.NewSessionTimeoutWorker(repo, outbox, tx, 10*time.Millisecond)

		ctx, cancel := context.WithCancel(context.Background())
		go func() {
			time.Sleep(20 * time.Millisecond)
			cancel()
		}()
		w.Start(ctx)
	})

	t.Run("processTimedOutSessions nil outbox writer", func(t *testing.T) {
		now := time.Now().UTC()
		started := now.Add(-250 * time.Minute)
		session := aggregate.ReconstituteWorkoutSession(
			"sess-timeout-3", "user-1", "plan-1",
			aggregate.StatusInProgress,
			nil, nil, nil, &started, nil,
			started, started,
		)

		repo := &mockSessionRepo{session: session}
		tx := &mockTxManager{}

		w := worker.NewSessionTimeoutWorker(repo, nil, tx, 10*time.Millisecond)

		ctx, cancel := context.WithCancel(context.Background())
		go func() {
			time.Sleep(20 * time.Millisecond)
			cancel()
		}()
		w.Start(ctx)
	})

	t.Run("processTimedOutSessions outbox write error", func(t *testing.T) {
		now := time.Now().UTC()
		started := now.Add(-250 * time.Minute)
		session := aggregate.ReconstituteWorkoutSession(
			"sess-timeout-4", "user-1", "plan-1",
			aggregate.StatusInProgress,
			nil, nil, nil, &started, nil,
			started, started,
		)

		repo := &mockSessionRepo{session: session}
		tx := &mockTxManager{}
		outbox := &mockOutboxWriter{err: errors.New("outbox write err")}

		w := worker.NewSessionTimeoutWorker(repo, outbox, tx, 10*time.Millisecond)

		ctx, cancel := context.WithCancel(context.Background())
		go func() {
			time.Sleep(20 * time.Millisecond)
			cancel()
		}()
		w.Start(ctx)
	})

	t.Run("processTimedOutSessions session not timed out", func(t *testing.T) {
		now := time.Now().UTC()
		started := now.Add(-10 * time.Minute) // Only 10 mins ago
		session := aggregate.ReconstituteWorkoutSession(
			"sess-active-1", "user-1", "plan-1",
			aggregate.StatusInProgress,
			nil, nil, nil, &started, nil,
			started, started,
		)

		repo := &mockSessionRepo{session: session}
		tx := &mockTxManager{}
		outbox := &mockOutboxWriter{}

		w := worker.NewSessionTimeoutWorker(repo, outbox, tx, 10*time.Millisecond)

		ctx, cancel := context.WithCancel(context.Background())
		go func() {
			time.Sleep(20 * time.Millisecond)
			cancel()
		}()
		w.Start(ctx)

		if session.Status() != aggregate.StatusInProgress {
			t.Errorf("got session status = %v, want %v", session.Status(), aggregate.StatusInProgress)
		}
	})
}
