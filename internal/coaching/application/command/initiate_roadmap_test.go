package command_test

import (
	"context"
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/coaching/application/command"
	"github.com/viethung213/gym-companion/internal/coaching/application/port"
	"github.com/viethung213/gym-companion/internal/coaching/domain"
)

type mockRoadmapRepo struct {
	saved *domain.WorkoutRoadmap
}

func (m *mockRoadmapRepo) Save(ctx context.Context, r *domain.WorkoutRoadmap) error {
	m.saved = r
	return nil
}
func (m *mockRoadmapRepo) FindByID(ctx context.Context, id string) (*domain.WorkoutRoadmap, error) {
	return m.saved, nil
}
func (m *mockRoadmapRepo) FindActiveByUserID(ctx context.Context, userID string) (*domain.WorkoutRoadmap, error) {
	return m.saved, nil
}

type mockScheduleRepo struct {
	saved *domain.WeeklySchedule
}

func (m *mockScheduleRepo) Save(ctx context.Context, s *domain.WeeklySchedule) error {
	m.saved = s
	return nil
}
func (m *mockScheduleRepo) FindByID(ctx context.Context, id string) (*domain.WeeklySchedule, error) {
	return m.saved, nil
}
func (m *mockScheduleRepo) FindCurrentByRoadmapID(ctx context.Context, roadmapID string, weekNumber int32) (*domain.WeeklySchedule, error) {
	return m.saved, nil
}

type mockCoachingAgent struct{}

func (m *mockCoachingAgent) GenerateRoadmapStrategy(ctx context.Context, params port.RoadmapStrategyParams) (*port.RoadmapStrategyOutput, error) {
	now := time.Now().UTC()
	return &port.RoadmapStrategyOutput{
		StartDate: now,
		EndDate:   now.AddDate(0, 0, 28),
	}, nil
}

func (m *mockCoachingAgent) GenerateWeeklySchedule(ctx context.Context, params port.WeeklyScheduleParams) (*port.WeeklyScheduleOutput, error) {
	now := time.Now().UTC()
	days := make([]domain.ScheduleDay, 7)
	for i := 0; i < 7; i++ {
		st := domain.WorkoutDayStatusTraining
		if i == 6 {
			st = domain.WorkoutDayStatusRest // Rest day
		}
		days[i] = domain.NewScheduleDay(now.AddDate(0, 0, i), "Day", st, []string{"Chest"}, "")
	}
	return &port.WeeklyScheduleOutput{
		MuscleSplitType: "Push/Pull/Legs",
		Days:            days,
	}, nil
}

func (m *mockCoachingAgent) GenerateDailyPrescription(ctx context.Context, params port.DailyPrescriptionParams) (*port.DailyPrescriptionOutput, error) {
	exs := []domain.PrescribedExercise{
		domain.NewPrescribedExercise("ex_1", "Bench", 3, 10, 60, 0, "", 90, 120, 7.5),
	}
	return &port.DailyPrescriptionOutput{
		Prescription:         domain.NewWorkoutPrescription(nil, exs, nil),
		ReasoningExplanation: "Good progress",
	}, nil
}

type mockPublisher struct{}

func (m *mockPublisher) PublishEvent(ctx context.Context, eventType string, aggregateID string, payload interface{}) error {
	return nil
}

func TestInitiateRoadmapHandler(t *testing.T) {
	t.Parallel()
	rRepo := &mockRoadmapRepo{}
	sRepo := &mockScheduleRepo{}
	agent := &mockCoachingAgent{}
	pub := &mockPublisher{}
	validator := domain.NewUpperSafetyEnvelopeValidator()

	handler := command.NewInitiateRoadmapHandler(rRepo, sRepo, agent, pub, validator)

	cmd := port.InitiateRoadmapCommand{
		UserID:             "usr_100",
		ProfileSnapshotID:  "snap_1",
		Goal:               "Hypertrophy",
		ExperienceLevel:    "Intermediate",
		AvailableSlots:     []string{"Mon", "Tue", "Wed", "Thu", "Fri", "Sat"},
		AvailableEquipment: []string{"Barbell", "Dumbbell"},
	}

	res, err := handler.Execute(context.Background(), cmd)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if res.Roadmap == nil || res.WeeklySchedule == nil {
		t.Fatalf("expected non-nil roadmap and schedule")
	}

	if res.Roadmap.UserID() != "usr_100" {
		t.Errorf("expected user_id usr_100, got %s", res.Roadmap.UserID())
	}
	if res.WeeklySchedule.WeekNumber() != 1 {
		t.Errorf("expected week number 1, got %d", res.WeeklySchedule.WeekNumber())
	}
}
