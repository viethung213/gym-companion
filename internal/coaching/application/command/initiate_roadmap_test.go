package command

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/coaching/application/port"
	domainevent "github.com/viethung213/gym-companion/internal/coaching/domain/event"
	"github.com/viethung213/gym-companion/internal/coaching/domain/guardrail"
	"github.com/viethung213/gym-companion/internal/coaching/domain/roadmap"
	"github.com/viethung213/gym-companion/internal/coaching/domain/service"
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

	if r.Status() == roadmap.StatusActive {
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

func (m *memRepo) ListByUser(_ context.Context, _ string, _ []roadmap.Status, _, _ int) ([]*roadmap.Roadmap, error) {
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

func (m *memRepo) FindPendingSessionsStarting(_ context.Context, _, _ time.Time) ([]port.ScheduledSession, error) {
	return nil, nil
}

type captureOutbox struct {
	events []domainevent.Event
}

func (c *captureOutbox) Enqueue(_ context.Context, _ string, events ...domainevent.Event) error {
	c.events = append(c.events, events...)
	return nil
}

type fakeCoachAgent struct {
	t time.Time
	// sets, when set, gives week N one training session whose single main
	// exercise carries sets[N] sets.
	sets map[int32]int32
}

func (f *fakeCoachAgent) GenerateRoadmap(_ context.Context, userID string) (*roadmap.Roadmap, error) {
	info := &roadmap.Info{
		RoadmapID: "roadmap-1",
		UserID:    userID,
		Status:    roadmap.StatusActive,
		StartDate: f.t,
		EndDate:   f.t.AddDate(0, 0, 28),
		CreatedAt: f.t,
		UpdatedAt: f.t,
	}
	var weeks []*roadmap.WeekPlan
	for i := 1; i <= 4; i++ {
		phase := roadmap.PhaseAccumulation
		if i == 2 {
			phase = roadmap.PhaseOverload
		} else if i == 3 {
			phase = roadmap.PhasePeak
		} else if i == 4 {
			phase = roadmap.PhaseDeload
		}
		weekInfo := &roadmap.WeekPlanInfo{
			WeekPlanID: fmt.Sprintf("week-%d", i),
			RoadmapID:  "roadmap-1",
			UserID:     userID,
			WeekNumber: int32(i),
			Phase:      phase,
			TargetRPE:  6.5,
			StartDate:  f.t.AddDate(0, 0, (i-1)*7),
			EndDate:    f.t.AddDate(0, 0, i*7),
		}
		wp, _ := roadmap.NewWeekPlan(weekInfo)
		if n, ok := f.sets[weekInfo.WeekNumber]; ok {
			f.addSession(wp, weekInfo, userID, n)
		}
		weeks = append(weeks, wp)
	}
	return roadmap.NewRoadmap(info, weeks, f.t)
}

// phaseRPE is a target inside each phase's window.
func phaseRPE(p roadmap.Phase) float32 {
	return map[roadmap.Phase]float32{
		roadmap.PhaseAccumulation: 6.5, roadmap.PhaseOverload: 7.5, roadmap.PhasePeak: 8.5, roadmap.PhaseDeload: 5.5,
	}[p]
}

func (f *fakeCoachAgent) addSession(wp *roadmap.WeekPlan, w *roadmap.WeekPlanInfo, userID string, sets int32) {
	date := w.StartDate.AddDate(0, 0, 1)
	dayID := w.WeekPlanID + "-day"
	dp, _ := roadmap.NewDayPlan(&roadmap.DayPlanInfo{
		DayPlanID: dayID, WeekPlanID: w.WeekPlanID, RoadmapID: w.RoadmapID, UserID: userID, ScheduledDate: date,
	})
	sp, _ := roadmap.NewSessionPlan(&roadmap.SessionPlanInfo{
		SessionPlanID: w.WeekPlanID + "-session", DayPlanID: dayID, WeekPlanID: w.WeekPlanID,
		RoadmapID: w.RoadmapID, UserID: userID, ScheduledDate: date, SlotTime: "17:30-19:00",
		Prescription: roadmap.WorkoutPrescription{MainExercises: []roadmap.PrescribedExercise{{
			ExerciseID: "squat", ExerciseName: "Squat", TargetSets: sets, TargetReps: 8, TargetRPE: phaseRPE(w.Phase),
		}}},
	}, f.t)
	_ = dp.AddSession(sp)
	_ = wp.AddDay(dp)
}

func (f *fakeCoachAgent) RegeneratePending(_ context.Context, _ string, _ []string) ([]*roadmap.SessionPlanInfo, error) {
	return nil, nil
}

func (f *fakeCoachAgent) Adapt(_ context.Context, _ string, _ *service.AdaptationDecision) ([]*roadmap.SessionPlanInfo, error) {
	return nil, nil
}

func (f *fakeCoachAgent) SuggestAdHocSession(_ context.Context, _ string, _ *port.AdHocHint) (port.SuggestedSession, error) {
	return port.SuggestedSession{}, nil
}

// fakeProfileReader reports a profile without injuries.
type fakeProfileReader struct{}

func (fakeProfileReader) GetProfile(_ context.Context, userID string) (port.Profile, error) {
	return port.Profile{UserID: userID}, nil
}

func buildHandler(t *testing.T) (*InitiateRoadmapHandler, *memRepo, *captureOutbox) {
	t.Helper()

	clock := &fakeClock{t: time.Date(2026, 7, 28, 12, 0, 0, 0, time.UTC)}
	mockAgent := &fakeCoachAgent{t: clock.t}
	guard := guardrail.NewEngine(service.NewOverloadValidator(), nil, nil)
	repo := newMemRepo()
	outbox := &captureOutbox{}

	h := NewInitiateRoadmapHandler(fakeTx{}, repo, mockAgent, guard, fakeProfileReader{}, outbox, clock)

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

	if res.Roadmap.Status() != roadmap.StatusActive {
		t.Errorf("status=%s", res.Roadmap.Status())
	}

	if _, err := repo.FindActiveByUser(context.Background(), "user-1"); err != nil {
		t.Errorf("expected active roadmap in repo: %v", err)
	}

	if len(outbox.events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(outbox.events))
	}

	if _, ok := outbox.events[0].(*domainevent.RoadmapInitiated); !ok {
		t.Errorf("expected RoadmapInitiated, got %T", outbox.events[0])
	}
}

// A DELOAD week drafted at PEAK volume is cut to BR-AC-11 by the domain and
// saved: the model is not asked again and the guardrail does not block it.
func TestInitiateRoadmap_FitsAHeavyDeloadWeek(t *testing.T) {
	h, repo, _ := buildHandler(t)
	h.agent = &fakeCoachAgent{t: h.clock.Now(), sets: map[int32]int32{1: 4, 2: 4, 3: 4, 4: 4}}

	if _, err := h.Handle(context.Background(), InitiateRoadmapCommand{UserID: "user-1"}); err != nil {
		t.Fatalf("Handle: %v", err)
	}
	saved, err := repo.FindActiveByUser(context.Background(), "user-1")
	if err != nil {
		t.Fatal(err)
	}
	if got, want := saved.WorkingSets(roadmap.PhaseDeload), 2; got != want {
		t.Errorf("DELOAD sets = %d, want %d (70%% of PEAK's 4)", got, want)
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
