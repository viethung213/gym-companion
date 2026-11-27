package consumer

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	segmentio "github.com/segmentio/kafka-go"
	"github.com/viethung213/gym-companion/internal/nutrition/application/command"
	"github.com/viethung213/gym-companion/internal/nutrition/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/nutrition/domain/repository"
)

type mockWorkoutPlanRepo struct {
	calledUserID string
}

var _ repository.NutritionPlanRepository = (*mockWorkoutPlanRepo)(nil)

func (m *mockWorkoutPlanRepo) FindByUserIDAndDate(ctx context.Context, userID string, date time.Time) (*aggregate.NutritionPlan, error) {
	m.calledUserID = userID
	return nil, errors.New("plan not found")
}

func (m *mockWorkoutPlanRepo) FindByUserIDAndDates(_ context.Context, _ string, _ []time.Time) ([]*aggregate.NutritionPlan, error) {
	return nil, nil
}

func (m *mockWorkoutPlanRepo) Save(ctx context.Context, plan *aggregate.NutritionPlan) error {
	return nil
}

func (m *mockWorkoutPlanRepo) Update(ctx context.Context, plan *aggregate.NutritionPlan) error {
	return nil
}

func (m *mockWorkoutPlanRepo) FindActiveUserIDs(ctx context.Context, withinDays int) ([]string, error) {
	return nil, nil
}

func (m *mockWorkoutPlanRepo) FindPlansForDate(ctx context.Context, targetDate time.Time) ([]*aggregate.NutritionPlan, error) {
	return nil, nil
}

func (m *mockWorkoutPlanRepo) GetUserMealSchedules(ctx context.Context, userID string) (map[string]string, error) {
	return nil, nil
}

func (m *mockWorkoutPlanRepo) SaveUserMealSchedules(ctx context.Context, userID string, schedules map[string]string) error {
	return nil
}

func TestWorkoutEventConsumer_HandleMessage(t *testing.T) {
	t.Run("CloudEvent envelope with WorkoutSessionCompleted payload", func(t *testing.T) {
		mockRepo := &mockWorkoutPlanRepo{}
		rebalHandler := command.NewRebalanceWorkoutNutritionHandler(mockRepo, nil)
		c := NewWorkoutEventConsumer(nil, rebalHandler, nil)

		innerPayload := map[string]any{
			"sessionId":   "sess-101",
			"userId":      "usr-202",
			"totalVolume": 1250.0,
			"completedAt": "2026-08-09T00:00:00Z",
		}
		dataBytes, _ := json.Marshal(innerPayload)

		ce := map[string]any{
			"specversion": "1.0",
			"id":          "evt-123",
			"source":      "services/workout-execution-service",
			"type":        "contracts.core.workout_execution.v1.workoutSessionCompleted",
			"data":        json.RawMessage(dataBytes),
		}
		ceBytes, _ := json.Marshal(ce)

		_ = c.HandleMessage(context.Background(), segmentio.Message{
			Value: ceBytes,
		})

		if got, want := mockRepo.calledUserID, "usr-202"; got != want {
			t.Errorf("expected recalibrate called for user %s, got %s", want, got)
		}
	})

	t.Run("Non-workout event is ignored", func(t *testing.T) {
		mockRepo := &mockWorkoutPlanRepo{}
		rebalHandler := command.NewRebalanceWorkoutNutritionHandler(mockRepo, nil)
		c := NewWorkoutEventConsumer(nil, rebalHandler, nil)

		ce := map[string]any{
			"specversion": "1.0",
			"id":          "evt-456",
			"type":        "contracts.core.nutrition.v1.mealLogged",
			"data":        json.RawMessage(`{"userId":"usr-303"}`),
		}
		ceBytes, _ := json.Marshal(ce)

		_ = c.HandleMessage(context.Background(), segmentio.Message{
			Value: ceBytes,
		})

		if mockRepo.calledUserID != "" {
			t.Errorf("expected no recalibration, but called for %s", mockRepo.calledUserID)
		}
	})
}
