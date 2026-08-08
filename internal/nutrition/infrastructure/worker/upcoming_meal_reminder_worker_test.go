package worker

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/nutrition/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/nutrition/domain/event"
	"github.com/viethung213/gym-companion/internal/nutrition/domain/vo"
)

type mockNutritionPlanRepo struct {
	plans []*aggregate.NutritionPlan
	err   error
}

func (m *mockNutritionPlanRepo) FindByUserIDAndDate(ctx context.Context, userID string, date time.Time) (*aggregate.NutritionPlan, error) {
	return nil, nil
}
func (m *mockNutritionPlanRepo) Save(ctx context.Context, plan *aggregate.NutritionPlan) error {
	return nil
}
func (m *mockNutritionPlanRepo) Update(ctx context.Context, plan *aggregate.NutritionPlan) error {
	return nil
}
func (m *mockNutritionPlanRepo) FindActiveUserIDs(ctx context.Context, withinDays int) ([]string, error) {
	return nil, nil
}
func (m *mockNutritionPlanRepo) FindPlansForDate(ctx context.Context, targetDate time.Time) ([]*aggregate.NutritionPlan, error) {
	return m.plans, m.err
}
func (m *mockNutritionPlanRepo) GetUserMealSchedules(ctx context.Context, userID string) (map[string]string, error) {
	return nil, nil
}
func (m *mockNutritionPlanRepo) SaveUserMealSchedules(ctx context.Context, userID string, schedules map[string]string) error {
	return nil
}

type mockEventPublisher struct {
	mu     sync.Mutex
	events []any
}

func (m *mockEventPublisher) PublishEvents(ctx context.Context, events []any) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.events = append(m.events, events...)
	return nil
}

func TestUpcomingMealReminderWorker_RunCheck(t *testing.T) {
	now := time.Now()
	// Set meal scheduled 30 minutes in the future
	futureMealTime := now.Add(30 * time.Minute)
	scheduledTimeStr := futureMealTime.Format("15:04")

	alloc, _ := vo.NewCalorieAllocation(2000, 150, 200, 60)
	mealOpt := aggregate.NewMealOption("opt_1", "Phở Bò", 500, 30, 60, 15, nil, nil, false)
	dailyMeal := aggregate.NewDailyMealWithSchedule("BREAKFAST", []aggregate.MealOption{mealOpt}, scheduledTimeStr)

	plan := aggregate.NewNutritionPlan("plan_001", "usr_100", futureMealTime, alloc, []aggregate.DailyMeal{dailyMeal})

	repo := &mockNutritionPlanRepo{plans: []*aggregate.NutritionPlan{plan}}
	publisher := &mockEventPublisher{}

	worker := NewUpcomingMealReminderWorker(repo, publisher, 100*time.Millisecond)
	worker.runCheck(context.Background())

	publisher.mu.Lock()
	count := len(publisher.events)
	publisher.mu.Unlock()

	if got, want := count, 1; got != want {
		t.Fatalf("events count got %d, want %d", got, want)
	}

	reminder, ok := publisher.events[0].(*event.UpcomingMealReminderEvent)
	if !ok {
		t.Fatalf("event type got %T, want *event.UpcomingMealReminderEvent", publisher.events[0])
	}

	if got, want := reminder.UserID(), "usr_100"; got != want {
		t.Errorf("UserID got %s, want %s", got, want)
	}
	if got, want := reminder.MealType(), "BREAKFAST"; got != want {
		t.Errorf("MealType got %s, want %s", got, want)
	}
}
