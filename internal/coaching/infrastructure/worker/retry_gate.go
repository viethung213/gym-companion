package worker

import (
	"sync"
	"time"
)

// retryGate spaces out the retries of a model-backed job that failed, so a
// failure that repeats (a draft the guardrail keeps rejecting) does not cost a
// full generation every tick. It only remembers roadmaps still listed, so its
// size is bounded by the work due.
type retryGate struct {
	delay time.Duration
	mu    sync.Mutex
	next  map[string]time.Time // roadmap ID -> earliest retry
}

func newRetryGate(delay time.Duration) *retryGate {
	return &retryGate{delay: delay, next: make(map[string]time.Time)}
}

// due returns the ids whose last failure is old enough to retry, and forgets
// the failures of roadmaps no longer listed.
func (g *retryGate) due(ids []string, now time.Time) []string {
	g.mu.Lock()
	defer g.mu.Unlock()
	listed := make(map[string]bool, len(ids))
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		listed[id] = true
		if next, failed := g.next[id]; !failed || !now.Before(next) {
			out = append(out, id)
		}
	}
	for id := range g.next {
		if !listed[id] {
			delete(g.next, id)
		}
	}
	return out
}

// record notes the outcome of a job run at now.
func (g *retryGate) record(id string, now time.Time, ok bool) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if ok {
		delete(g.next, id)
		return
	}
	g.next[id] = now.Add(g.delay)
}
