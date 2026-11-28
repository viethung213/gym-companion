package worker_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/gamification/application/command"
	"github.com/viethung213/gym-companion/internal/gamification/infrastructure/worker"
)

type scriptedRebuilder struct {
	mu      sync.Mutex
	resets  []bool
	results []error
	runs    chan struct{}
	// during runs inside Handle, like the reads that hit a not-ready board while it rebuilds.
	during func()
}

func newScriptedRebuilder(results ...error) *scriptedRebuilder {
	return &scriptedRebuilder{results: results, runs: make(chan struct{}, 16)}
}

func (r *scriptedRebuilder) Handle(_ context.Context, cmd command.RebuildLeaderboardCommand) (command.RebuildLeaderboardResult, error) {
	r.mu.Lock()
	idx := len(r.resets)
	r.resets = append(r.resets, cmd.Reset)
	var err error
	if idx < len(r.results) {
		err = r.results[idx]
	}
	during := r.during
	r.mu.Unlock()
	if during != nil {
		during()
	}
	r.runs <- struct{}{}
	return command.RebuildLeaderboardResult{Projected: 3}, err
}

func (r *scriptedRebuilder) resetFlags() []bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]bool(nil), r.resets...)
}

func waitRuns(t *testing.T, r *scriptedRebuilder, n int) {
	t.Helper()
	for i := range n {
		select {
		case <-r.runs:
		case <-time.After(2 * time.Second):
			t.Fatalf("timed out waiting for run %d of %d", i+1, n)
		}
	}
}

// requestUntilRun keeps requesting, as reads of a not-ready board do, until a run starts. A single
// request can be discarded when it lands while the previous successful run is finishing.
func requestUntilRun(t *testing.T, w *worker.LeaderboardReconcileWorker, r *scriptedRebuilder) {
	t.Helper()
	deadline := time.After(2 * time.Second)
	tick := time.NewTicker(5 * time.Millisecond)
	defer tick.Stop()
	for {
		w.RequestRebuild()
		select {
		case <-r.runs:
			return
		case <-tick.C:
		case <-deadline:
			t.Fatal("timed out waiting for a requested run")
		}
	}
}

func startWorker(t *testing.T, w *worker.LeaderboardReconcileWorker) (stop func()) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		w.Start(ctx)
		close(done)
	}()
	return func() {
		cancel()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Fatal("worker did not stop after cancellation")
		}
	}
}

func TestReconcileWorker_FirstRunResetsThenReconciles(t *testing.T) {
	rebuilder := newScriptedRebuilder()
	stop := startWorker(t, worker.NewLeaderboardReconcileWorker(rebuilder, 10*time.Millisecond, time.Minute))
	waitRuns(t, rebuilder, 3)
	stop()

	flags := rebuilder.resetFlags()
	if !flags[0] || flags[1] || flags[2] {
		t.Fatalf("reset flags = %v, want [true false false ...]", flags)
	}
}

func TestReconcileWorker_ResetRetriedUntilSuccess(t *testing.T) {
	rebuilder := newScriptedRebuilder(errors.New("redis down"), errors.New("redis down"), nil)
	stop := startWorker(t, worker.NewLeaderboardReconcileWorker(rebuilder, 10*time.Millisecond, time.Minute))
	waitRuns(t, rebuilder, 4)
	stop()

	flags := rebuilder.resetFlags()
	if !flags[0] || !flags[1] || !flags[2] || flags[3] {
		t.Fatalf("reset flags = %v, want [true true true false ...]", flags)
	}
}

func TestReconcileWorker_RequestRebuildTriggersEarlyRun(t *testing.T) {
	rebuilder := newScriptedRebuilder()
	w := worker.NewLeaderboardReconcileWorker(rebuilder, time.Hour, time.Minute)
	stop := startWorker(t, w)
	waitRuns(t, rebuilder, 1)

	requestUntilRun(t, w, rebuilder)
	stop()

	if flags := rebuilder.resetFlags(); len(flags) != 2 || flags[1] {
		t.Fatalf("reset flags = %v, want [true false]", flags)
	}
}

func TestReconcileWorker_RequestRebuildNonBlocking(t *testing.T) {
	w := worker.NewLeaderboardReconcileWorker(newScriptedRebuilder(), time.Hour, time.Minute)
	done := make(chan struct{})
	go func() {
		for range 5 {
			w.RequestRebuild()
		}
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("RequestRebuild blocked while the worker was not running")
	}
}

func TestReconcileWorker_DefaultsAreNotTight(t *testing.T) {
	rebuilder := newScriptedRebuilder()
	stop := startWorker(t, worker.NewLeaderboardReconcileWorker(rebuilder, 0, 0))
	waitRuns(t, rebuilder, 1)

	select {
	case <-rebuilder.runs:
		t.Fatal("a second run started immediately; the default interval must be long (15m)")
	case <-time.After(200 * time.Millisecond):
	}
	stop()
}

type blockingRebuilder struct {
	errs chan error
}

func (b *blockingRebuilder) Handle(
	ctx context.Context,
	_ command.RebuildLeaderboardCommand,
) (command.RebuildLeaderboardResult, error) {
	<-ctx.Done()
	b.errs <- ctx.Err()
	return command.RebuildLeaderboardResult{}, ctx.Err()
}

func TestReconcileWorker_RunTimeoutBoundsHungRebuild(t *testing.T) {
	rebuilder := &blockingRebuilder{errs: make(chan error, 1)}
	stop := startWorker(t, worker.NewLeaderboardReconcileWorker(rebuilder, time.Hour, 20*time.Millisecond))
	defer stop()

	select {
	case err := <-rebuilder.errs:
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("run ctx err = %v, want DeadlineExceeded", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("hung rebuild was not cancelled by the run timeout")
	}
}

func TestReconcileWorker_RequestsDuringSuccessfulRunAreSatisfiedByIt(t *testing.T) {
	rebuilder := newScriptedRebuilder()
	w := worker.NewLeaderboardReconcileWorker(rebuilder, time.Hour, time.Minute)
	rebuilder.during = w.RequestRebuild
	stop := startWorker(t, w)
	waitRuns(t, rebuilder, 1)

	select {
	case <-rebuilder.runs:
		t.Fatal("a request raised during a successful run started another full rebuild")
	case <-time.After(200 * time.Millisecond):
	}
	stop()
}

func TestReconcileWorker_RequestsDuringFailedRunRetry(t *testing.T) {
	rebuilder := newScriptedRebuilder(errors.New("redis down"))
	w := worker.NewLeaderboardReconcileWorker(rebuilder, time.Hour, time.Minute)
	rebuilder.during = w.RequestRebuild
	stop := startWorker(t, w)
	waitRuns(t, rebuilder, 2)
	stop()

	if flags := rebuilder.resetFlags(); !flags[0] || !flags[1] {
		t.Fatalf("reset flags = %v, want the failed reset retried on request", flags)
	}
}
