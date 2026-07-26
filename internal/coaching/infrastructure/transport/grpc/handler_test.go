package grpc_test

import (
	"context"
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/coaching/application/command"
	"github.com/viethung213/gym-companion/internal/coaching/application/port"
	"github.com/viethung213/gym-companion/internal/coaching/domain"
	coachinggrpc "github.com/viethung213/gym-companion/internal/coaching/infrastructure/transport/grpc"
	coachingv1message "github.com/viethung213/gym-companion/internal/gen/go/contracts/core/coaching/v1/message"
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

type mockDailyPlanRepo struct {
	saved *domain.DailyWorkoutPlan
}

func (m *mockDailyPlanRepo) Save(ctx context.Context, p *domain.DailyWorkoutPlan) error {
	m.saved = p
	return nil
}
func (m *mockDailyPlanRepo) FindByID(ctx context.Context, id string) (*domain.DailyWorkoutPlan, error) {
	return m.saved, nil
}
func (m *mockDailyPlanRepo) FindByUserAndDate(ctx context.Context, userID string, date time.Time) (*domain.DailyWorkoutPlan, error) {
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
			st = domain.WorkoutDayStatusRest
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
		ReasoningExplanation: "Reasoning",
	}, nil
}

type mockExerciseProvider struct{}

func (m *mockExerciseProvider) GetAvailableExerciseIDs(ctx context.Context, equipment []string, activeInjuries []string) ([]string, error) {
	return []string{"ex_1"}, nil
}
func (m *mockExerciseProvider) GetBaseline1RM(ctx context.Context, userID string) (map[string]float32, error) {
	return map[string]float32{"ex_1": 100.0}, nil
}

func TestCoachingGRPCHandler_InitiateRoadmap(t *testing.T) {
	t.Parallel()
	rRepo := &mockRoadmapRepo{}
	sRepo := &mockScheduleRepo{}
	dRepo := &mockDailyPlanRepo{}
	agent := &mockCoachingAgent{}
	exProvider := &mockExerciseProvider{}
	validator := domain.NewUpperSafetyEnvelopeValidator()

	initiateUC := command.NewInitiateRoadmapHandler(rRepo, sRepo, agent, nil, validator)
	genDailyUC := command.NewGenerateDailyPlanHandler(rRepo, sRepo, dRepo, agent, exProvider, nil, validator)
	processPostUC := command.NewProcessPostWorkoutHandler(dRepo, sRepo, agent, exProvider, validator)

	handler := coachinggrpc.NewCoachingGRPCHandler(initiateUC, genDailyUC, processPostUC, rRepo, sRepo, dRepo)

	req := &coachingv1message.InitiateRoadmapRequest{
		UserId: "usr_100",
		Payload: &coachingv1message.InitiateRoadmapPayload{
			ProfileSnapshotId: "snap_1",
		},
	}

	resp, err := handler.InitiateRoadmap(context.Background(), req)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if resp.Roadmap == nil || resp.FirstWeeklySchedule == nil {
		t.Fatalf("expected non-nil response fields")
	}

	if resp.Roadmap.UserId != "usr_100" {
		t.Errorf("expected user_id usr_100, got %s", resp.Roadmap.UserId)
	}
}
