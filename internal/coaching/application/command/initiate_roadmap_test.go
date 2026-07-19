package command_test

import (
	"context"
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/coaching/application/command"
	"github.com/viethung213/gym-companion/internal/coaching/application/port"
	"github.com/viethung213/gym-companion/internal/coaching/domain"
)

// Mock implementations for application ports

type mockRoadmapRepo struct {
	activeRoadmap *domain.WorkoutRoadmap
	savedRoadmap  *domain.WorkoutRoadmap
	savedEvent    *domain.Event
}

func (m *mockRoadmapRepo) Save(_ context.Context, r *domain.WorkoutRoadmap, e *domain.Event) error {
	m.savedRoadmap = r
	m.savedEvent = e
	return nil
}

func (m *mockRoadmapRepo) FindActiveByUserID(_ context.Context, _ string) (*domain.WorkoutRoadmap, error) {
	return m.activeRoadmap, nil
}

type mockScheduleRepo struct {
	savedSchedule *domain.WeeklySchedule
	savedEvent    *domain.Event
}

func (m *mockScheduleRepo) Save(_ context.Context, s *domain.WeeklySchedule, e *domain.Event) error {
	m.savedSchedule = s
	m.savedEvent = e
	return nil
}

type mockPlanRepo struct {
	savedPlans []*domain.DailyWorkoutPlan
	savedEvents []*domain.Event
}

func (m *mockPlanRepo) SaveBatch(_ context.Context, plans []*domain.DailyWorkoutPlan, events []*domain.Event) error {
	m.savedPlans = plans
	m.savedEvents = events
	return nil
}

type mockExerciseSvc struct{}

func (m mockExerciseSvc) SearchExercises(_ context.Context, _ port.ExerciseSearchFilters) ([]port.ExerciseInfo, error) {
	return []port.ExerciseInfo{
		{ID: "ex-1", Name: "Barbell Bench Press", Category: "Compound", PrimaryMuscle: "Chest"},
		{ID: "ex-2", Name: "Incline Dumbbell Press", Category: "Compound", PrimaryMuscle: "Chest"},
	}, nil
}

type mockPlanner struct{}

func (m mockPlanner) PlanWorkout(_ context.Context, req port.PlanWorkoutRequest) (*port.PlanWorkoutResult, error) {
	return &port.PlanWorkoutResult{
		SelectedExerciseIDs:  []string{"ex-1", "ex-2"},
		ReasoningExplanation: "Mock test arrangement",
	}, nil
}

type fixedClock struct{ t time.Time }

func (c fixedClock) Now() time.Time { return c.t }

type testIDGen struct{ counter int }

func (g *testIDGen) NewID() (string, error) {
	g.counter++
	return "test-id", nil
}

func TestInitiateRoadmapHandler_Handle(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 7, 20, 10, 0, 0, 0, time.UTC)
	ctx := context.Background()

	t.Run("successfully initiates roadmap when user has no active roadmap", func(t *testing.T) {
		t.Parallel()

		roadmapRepo := &mockRoadmapRepo{}
		scheduleRepo := &mockScheduleRepo{}
		planRepo := &mockPlanRepo{}
		exerciseSvc := mockExerciseSvc{}
		planner := mockPlanner{}
		clock := fixedClock{t: now}
		ids := &testIDGen{}

		handler := command.NewInitiateRoadmapHandler(
			roadmapRepo, scheduleRepo, planRepo,
			exerciseSvc, planner, clock, ids,
		)

		cmd := &command.InitiateRoadmapCommand{
			UserID:             "user-100",
			Goals:              []string{"muscle_gain"},
			RegisteredInjuries: []string{"knee"},
			ExperienceLevel:    "beginner",
		}

		if err := handler.Handle(ctx, cmd); err != nil {
			t.Fatalf("handle failed: %v", err)
		}

		if roadmapRepo.savedRoadmap == nil {
			t.Fatalf("expected saved roadmap")
		}
		if got, want := roadmapRepo.savedRoadmap.UserID(), "user-100"; got != want {
			t.Errorf("UserID got = %s, want = %s", got, want)
		}
		if got, want := roadmapRepo.savedRoadmap.Status(), domain.RoadmapStatusActive; got != want {
			t.Errorf("Status got = %s, want = %s", got, want)
		}

		if scheduleRepo.savedSchedule == nil {
			t.Fatalf("expected saved schedule")
		}
		if got, want := scheduleRepo.savedSchedule.WeekNumber(), 1; got != want {
			t.Errorf("WeekNumber got = %d, want = %d", got, want)
		}

		// Beginner gets 3 training days -> 3 daily plans
		if got, want := len(planRepo.savedPlans), 3; got != want {
			t.Errorf("Daily plans count got = %d, want = %d", got, want)
		}
	})

	t.Run("skips initiation if user already has an active roadmap (idempotent)", func(t *testing.T) {
		t.Parallel()

		existingRoadmap, _ := domain.Initiate("existing-r", "user-100", now, now)
		roadmapRepo := &mockRoadmapRepo{activeRoadmap: existingRoadmap}
		scheduleRepo := &mockScheduleRepo{}
		planRepo := &mockPlanRepo{}

		handler := command.NewInitiateRoadmapHandler(
			roadmapRepo, scheduleRepo, planRepo,
			mockExerciseSvc{}, mockPlanner{}, fixedClock{t: now}, &testIDGen{},
		)

		cmd := &command.InitiateRoadmapCommand{
			UserID:          "user-100",
			ExperienceLevel: "beginner",
		}

		if err := handler.Handle(ctx, cmd); err != nil {
			t.Fatalf("handle failed: %v", err)
		}

		if roadmapRepo.savedRoadmap != nil {
			t.Errorf("expected no roadmap saved since active roadmap already exists")
		}
	})
}
