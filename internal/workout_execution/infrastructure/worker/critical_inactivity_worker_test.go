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

	t.Run("processCriticalInactiveSessions marks session ANOMALOUS on continuous errors in recent 5m", func(t *testing.T) {
		now := time.Now().UTC()
		started := now.Add(-10 * time.Minute)

		session := aggregate.ReconstituteWorkoutSession(
			"sess-ci-worker-1", "user-1", "plan-1",
			aggregate.StatusInProgress,
			nil,
			[]aggregate.SessionError{
				{
					ID:        "err-1a",
					SessionID: "sess-ci-worker-1",
					ErrorCode: "ERR_BAR_TRAPPED",
					Severity:  "CRITICAL",
					Timestamp: now.Add(-4 * time.Minute),
				},
				{
					ID:        "err-1b",
					SessionID: "sess-ci-worker-1",
					ErrorCode: "ERR_BAR_TRAPPED",
					Severity:  "CRITICAL",
					Timestamp: now.Add(-2 * time.Minute),
				},
				{
					ID:        "err-1c",
					SessionID: "sess-ci-worker-1",
					ErrorCode: "ERR_BAR_TRAPPED",
					Severity:  "CRITICAL",
					Timestamp: now.Add(-30 * time.Second),
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

		session := aggregate.ReconstituteWorkoutSession(
			"sess-ci-worker-2", "user-1", "plan-1",
			aggregate.StatusInProgress,
			nil,
			[]aggregate.SessionError{
				{
					ID:        "err-2a",
					SessionID: "sess-ci-worker-2",
					ErrorCode: "ERR_FALL_DETECTED",
					Severity:  "CRITICAL",
					Timestamp: now.Add(-4 * time.Minute),
				},
				{
					ID:        "err-2b",
					SessionID: "sess-ci-worker-2",
					ErrorCode: "ERR_FALL_DETECTED",
					Severity:  "CRITICAL",
					Timestamp: now.Add(-2 * time.Minute),
				},
				{
					ID:        "err-2c",
					SessionID: "sess-ci-worker-2",
					ErrorCode: "ERR_FALL_DETECTED",
					Severity:  "CRITICAL",
					Timestamp: now.Add(-30 * time.Second),
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

		session := aggregate.ReconstituteWorkoutSession(
			"sess-ci-worker-3", "user-1", "plan-1",
			aggregate.StatusInProgress,
			nil,
			[]aggregate.SessionError{
				{
					ID:        "err-3a",
					SessionID: "sess-ci-worker-3",
					ErrorCode: "ERR_BAR_TRAPPED",
					Severity:  "CRITICAL",
					Timestamp: now.Add(-4 * time.Minute),
				},
				{
					ID:        "err-3b",
					SessionID: "sess-ci-worker-3",
					ErrorCode: "ERR_BAR_TRAPPED",
					Severity:  "CRITICAL",
					Timestamp: now.Add(-2 * time.Minute),
				},
				{
					ID:        "err-3c",
					SessionID: "sess-ci-worker-3",
					ErrorCode: "ERR_BAR_TRAPPED",
					Severity:  "CRITICAL",
					Timestamp: now.Add(-30 * time.Second),
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
					Timestamp: now.Add(-3 * time.Minute),
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

		if got, want := session.Status(), aggregate.StatusInProgress; got != want {
			t.Errorf("got Status = %v, want %v (non-critical error session should remain in progress)", got, want)
		}
	})

	t.Run("processCriticalInactiveSessions isolated errors 5m apart are NOT marked anomalous", func(t *testing.T) {
		now := time.Now().UTC()
		started := now.Add(-15 * time.Minute)
		errorAt1h := now.Add(-6 * time.Minute)
		errorAt1h5p := now.Add(-1 * time.Minute) // Only 2 isolated errors, not continuous

		session := aggregate.ReconstituteWorkoutSession(
			"sess-ci-worker-5-isolated", "user-1", "plan-1",
			aggregate.StatusInProgress,
			nil,
			[]aggregate.SessionError{
				{
					ID:        "err-5-old",
					SessionID: "sess-ci-worker-5-isolated",
					ErrorCode: "ERR_BAR_TRAPPED",
					Severity:  "CRITICAL",
					Timestamp: errorAt1h,
				},
				{
					ID:        "err-5-recent",
					SessionID: "sess-ci-worker-5-isolated",
					ErrorCode: "ERR_BAR_TRAPPED",
					Severity:  "CRITICAL",
					Timestamp: errorAt1h5p,
				},
			},
			nil, &started, nil,
			started, now,
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

		if got, want := session.Status(), aggregate.StatusInProgress; got != want {
			t.Errorf("got Status = %v, want %v (isolated errors should NOT trigger anomaly)", got, want)
		}
	})

	t.Run("processCriticalInactiveSessions repeated continuous critical errors in recent 5m marks ANOMALOUS", func(t *testing.T) {
		now := time.Now().UTC()
		started := now.Add(-10 * time.Minute)

		session := aggregate.ReconstituteWorkoutSession(
			"sess-ci-worker-5-continuous", "user-1", "plan-1",
			aggregate.StatusInProgress,
			nil,
			[]aggregate.SessionError{
				{
					ID:        "err-5a",
					SessionID: "sess-ci-worker-5-continuous",
					ErrorCode: "ERR_BAR_TRAPPED",
					Severity:  "CRITICAL",
					Timestamp: now.Add(-4 * time.Minute),
				},
				{
					ID:        "err-5b",
					SessionID: "sess-ci-worker-5-continuous",
					ErrorCode: "ERR_BAR_TRAPPED",
					Severity:  "CRITICAL",
					Timestamp: now.Add(-2 * time.Minute),
				},
				{
					ID:        "err-5c",
					SessionID: "sess-ci-worker-5-continuous",
					ErrorCode: "ERR_BAR_TRAPPED",
					Severity:  "CRITICAL",
					Timestamp: now.Add(-30 * time.Second),
				},
			},
			nil, &started, nil,
			started, now,
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

	t.Run("processCriticalInactiveSessions different critical errors in recent 5m are NOT marked anomalous", func(t *testing.T) {
		now := time.Now().UTC()
		started := now.Add(-10 * time.Minute)

		// 3 different errors occurring in the 5-minute window (means user is actively moving/exercising, not stuck)
		session := aggregate.ReconstituteWorkoutSession(
			"sess-ci-worker-different-errors", "user-1", "plan-1",
			aggregate.StatusInProgress,
			nil,
			[]aggregate.SessionError{
				{
					ID:        "err-diff-1",
					SessionID: "sess-ci-worker-different-errors",
					ErrorCode: "ERR_BAR_TRAPPED",
					Severity:  "CRITICAL",
					Timestamp: now.Add(-4 * time.Minute),
				},
				{
					ID:        "err-diff-2",
					SessionID: "sess-ci-worker-different-errors",
					ErrorCode: "ERR_FALL_DETECTED",
					Severity:  "CRITICAL",
					Timestamp: now.Add(-2 * time.Minute),
				},
				{
					ID:        "err-diff-3",
					SessionID: "sess-ci-worker-different-errors",
					ErrorCode: "ERR_ANOTHER_CRITICAL",
					Severity:  "CRITICAL",
					Timestamp: now.Add(-30 * time.Second),
				},
			},
			nil, &started, nil,
			started, now,
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

		if got, want := session.Status(), aggregate.StatusInProgress; got != want {
			t.Errorf("got Status = %v, want %v (different errors mean user is still active, should remain IN_PROGRESS)", got, want)
		}
	})
}
