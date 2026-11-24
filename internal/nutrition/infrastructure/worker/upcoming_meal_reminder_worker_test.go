package worker

import (
	"context"
	"sync"
	"testing"
	"time"
	_ "time/tzdata"

	"github.com/viethung213/gym-companion/internal/nutrition/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/nutrition/domain/event"
	"github.com/viethung213/gym-companion/internal/nutrition/domain/repository"
	"github.com/viethung213/gym-companion/internal/nutrition/domain/vo"
)

type mockNutritionPlanRepo struct {
	plans []*aggregate.NutritionPlan
	err   error
}

func (m *mockNutritionPlanRepo) FindByUserIDAndDate(ctx context.Context, userID string, date time.Time) (*aggregate.NutritionPlan, error) {
	return nil, nil
}
func (m *mockNutritionPlanRepo) FindByUserIDAndDates(ctx context.Context, userID string, dates []time.Time) ([]*aggregate.NutritionPlan, error) {
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

type mockEventRecorder struct {
	mu     sync.Mutex
	events []any
}

func (m *mockEventRecorder) RecordEvents(ctx context.Context, events []any) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.events = append(m.events, events...)
	return nil
}

func (m *mockEventRecorder) PublishEvents(ctx context.Context, events []any) error {
	return m.RecordEvents(ctx, events)
}

// zoneProfileClient reports every user in one zone and counts lookups.
type zoneProfileClient struct {
	zone  string
	calls int
}

func (z *zoneProfileClient) GetBiologicalMetrics(_ context.Context, _ string) (*repository.UserBiologicalMetrics, error) {
	z.calls++
	return &repository.UserBiologicalMetrics{Timezone: z.zone}, nil
}

// breakfastPlan is usr_100's plan for planDate with breakfast at clock.
func breakfastPlan(planDate time.Time, clock string) *aggregate.NutritionPlan {
	alloc, _ := vo.NewCalorieAllocation(2000, 150, 200, 60)
	mealOpt := aggregate.NewMealOption("opt_1", "Phở Bò", 500, 30, 60, 15, nil, nil, false)
	dailyMeal := aggregate.NewDailyMealWithSchedule("BREAKFAST", []aggregate.MealOption{mealOpt}, clock)
	return aggregate.NewNutritionPlan("plan_001", "usr_100", planDate, alloc, []aggregate.DailyMeal{dailyMeal})
}

func TestUpcomingMealReminderWorker_RemindsInUserZone(t *testing.T) {
	// Breakfast at 07:30 ICT on 2026-11-25 = 00:30 UTC; now is 00:00 UTC (07:00 ICT).
	now := time.Date(2026, 11, 25, 0, 0, 0, 0, time.UTC)
	repo := &mockNutritionPlanRepo{plans: []*aggregate.NutritionPlan{
		breakfastPlan(time.Date(2026, 11, 25, 0, 0, 0, 0, time.UTC), "07:30"),
	}}
	recorder := &mockEventRecorder{}
	profiles := &zoneProfileClient{zone: "Asia/Ho_Chi_Minh"}

	worker := NewUpcomingMealReminderWorker(repo, recorder, 100*time.Millisecond).WithProfileClient(profiles)
	worker.runCheckAt(context.Background(), now)
	worker.runCheckAt(context.Background(), now.Add(5*time.Minute))

	recorder.mu.Lock()
	defer recorder.mu.Unlock()
	if got, want := len(recorder.events), 1; got != want {
		t.Fatalf("events count got %d, want %d (one reminder, no duplicates)", got, want)
	}
	reminder, ok := recorder.events[0].(*event.UpcomingMealReminderEvent)
	if !ok {
		t.Fatalf("event type got %T, want *event.UpcomingMealReminderEvent", recorder.events[0])
	}
	if got, want := reminder.UserID(), "usr_100"; got != want {
		t.Errorf("UserID got %s, want %s", got, want)
	}
	if got, want := reminder.MealType(), "BREAKFAST"; got != want {
		t.Errorf("MealType got %s, want %s", got, want)
	}
	if profiles.calls != 1 {
		t.Errorf("profile looked up %d times, want 1 (zone is cached)", profiles.calls)
	}
}

func TestUpcomingMealReminderWorker_WithoutProfileReadsUTC(t *testing.T) {
	// Same plan, no profile client: 07:30 is read as UTC, seven hours away.
	now := time.Date(2026, 11, 25, 0, 0, 0, 0, time.UTC)
	repo := &mockNutritionPlanRepo{plans: []*aggregate.NutritionPlan{
		breakfastPlan(time.Date(2026, 11, 25, 0, 0, 0, 0, time.UTC), "07:30"),
	}}
	recorder := &mockEventRecorder{}

	NewUpcomingMealReminderWorker(repo, recorder, 100*time.Millisecond).runCheckAt(context.Background(), now)
	if len(recorder.events) != 0 {
		t.Fatalf("got %d reminders, want none 7 hours ahead of the meal", len(recorder.events))
	}
	NewUpcomingMealReminderWorker(repo, recorder, 100*time.Millisecond).runCheckAt(context.Background(), now.Add(7*time.Hour))
	if len(recorder.events) != 1 {
		t.Fatalf("got %d reminders, want 1 at 07:00 UTC", len(recorder.events))
	}
}
