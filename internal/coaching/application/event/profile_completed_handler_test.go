package event_test

import (
	"context"
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/coaching/application/command"
	appEvent "github.com/viethung213/gym-companion/internal/coaching/application/event"
	"github.com/viethung213/gym-companion/internal/coaching/application/port"
	"github.com/viethung213/gym-companion/internal/coaching/domain"
)

type mockRoadmapRepo struct {
	savedRoadmap *domain.WorkoutRoadmap
}

func (m *mockRoadmapRepo) Save(_ context.Context, r *domain.WorkoutRoadmap, _ *domain.Event) error {
	m.savedRoadmap = r
	return nil
}

func (m *mockRoadmapRepo) FindActiveByUserID(_ context.Context, _ string) (*domain.WorkoutRoadmap, error) {
	return nil, nil
}

type mockScheduleRepo struct{}

func (m *mockScheduleRepo) Save(_ context.Context, _ *domain.WeeklySchedule, _ *domain.Event) error {
	return nil
}

type mockPlanRepo struct{}

func (m *mockPlanRepo) SaveBatch(_ context.Context, _ []*domain.DailyWorkoutPlan, _ []*domain.Event) error {
	return nil
}

type mockExerciseSvc struct{}

func (m mockExerciseSvc) SearchExercises(_ context.Context, _ port.ExerciseSearchFilters) ([]port.ExerciseInfo, error) {
	return []port.ExerciseInfo{
		{ID: "ex-1", Name: "Pushup", Category: "Compound", PrimaryMuscle: "Chest"},
	}, nil
}

type mockPlanner struct{}

func (m mockPlanner) PlanWorkout(_ context.Context, _ port.PlanWorkoutRequest) (*port.PlanWorkoutResult, error) {
	return &port.PlanWorkoutResult{
		SelectedExerciseIDs:  []string{"ex-1"},
		ReasoningExplanation: "Event handler test mock",
	}, nil
}

type fixedClock struct{ t time.Time }

func (c fixedClock) Now() time.Time { return c.t }

type testIDGen struct{}

func (g *testIDGen) NewID() (string, error) { return "test-evt-id", nil }

func TestProfileCompletedHandler_Handle(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 7, 20, 10, 0, 0, 0, time.UTC)
	ctx := context.Background()

	roadmapRepo := &mockRoadmapRepo{}
	initiateHandler := command.NewInitiateRoadmapHandler(
		roadmapRepo, &mockScheduleRepo{}, &mockPlanRepo{},
		mockExerciseSvc{}, mockPlanner{}, fixedClock{t: now}, &testIDGen{},
	)

	handler := appEvent.NewProfileCompletedHandler(initiateHandler)

	evt := appEvent.ProfileCompletedEvent{
		UserID:             "user-evt-1",
		Goals:              []string{"fat_loss"},
		RegisteredInjuries: []string{"shoulder"},
		ExperienceLevel:    "intermediate",
	}

	if err := handler.Handle(ctx, evt); err != nil {
		t.Fatalf("handle profile completed event failed: %v", err)
	}

	if roadmapRepo.savedRoadmap == nil {
		t.Fatalf("expected roadmap saved from event dispatch")
	}
	if got, want := roadmapRepo.savedRoadmap.UserID(), "user-evt-1"; got != want {
		t.Errorf("UserID got = %s, want = %s", got, want)
	}
}
