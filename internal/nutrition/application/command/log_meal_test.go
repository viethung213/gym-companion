package command_test

import (
	"context"
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/nutrition/application/command"
	"github.com/viethung213/gym-companion/internal/nutrition/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/nutrition/domain/vo"
)

func TestLogMealHandler_HandleManualAndPlanned(t *testing.T) {
	t.Parallel()

	planRepo := &mockPlanRepo{plans: make(map[string]*aggregate.NutritionPlan)}
	historyRepo := &mockHistoryRepo{}
	aiStub := &mockAIService{}

	alloc, _ := vo.NewCalorieAllocation(2000, 150, 200, 66)
	opt := aggregate.NewMealOption("opt-log-1", "Ức gà áp chảo", 450, 35, 20, 10, nil, nil, false)
	meal := aggregate.NewDailyMeal("Lunch", []aggregate.MealOption{opt})
	now := time.Now()
	plan := aggregate.NewNutritionPlan("plan-log-1", "user-log", now, alloc, []aggregate.DailyMeal{meal})
	_ = planRepo.Save(context.Background(), plan)

	handler := command.NewLogMealHandler(planRepo, historyRepo, nil, aiStub)

	// 1. Log planned option
	cmdPlanned := command.LogMealCommand{
		UserID:          "user-log",
		PlanDate:        now,
		PlannedOptionID: "opt-log-1",
		MealType:        "Lunch",
	}

	logged, err := handler.Handle(context.Background(), cmdPlanned)
	if err != nil {
		t.Fatalf("unexpected error logging planned meal: %v", err)
	}
	if got := logged.MealName(); got != "Ức gà áp chảo" {
		t.Fatalf("got meal name %q, want %q", got, "Ức gà áp chảo")
	}

	// 2. Log manual meal requiring AI estimation
	cmdManual := command.LogMealCommand{
		UserID:   "user-log",
		PlanDate: now,
		MealType: "Snack",
		MealName: "Bánh chuối nướng",
		Portion:  "1 cái",
	}

	loggedManual, err := handler.Handle(context.Background(), cmdManual)
	if err != nil {
		t.Fatalf("unexpected error logging manual meal: %v", err)
	}
	if got := loggedManual.Calories(); got != 450 {
		t.Fatalf("got calories %f, want 450", got)
	}
}
