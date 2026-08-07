package command

import (
	"context"
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/nutrition/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/nutrition/domain/vo"
)

type mockPlanRepoForUpdateSchedule struct {
	plan *aggregate.NutritionPlan
}

func (m *mockPlanRepoForUpdateSchedule) FindByUserIDAndDate(ctx context.Context, userID string, date time.Time) (*aggregate.NutritionPlan, error) {
	return m.plan, nil
}
func (m *mockPlanRepoForUpdateSchedule) Save(ctx context.Context, plan *aggregate.NutritionPlan) error {
	m.plan = plan
	return nil
}
func (m *mockPlanRepoForUpdateSchedule) Update(ctx context.Context, plan *aggregate.NutritionPlan) error {
	m.plan = plan
	return nil
}
func (m *mockPlanRepoForUpdateSchedule) FindActiveUserIDs(ctx context.Context, withinDays int) ([]string, error) {
	return nil, nil
}
func (m *mockPlanRepoForUpdateSchedule) FindPlansForDate(ctx context.Context, targetDate time.Time) ([]*aggregate.NutritionPlan, error) {
	return []*aggregate.NutritionPlan{m.plan}, nil
}
func (m *mockPlanRepoForUpdateSchedule) GetUserMealSchedules(ctx context.Context, userID string) (map[string]string, error) {
	return map[string]string{"BREAKFAST": "08:30"}, nil
}
func (m *mockPlanRepoForUpdateSchedule) SaveUserMealSchedules(ctx context.Context, userID string, schedules map[string]string) error {
	return nil
}

func TestUpdateMealScheduleHandler_Success(t *testing.T) {
	now := time.Now()
	alloc, _ := vo.NewCalorieAllocation(2000, 150, 200, 60)
	mealOpt := aggregate.NewMealOption("opt_1", "Phở Bò", 500, 30, 60, 15, nil, nil, false)
	dailyMeal := aggregate.NewDailyMealWithSchedule("BREAKFAST", []aggregate.MealOption{mealOpt}, "07:00")

	plan := aggregate.NewNutritionPlan("plan_001", "usr_100", now, alloc, []aggregate.DailyMeal{dailyMeal})
	repo := &mockPlanRepoForUpdateSchedule{plan: plan}

	hdlr := NewUpdateMealScheduleHandler(repo)

	res, err := hdlr.Handle(context.Background(), UpdateMealScheduleCommand{
		UserID: "usr_100",
		Schedules: []MealScheduleItemInput{
			{MealType: "BREAKFAST", ScheduledTime: "08:30"},
		},
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !res.Success {
		t.Fatalf("expected success true, got false")
	}

	if len(res.Schedules) != 1 {
		t.Fatalf("schedules count got %d, want 1", len(res.Schedules))
	}

	if got, want := res.Schedules[0].ScheduledTime, "08:30"; got != want {
		t.Errorf("ScheduledTime got %s, want %s", got, want)
	}
}
