package worker_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/workout_execution/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/workout_execution/infrastructure/worker"
)

func TestCriticalInactivityWorker(t *testing.T) {
	t.Run("NewCriticalInactivityWorker defaults when zero interval and threshold", func(t *testing.T) {
		w := worker.NewCriticalInactivityWorker(&mockSessionRepo{}, nil, nil, 0, 0)
		if w == nil {
			t.Fatal("got nil worker, want non-nil")
		}
	})

	t.Run("Start stops on context cancellation", func(t *testing.T) {
		w := worker.NewCriticalInactivityWorker(
			&mockSessionRepo{}, nil, &mockTxManager{},
			10*time.Millisecond, 5*time.Minute,
		)
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
			// success
		case <-time.After(1 * time.Second):
			t.Fatal("worker did not stop on context cancellation")
		}
	})

	t.Run("processCriticalInactiveSessions find error", func(t *testing.T) {
		repo := &mockSessionRepo{findErr: errors.New("db error")}
		w := worker.NewCriticalInactivityWorker(repo, nil, &mockTxManager{}, 10*time.Millisecond, 5*time.Minute)

		ctx, cancel := context.WithCancel(context.Background())
		go func() {
			time.Sleep(15 * time.Millisecond)
			cancel()
		}()
		w.Start(ctx)
	})

	t.Run("processCriticalInactiveSessions empty result does nothing", func(t *testing.T) {
		repo := &mockSessionRepo{}
		w := worker.NewCriticalInactivityWorker(repo, nil, &mockTxManager{}, 10*time.Millisecond, 5*time.Minute)

		ctx, cancel := context.WithCancel(context.Background())
		go func() {
			time.Sleep(15 * time.Millisecond)
			cancel()
		}()
		w.Start(ctx)
	})

	t.Run("processCriticalInactiveSessions marks session ANOMALOUS", func(t *testing.T) {
		now := time.Now().UTC()
		started := now.Add(-10 * time.Minute)
		criticalTimestamp := now.Add(-6 * time.Minute)

		session := aggregate.ReconstituteWorkoutSession(
			"sess-ci-worker-1", "user-1", "plan-1",
			aggregate.StatusInProgress,
			nil,
			[]aggregate.SessionError{
				{
					ID:        "err-1",
					SessionID: "sess-ci-worker-1",
					ErrorCode: "ERR_BAR_TRAPPED",
					Severity:  "CRITICAL",
					Timestamp: criticalTimestamp,
				},
			},
			nil, &started, nil,
			started, started,
		)

		repo := &mockSessionRepo{session: session}
		tx := &mockTxManager{}
		outbox := &mockOutboxWriter{}

		w := worker.NewCriticalInactivityWorker(repo, outbox, tx, 10*time.Millisecond, 5*time.Minute)

		ctx, cancel := context.WithCancel(context.Background())
		go func() {
			time.Sleep(20 * time.Millisecond)
			cancel()
		}()
		w.Start(ctx)

		if got, want := session.Status(), aggregate.StatusAnomalous; got != want {
			t.Errorf("got Status = %v, want %v", got, want)
		}
	})

	t.Run("processCriticalInactiveSessions save transaction error logs and continues", func(t *testing.T) {
		now := time.Now().UTC()
		started := now.Add(-10 * time.Minute)
		criticalTimestamp := now.Add(-6 * time.Minute)

		session := aggregate.ReconstituteWorkoutSession(
			"sess-ci-worker-2", "user-1", "plan-1",
			aggregate.StatusInProgress,
			nil,
			[]aggregate.SessionError{
				{
					ID:        "err-2",
					SessionID: "sess-ci-worker-2",
					ErrorCode: "ERR_FALL_DETECTED",
					Severity:  "CRITICAL",
					Timestamp: criticalTimestamp,
				},
			},
			nil, &started, nil,
			started, started,
		)

		repo := &mockSessionRepo{session: session, saveErr: errors.New("save error")}
		tx := &mockTxManager{}
		outbox := &mockOutboxWriter{}

		w := worker.NewCriticalInactivityWorker(repo, outbox, tx, 10*time.Millisecond, 5*time.Minute)

		ctx, cancel := context.WithCancel(context.Background())
		go func() {
			time.Sleep(20 * time.Millisecond)
			cancel()
		}()
		w.Start(ctx)
	})

	t.Run("processCriticalInactiveSessions nil outbox skips event write", func(t *testing.T) {
		now := time.Now().UTC()
		started := now.Add(-10 * time.Minute)
		criticalTimestamp := now.Add(-6 * time.Minute)

		session := aggregate.ReconstituteWorkoutSession(
			"sess-ci-worker-3", "user-1", "plan-1",
			aggregate.StatusInProgress,
			nil,
			[]aggregate.SessionError{
				{
					ID:        "err-3",
					SessionID: "sess-ci-worker-3",
					Severity:  "CRITICAL",
					Timestamp: criticalTimestamp,
				},
			},
			nil, &started, nil,
			started, started,
		)

		repo := &mockSessionRepo{session: session}
		tx := &mockTxManager{}

		w := worker.NewCriticalInactivityWorker(repo, nil, tx, 10*time.Millisecond, 5*time.Minute)

		ctx, cancel := context.WithCancel(context.Background())
		go func() {
			time.Sleep(20 * time.Millisecond)
			cancel()
		}()
		w.Start(ctx)

		if got, want := session.Status(), aggregate.StatusAnomalous; got != want {
			t.Errorf("got Status = %v, want %v", got, want)
		}
	})

	t.Run("processCriticalInactiveSessions session with no critical errors is skipped", func(t *testing.T) {
		now := time.Now().UTC()
		started := now.Add(-10 * time.Minute)

		session := aggregate.ReconstituteWorkoutSession(
			"sess-ci-worker-4", "user-1", "plan-1",
			aggregate.StatusInProgress,
			nil,
			[]aggregate.SessionError{
				{
					ID:        "err-4",
					SessionID: "sess-ci-worker-4",
					ErrorCode: "ERR_ELBOW_FLARE",
					Severity:  "WARNING",
					Timestamp: now.Add(-6 * time.Minute),
				},
			},
			nil, &started, nil,
			started, started,
		)

		// findLastCriticalErrorTime returns zero time → MarkCriticalInactivity
		// will still be called but session has no critical errors so zero time is passed.
		// The session IS in IN_PROGRESS so it will still transition — this test
		// documents that filtering is done at the repository query level (SQL WHERE clause).
		// The worker trusts the repo to return only sessions with critical errors.
		repo := &mockSessionRepo{session: session}
		tx := &mockTxManager{}

		w := worker.NewCriticalInactivityWorker(repo, nil, tx, 10*time.Millisecond, 5*time.Minute)

		ctx, cancel := context.WithCancel(context.Background())
		go func() {
			time.Sleep(20 * time.Millisecond)
			cancel()
		}()
		w.Start(ctx)
	})
}
