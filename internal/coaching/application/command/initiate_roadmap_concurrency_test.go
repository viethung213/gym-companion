package command

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/coaching/domain/guardrail"
	"github.com/viethung213/gym-companion/internal/coaching/domain/roadmap"
	"github.com/viethung213/gym-companion/internal/coaching/domain/service"
)

// gatedCoachAgent counts generations and holds each until release closes.
type gatedCoachAgent struct {
	fakeCoachAgent
	calls   atomic.Int32
	started chan struct{}
	once    sync.Once
	release chan struct{}
}

func (g *gatedCoachAgent) GenerateRoadmap(ctx context.Context, userID string) (*roadmap.Roadmap, error) {
	g.calls.Add(1)
	g.once.Do(func() { close(g.started) })
	<-g.release
	return g.fakeCoachAgent.GenerateRoadmap(ctx, userID)
}

// syncMemRepo guards memRepo for concurrent callers.
type syncMemRepo struct {
	mu sync.Mutex
	*memRepo
}

func (s *syncMemRepo) Save(ctx context.Context, r *roadmap.Roadmap) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.memRepo.Save(ctx, r)
}

func (s *syncMemRepo) FindActiveByUser(ctx context.Context, userID string) (*roadmap.Roadmap, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.memRepo.FindActiveByUser(ctx, userID)
}

// TestInitiateRoadmap_ConcurrentCallsShareOneGeneration covers onboarding: the
// RPC and the ProfileCompleted consumer initiate the same user at once.
func TestInitiateRoadmap_ConcurrentCallsShareOneGeneration(t *testing.T) {
	clock := &fakeClock{t: time.Date(2026, 7, 28, 12, 0, 0, 0, time.UTC)}
	agent := &gatedCoachAgent{
		fakeCoachAgent: fakeCoachAgent{t: clock.t},
		started:        make(chan struct{}),
		release:        make(chan struct{}),
	}
	repo := &syncMemRepo{memRepo: newMemRepo()}
	h := NewInitiateRoadmapHandler(fakeTx{}, repo, agent,
		guardrail.NewEngine(service.NewOverloadValidator(), nil, nil), fakeProfileReader{}, &captureOutbox{}, clock)

	const callers = 3
	results := make(chan error, callers)
	var wg sync.WaitGroup
	call := func() {
		defer wg.Done()
		_, err := h.Handle(context.Background(), InitiateRoadmapCommand{UserID: "user-1"})
		results <- err
	}
	wg.Add(1)
	go call()
	<-agent.started
	for range callers - 1 {
		wg.Add(1)
		go call()
	}
	time.Sleep(50 * time.Millisecond) // let the others join the in-flight generation
	close(agent.release)
	wg.Wait()
	close(results)

	for err := range results {
		if err != nil {
			t.Fatalf("Handle: %v", err)
		}
	}
	if got := agent.calls.Load(); got != 1 {
		t.Fatalf("got %d generations, want 1", got)
	}

	// A later call sees the saved roadmap.
	if _, err := h.Handle(context.Background(), InitiateRoadmapCommand{UserID: "user-1"}); !errors.Is(err, roadmap.ErrActiveRoadmapExists) {
		t.Fatalf("got %v, want ErrActiveRoadmapExists", err)
	}
}
