package command

import (
	"context"
	"errors"
	"strconv"
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/coaching/application/contextbuilder"
	"github.com/viethung213/gym-companion/internal/coaching/application/port"
	domainevent "github.com/viethung213/gym-companion/internal/coaching/domain/event"
	"github.com/viethung213/gym-companion/internal/coaching/domain/roadmap"
	"github.com/viethung213/gym-companion/internal/coaching/domain/service"
	"github.com/viethung213/gym-companion/internal/coaching/infrastructure/ai"
	"github.com/viethung213/gym-companion/internal/coaching/infrastructure/guardrail"
)

// ---- mocks ----

type fakeClock struct{ t time.Time }

func (c *fakeClock) Now() time.Time { return c.t }

type incrIDs struct{ n int }

func (i *incrIDs) NewID() string {
	i.n++
	return "id-" + strconv.Itoa(i.n)
}

type fakeTx struct{}

func (fakeTx) WithTransaction(ctx context.Context, fn func(context.Context) error) error {
	return fn(ctx)
}

type memRepo struct {
	byID       map[string]*roadmap.Roadmap
	activeByID map[string]string // userID -> roadmap.ID
}

func newMemRepo() *memRepo {
	return &memRepo{byID: map[string]*roadmap.Roadmap{}, activeByID: map[string]string{}}
}

func (m *memRepo) Save(_ context.Context, r *roadmap.Roadmap) error {
	m.byID[r.ID()] = r
	if r.Status() == roadmap.RoadmapStatusActive {
		m.activeByID[r.UserID()] = r.ID()
	}
	return nil
}
func (m *memRepo) FindByID(_ context.Context, id string) (*roadmap.Roadmap, error) {
	if r, ok := m.byID[id]; ok {
		return r, nil
	}
	return nil, roadmap.ErrRoadmapNotFound
}
func (m *memRepo) FindActiveByUser(_ context.Context, userID string) (*roadmap.Roadmap, error) {
	if id, ok := m.activeByID[userID]; ok {
		return m.byID[id], nil
	}
	return nil, roadmap.ErrRoadmapNotFound
}
func (m *memRepo) ListByUser(context.Context, string, roadmap.RoadmapStatus, int, int) ([]*roadmap.Roadmap, error) {
	return nil, nil
}
func (m *memRepo) FindSessionByID(_ context.Context, sid string) (*roadmap.Roadmap, error) {
	for _, r := range m.byID {
		if _, ok := r.FindSession(sid); ok {
			return r, nil
		}
	}
	return nil, roadmap.ErrSessionNotFound
}

type captureOutbox struct {
	events []domainevent.Event
}

func (c *captureOutbox) Enqueue(_ context.Context, _ string, evs ...domainevent.Event) error {
	c.events = append(c.events, evs...)
	return nil
}

type stubProfile struct{ p port.Profile }

func (s *stubProfile) GetProfile(_ context.Context, _ string) (port.Profile, error) { return s.p, nil }

type stubWorkouts struct{}

func (stubWorkouts) GetRecentSessions(context.Context, string, time.Time) ([]port.WorkoutSession, error) {
	return nil, nil
}
func (stubWorkouts) GetSetLogs(context.Context, string, string, int) ([]port.SetLog, error) {
	return nil, nil
}

func buildHandler(t *testing.T) (*InitiateRoadmapHandler, *memRepo, *captureOutbox) {
	t.Helper()
	clock := &fakeClock{t: time.Date(2026, 7, 28, 12, 0, 0, 0, time.UTC)}
	ids := &incrIDs{}
	agent := ai.NewMockCoachAgent(ids, clock)
	builder := contextbuilder.NewBuilder(
		&stubProfile{p: port.Profile{
			UserID:                "user-1",
			PrimaryGoal:           "MUSCLE_GAIN",
			PreferredMuscleGroups: []string{"chest", "back"},
		}},
		stubWorkouts{},
		contextbuilder.NewStaticPromptRegistry(),
	)
	guard := guardrail.NewEngine(service.NewOverloadValidator(), nil, nil)
	repo := newMemRepo()
	outbox := &captureOutbox{}
	h := NewInitiateRoadmapHandler(fakeTx{}, repo, agent, builder, guard, outbox, clock)
	return h, repo, outbox
}

func TestInitiateRoadmap_HappyPath(t *testing.T) {
	h, repo, outbox := buildHandler(t)
	res, err := h.Handle(context.Background(), InitiateRoadmapCommand{UserID: "user-1"})
	if err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if res == nil || res.Roadmap == nil {
		t.Fatalf("nil result")
	}
	if res.Roadmap.Status() != roadmap.RoadmapStatusActive {
		t.Errorf("status=%s", res.Roadmap.Status())
	}
	// Roadmap must be persisted.
	if _, err := repo.FindActiveByUser(context.Background(), "user-1"); err != nil {
		t.Errorf("expected active roadmap in repo: %v", err)
	}
	// Outbox must have RoadmapInitiated.
	if len(outbox.events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(outbox.events))
	}
	if _, ok := outbox.events[0].(*domainevent.RoadmapInitiated); !ok {
		t.Errorf("expected RoadmapInitiated, got %T", outbox.events[0])
	}
}

func TestInitiateRoadmap_RejectsDuplicateActive(t *testing.T) {
	h, _, _ := buildHandler(t)
	if _, err := h.Handle(context.Background(), InitiateRoadmapCommand{UserID: "user-1"}); err != nil {
		t.Fatalf("first init: %v", err)
	}
	_, err := h.Handle(context.Background(), InitiateRoadmapCommand{UserID: "user-1"})
	if !errors.Is(err, roadmap.ErrActiveRoadmapExists) {
		t.Errorf("expected ErrActiveRoadmapExists, got %v", err)
	}
}

func TestInitiateRoadmap_MissingUserID(t *testing.T) {
	h, _, _ := buildHandler(t)
	_, err := h.Handle(context.Background(), InitiateRoadmapCommand{UserID: ""})
	if err == nil {
		t.Errorf("expected error for missing user_id")
	}
}
