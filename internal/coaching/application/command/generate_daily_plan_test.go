package command_test

import (
	"context"
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/coaching/application/command"
	"github.com/viethung213/gym-companion/internal/coaching/application/port"
	"github.com/viethung213/gym-companion/internal/coaching/domain"
)

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

type mockExerciseProvider struct{}

func (m *mockExerciseProvider) GetAvailableExerciseIDs(ctx context.Context, equipment []string, activeInjuries []string) ([]string, error) {
	return []string{"ex_1"}, nil
}

func (m *mockExerciseProvider) GetBaseline1RM(ctx context.Context, userID string) (map[string]float32, error) {
	return map[string]float32{
		"ex_1": 100.0,
	}, nil
}

func TestGenerateDailyPlanHandler(t *testing.T) {
	t.Parallel()
	now := time.Now().UTC()
	rRepo := &mockRoadmapRepo{}
	sRepo := &mockScheduleRepo{}
	dRepo := &mockDailyPlanRepo{}
	agent := &mockCoachingAgent{}
	exProvider := &mockExerciseProvider{}
	pub := &mockPublisher{}
	validator := domain.NewUpperSafetyEnvelopeValidator()

	// Seed roadmap & schedule
	r, _ := domain.NewWorkoutRoadmap("rdp_1", "usr_100", now, now.AddDate(0, 0, 28))
	rRepo.Save(context.Background(), r)

	days := make([]domain.ScheduleDay, 7)
	for i := 0; i < 7; i++ {
		st := domain.WorkoutDayStatusTraining
		if i == 6 {
			st = domain.WorkoutDayStatusRest
		}
		days[i] = domain.NewScheduleDay(now.AddDate(0, 0, i), "Day", st, []string{"Chest"}, "")
	}
	ws, _ := domain.NewWeeklySchedule("ws_1", "rdp_1", "usr_100", 1, now, now.AddDate(0, 0, 7), "PPL", days)
	sRepo.Save(context.Background(), ws)

	handler := command.NewGenerateDailyPlanHandler(rRepo, sRepo, dRepo, agent, exProvider, pub, validator)

	cmd := port.GenerateDailyPlanCommand{
		UserID:                  "usr_100",
		ScheduledDate:           now,
		CheckInAnswers:          nil,
		AnomalousSessionDetected: false,
		IsDeloadWeek:            false,
	}

	plan, err := handler.Execute(context.Background(), cmd)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if plan == nil {
		t.Fatalf("expected non-nil daily plan")
	}

	if plan.UserID() != "usr_100" {
		t.Errorf("expected user_id usr_100, got %s", plan.UserID())
	}

	// Verify BR-AC-02 Load Adjustment Ceiling (+30% of 100 = 130 max)
	mainEx := plan.Prescription().MainExercises()[0]
	if mainEx.TargetWeight() > 130.0 {
		t.Errorf("expected target weight <= 130.0, got %f", mainEx.TargetWeight())
	}
}
