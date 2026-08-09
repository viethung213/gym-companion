package kafka

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

type mockPlanRepo struct {
	calledUserID string
}

var _ repository.NutritionPlanRepository = (*mockPlanRepo)(nil)

func (m *mockPlanRepo) FindByUserIDAndDate(ctx context.Context, userID string, date time.Time) (*aggregate.NutritionPlan, error) {
	m.calledUserID = userID
	return nil, errors.New("plan not found")
}

func (m *mockPlanRepo) Save(ctx context.Context, plan *aggregate.NutritionPlan) error {
	return nil
}

func (m *mockPlanRepo) Update(ctx context.Context, plan *aggregate.NutritionPlan) error {
	return nil
}

func (m *mockPlanRepo) FindActiveUserIDs(ctx context.Context, withinDays int) ([]string, error) {
	return nil, nil
}

func (m *mockPlanRepo) FindPlansForDate(ctx context.Context, targetDate time.Time) ([]*aggregate.NutritionPlan, error) {
	return nil, nil
}

func (m *mockPlanRepo) GetUserMealSchedules(ctx context.Context, userID string) (map[string]string, error) {
	return nil, nil
}

func (m *mockPlanRepo) SaveUserMealSchedules(ctx context.Context, userID string, schedules map[string]string) error {
	return nil
}

func TestConsumer_HandleMessage(t *testing.T) {
	t.Run("CloudEvent envelope with WorkoutSessionCompleted payload", func(t *testing.T) {
		mockRepo := &mockPlanRepo{}
		recalHandler := command.NewRecalibratePlanWithPantryHandler(mockRepo, nil, nil, nil)
		c := NewConsumer(nil, recalHandler)

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

		c.handleMessage(context.Background(), segmentio.Message{
			Value: ceBytes,
		})

		if got, want := mockRepo.calledUserID, "usr-202"; got != want {
			t.Errorf("expected recalibrate called for user %s, got %s", want, got)
		}
	})

	t.Run("Non-workout event is ignored", func(t *testing.T) {
		mockRepo := &mockPlanRepo{}
		recalHandler := command.NewRecalibratePlanWithPantryHandler(mockRepo, nil, nil, nil)
		c := NewConsumer(nil, recalHandler)

		ce := map[string]any{
			"specversion": "1.0",
			"id":          "evt-456",
			"type":        "contracts.core.nutrition.v1.mealLogged",
			"data":        json.RawMessage(`{"userId":"usr-303"}`),
		}
		ceBytes, _ := json.Marshal(ce)

		c.handleMessage(context.Background(), segmentio.Message{
			Value: ceBytes,
		})

		if mockRepo.calledUserID != "" {
			t.Errorf("expected no recalibration, but called for %s", mockRepo.calledUserID)
		}
	})
}
