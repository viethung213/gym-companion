package query

import (
	"context"
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/coaching/application/port"
	"github.com/viethung213/gym-companion/internal/coaching/domain/roadmap"
)

type fakeClock struct{ t time.Time }

func (c *fakeClock) Now() time.Time { return c.t }

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

type fakeCoachAgent struct{}

func (f *fakeCoachAgent) GenerateRoadmap(_ context.Context, _ string) (*roadmap.Roadmap, error) {
	return nil, nil
}

func (f *fakeCoachAgent) RegeneratePending(_ context.Context, _ string, _ []string) ([]*roadmap.SessionPlanInfo, error) {
	return nil, nil
}

func (f *fakeCoachAgent) Adapt(_ context.Context, _ string, _ string) ([]*roadmap.SessionPlanInfo, error) {
	return nil, nil
}

func (f *fakeCoachAgent) SuggestAdHocSession(_ context.Context, _ string, hint port.AdHocHint) (port.SuggestedSession, error) {
	rpe := float32(7.0)
	if hint.IntensityHint == "light" {
		rpe = 6.0
	} else if hint.IntensityHint == "hard" {
		rpe = 8.0
	}

	muscleGroups := []string{"back"}
	if len(hint.MuscleGroups) > 0 {
		muscleGroups = hint.MuscleGroups
	}

	sets := int32(3)
	if hint.DurationMinutes == 15 {
		sets = 1
	}

	return port.SuggestedSession{
		MuscleGroups: muscleGroups,
		Prescription: roadmap.WorkoutPrescription{
			MainExercises: []roadmap.PrescribedExercise{
				{
					ExerciseID:   "ex-1",
					ExerciseName: "Pullup",
					TargetSets:   sets,
					TargetReps:   8,
					TargetWeight: 0,
					TargetRPE:    rpe,
				},
			},
		},
		Reasoning:    "Based on constraints",
		EstimatedRPE: rpe,
	}, nil
}

func buildSuggestHandler(t *testing.T) *SuggestAdHocSessionHandler {
	t.Helper()

	clock := &fakeClock{t: time.Date(2026, 7, 28, 8, 0, 0, 0, time.UTC)}
	mockAgent := &fakeCoachAgent{}

	return NewSuggestAdHocSessionHandler(&memRepo{}, mockAgent, clock)
}

func TestSuggestAdHocSession_HappyPath_NoActiveRoadmap(t *testing.T) {
	h := buildSuggestHandler(t)

	got, err := h.Handle(context.Background(), &SuggestAdHocSessionQuery{
		UserID: "u-1",
		Hint:   port.AdHocHint{FreeText: "quick back session"},
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
		Hint:   port.AdHocHint{MuscleGroups: []string{"legs"}},
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

	got, err := h.Handle(context.Background(), &SuggestAdHocSessionQuery{
		UserID: "u-1",
		Hint:   port.AdHocHint{DurationMinutes: 15},
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
		UserID: "u-1", Hint: port.AdHocHint{IntensityHint: "light"},
	})

	hard, _ := h.Handle(context.Background(), &SuggestAdHocSessionQuery{
		UserID: "u-1", Hint: port.AdHocHint{IntensityHint: "hard"},
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
}
