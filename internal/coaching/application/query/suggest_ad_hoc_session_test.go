package query

import (
	"context"
	"errors"
	"strconv"
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/coaching/agent"
	"github.com/viethung213/gym-companion/internal/coaching/agent/contextbuilder"
	"github.com/viethung213/gym-companion/internal/coaching/agent/llm/mock"
	"github.com/viethung213/gym-companion/internal/coaching/application/port"
	"github.com/viethung213/gym-companion/internal/coaching/domain/roadmap"
)

type fakeClock struct{ t time.Time }

func (c *fakeClock) Now() time.Time { return c.t }

type incrIDs struct{ n int }

func (i *incrIDs) NewID() string {
	i.n++

	return "id-" + strconv.Itoa(i.n)
}

// memRepo — minimal implementation of port.RoadmapRepository for tests.
type memRepo struct{ active *roadmap.Roadmap }

func (m *memRepo) Save(context.Context, *roadmap.Roadmap) error { return nil }
func (m *memRepo) FindByID(context.Context, string) (*roadmap.Roadmap, error) {
	return nil, roadmap.ErrRoadmapNotFound
}

func (m *memRepo) FindActiveByUser(context.Context, string) (*roadmap.Roadmap, error) {
	if m.active != nil {
		return m.active, nil
	}

	return nil, roadmap.ErrRoadmapNotFound
}

func (m *memRepo) ListByUser(context.Context, string, roadmap.Status, int, int) ([]*roadmap.Roadmap, error) {
	return nil, nil
}

func (m *memRepo) FindSessionByID(context.Context, string) (*roadmap.Roadmap, error) {
	return nil, roadmap.ErrSessionNotFound
}

type stubProfile struct{ p port.Profile }

func (s *stubProfile) GetProfile(context.Context, string) (port.Profile, error) { return s.p, nil }

type stubWorkouts struct{}

func (stubWorkouts) GetRecentSessions(context.Context, string, time.Time) ([]port.WorkoutSession, error) {
	return nil, nil
}

func (stubWorkouts) GetSetLogs(context.Context, string, string, int) ([]port.SetLog, error) {
	return nil, nil
}

func buildSuggestHandler(t *testing.T) *SuggestAdHocSessionHandler {
	t.Helper()

	clock := &fakeClock{t: time.Date(2026, 7, 28, 8, 0, 0, 0, time.UTC)}

	mockAgent := mock.NewCoachAgent(&incrIDs{}, clock)

	builder := contextbuilder.NewBuilder(

		&stubProfile{p: port.Profile{
			UserID:                "u-1",
			PrimaryGoal:           "MUSCLE_GAIN",
			PreferredMuscleGroups: []string{"back"},
			AvailableEquipment:    []string{"BARBELL"},
		}},

		stubWorkouts{},

		contextbuilder.NewStaticPromptRegistry(),
	)

	return NewSuggestAdHocSessionHandler(&memRepo{}, mockAgent, builder, clock)
}

func TestSuggestAdHocSession_HappyPath_NoActiveRoadmap(t *testing.T) {
	h := buildSuggestHandler(t)

	got, err := h.Handle(context.Background(), &SuggestAdHocSessionQuery{
		UserID: "u-1",

		Hint: agent.AdHocHint{FreeText: "quick back session"},
	})

	if err != nil {
		t.Fatalf("Handle: %v", err)
	}

	if len(got.Prescription.MainExercises) == 0 {
		t.Errorf("expected main exercises, got empty")
	}

	if len(got.MuscleGroups) == 0 {
		t.Errorf("expected muscle groups")
	}

	if got.EstimatedRPE == 0 {
		t.Errorf("expected non-zero EstimatedRPE")
	}
}

func TestSuggestAdHocSession_HintOverridesMuscleGroups(t *testing.T) {
	h := buildSuggestHandler(t)

	got, err := h.Handle(context.Background(), &SuggestAdHocSessionQuery{
		UserID: "u-1",

		Hint: agent.AdHocHint{MuscleGroups: []string{"legs"}},
	})

	if err != nil {
		t.Fatalf("Handle: %v", err)
	}

	if len(got.MuscleGroups) != 1 || got.MuscleGroups[0] != "legs" {
		t.Errorf("expected legs override, got %v", got.MuscleGroups)
	}
}

func TestSuggestAdHocSession_DurationCap_ShrinksSets(t *testing.T) {
	h := buildSuggestHandler(t)

	// 15 minutes - 10' overhead = 5' → floor(5/3) = 1 set max

	got, err := h.Handle(context.Background(), &SuggestAdHocSessionQuery{
		UserID: "u-1",

		Hint: agent.AdHocHint{DurationMinutes: 15},
	})

	if err != nil {
		t.Fatalf("Handle: %v", err)
	}

	totalSets := 0

	for _, ex := range got.Prescription.MainExercises {
		totalSets += int(ex.TargetSets)
	}

	if totalSets > 1 {
		t.Errorf("expected total sets <= 1 for 15min budget, got %d", totalSets)
	}
}

func TestSuggestAdHocSession_IntensityHintAffectsRPE(t *testing.T) {
	h := buildSuggestHandler(t)

	light, _ := h.Handle(context.Background(), &SuggestAdHocSessionQuery{
		UserID: "u-1", Hint: agent.AdHocHint{IntensityHint: "light"},
	})

	hard, _ := h.Handle(context.Background(), &SuggestAdHocSessionQuery{
		UserID: "u-1", Hint: agent.AdHocHint{IntensityHint: "hard"},
	})

	if light.EstimatedRPE >= hard.EstimatedRPE {
		t.Errorf("light RPE (%v) should be < hard RPE (%v)", light.EstimatedRPE, hard.EstimatedRPE)
	}
}

func TestSuggestAdHocSession_MissingUserID(t *testing.T) {
	h := buildSuggestHandler(t)

	_, err := h.Handle(context.Background(), &SuggestAdHocSessionQuery{})

	if err == nil {
		t.Errorf("expected error for missing user_id")
	}

	_ = errors.Is
}
