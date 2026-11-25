package worker

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/coaching/application/command"
	"github.com/viethung213/gym-companion/internal/coaching/application/port"
	"github.com/viethung213/gym-companion/internal/coaching/domain/roadmap"
)

type fakeLifecycleQueue struct {
	ended, toReview, toPlan []string
}

func (q *fakeLifecycleQueue) RoadmapIDsWithOpenWeekEndingBy(context.Context, time.Time) ([]string, error) {
	return q.toReview, nil
}

// ActiveRoadmapsEndingBy answers the roll-over query (about a day ahead) with
// the ended users and the planning query (a week further) with toPlan.
func (q *fakeLifecycleQueue) ActiveRoadmapsEndingBy(_ context.Context, date time.Time) ([]port.RoadmapRef, error) {
	refs := make([]port.RoadmapRef, 0, len(q.ended)+len(q.toPlan))
	if date.After(time.Now().AddDate(0, 0, 3)) {
		for _, id := range q.toPlan {
			refs = append(refs, port.RoadmapRef{RoadmapID: id})
		}
		return refs, nil
	}
	for _, u := range q.ended {
		refs = append(refs, port.RoadmapRef{UserID: u})
	}
	return refs, nil
}

func (q *fakeLifecycleQueue) FindPlannedByUser(context.Context, string) (*roadmap.Roadmap, error) {
	return nil, roadmap.ErrRoadmapNotFound
}

func (q *fakeLifecycleQueue) DeletePlannedRoadmap(context.Context, string) error { return nil }

// recorder logs the order of steps and tracks how many jobs run at once.
type recorder struct {
	mu       sync.Mutex
	steps    []string
	running  atomic.Int32
	peak     atomic.Int32
	full     chan struct{} // closed once maxConcurrentGenerations jobs run at once
	fullOnce sync.Once
}

func newRecorder() *recorder { return &recorder{full: make(chan struct{})} }

func (r *recorder) log(step string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.steps = append(r.steps, step)
}

// slowJob holds every job until the limit of jobs runs at once (a barrier, so
// the test does not depend on timing), then records the peak. The timeout only
// stops a worker that never reaches the limit from hanging the test.
func (r *recorder) slowJob(step string) {
	n := r.running.Add(1)
	for {
		p := r.peak.Load()
		if n <= p || r.peak.CompareAndSwap(p, n) {
			break
		}
	}
	if n >= maxConcurrentGenerations {
		r.fullOnce.Do(func() { close(r.full) })
	}
	select {
	case <-r.full:
	case <-time.After(2 * time.Second):
	}
	r.running.Add(-1)
	r.log(step)
}

type (
	recRollOver struct{ r *recorder }
	recReviewer struct{ r *recorder }
	recPlanner  struct{ r *recorder }
)

func (f recRollOver) Handle(context.Context, string) (*command.RollOverResult, error) {
	f.r.log("rollover")
	return &command.RollOverResult{}, nil
}

func (f recReviewer) Handle(context.Context, string) (*command.ReviewWeeksResult, error) {
	f.r.slowJob("review")
	return &command.ReviewWeeksResult{}, nil
}

func (f recPlanner) Handle(context.Context, string) (*roadmap.Roadmap, error) {
	f.r.slowJob("plan")
	return nil, nil
}

// Roll-overs come first, then every review, then every plan; model-backed
// jobs run in parallel but never more than maxConcurrentGenerations at once.
func TestRoadmapLifecycleWorker_RunOnce(t *testing.T) {
	ids := func(n int) []string {
		out := make([]string, n)
		for i := range out {
			out[i] = string(rune('a' + i))
		}
		return out
	}
	q := &fakeLifecycleQueue{ended: ids(2), toReview: ids(7), toPlan: ids(5)}
	rec := newRecorder()
	w := NewRoadmapLifecycleWorker(q, recRollOver{rec}, recReviewer{rec}, recPlanner{rec}, time.Hour)

	w.RunOnce(context.Background(), time.Now())

	if len(rec.steps) != 14 {
		t.Fatalf("ran %d jobs, want 14: %v", len(rec.steps), rec.steps)
	}
	phase := map[string]int{"rollover": 0, "review": 1, "plan": 2}
	for i := 1; i < len(rec.steps); i++ {
		if phase[rec.steps[i]] < phase[rec.steps[i-1]] {
			t.Fatalf("steps out of order: %v", rec.steps)
		}
	}
	if p := rec.peak.Load(); p != maxConcurrentGenerations {
		t.Errorf("peak concurrency = %d, want exactly %d (parallel, capped)", p, maxConcurrentGenerations)
	}
}

// unplannedRollOver ends every roadmap; users in "planned" get a successor.
type unplannedRollOver struct{ planned map[string]bool }

func (f unplannedRollOver) Handle(_ context.Context, userID string) (*command.RollOverResult, error) {
	if f.planned[userID] {
		return &command.RollOverResult{RolledOver: true, Active: &roadmap.Roadmap{}}, nil
	}
	return &command.RollOverResult{RolledOver: true}, nil
}

type recInitiator struct {
	mu    sync.Mutex
	users []string
}

func (r *recInitiator) Handle(_ context.Context, cmd command.InitiateRoadmapCommand) (*command.InitiateRoadmapResult, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.users = append(r.users, cmd.UserID)
	return &command.InitiateRoadmapResult{Roadmap: &roadmap.Roadmap{}}, nil
}

// An athlete whose block ended with no successor gets a fresh roadmap in the
// same tick; one rolled over to a planned block does not.
func TestRoadmapLifecycleWorker_StartsFreshWhenNothingWasPlanned(t *testing.T) {
	q := &fakeLifecycleQueue{ended: []string{"left-empty", "has-plan"}}
	rec := newRecorder()
	init := &recInitiator{}
	w := NewRoadmapLifecycleWorker(q, unplannedRollOver{planned: map[string]bool{"has-plan": true}},
		recReviewer{rec}, recPlanner{rec}, time.Hour).WithInitiator(init)

	w.RunOnce(context.Background(), time.Now())

	if len(init.users) != 1 || init.users[0] != "left-empty" {
		t.Errorf("fresh roadmaps started for %v, want only left-empty", init.users)
	}
}
