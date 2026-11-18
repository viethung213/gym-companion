package command_test

import (
	"context"
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/nutrition/application/command"
	"github.com/viethung213/gym-companion/internal/nutrition/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/nutrition/domain/entity"
	"github.com/viethung213/gym-companion/internal/nutrition/domain/repository"
	"github.com/viethung213/gym-companion/internal/nutrition/domain/vo"
)

type mockWeeklyInsightRepo struct {
	saved *entity.WeeklyInsight
}

func (m *mockWeeklyInsightRepo) Save(_ context.Context, insight *entity.WeeklyInsight) error {
	m.saved = insight
	return nil
}

func (m *mockWeeklyInsightRepo) GetLatestByUserID(_ context.Context, _ string) (*entity.WeeklyInsight, error) {
	return m.saved, nil
}

func (m *mockWeeklyInsightRepo) GetByUserIDAndStartDate(_ context.Context, _ string, _ time.Time) (*entity.WeeklyInsight, error) {
	return m.saved, nil
}

type mockAIServiceForCommand struct {
	lastContext repository.InsightPromptContext
}

func (m *mockAIServiceForCommand) SelectCreativeMealOptions(_ context.Context, _ repository.AIMenuPromptContext) ([]repository.GeneratedRecipeResult, error) {
	return nil, nil
}

func (m *mockAIServiceForCommand) EstimateNutrient(_ context.Context, _, _ string) (*repository.EstimatedNutrientResult, error) {
	return nil, nil
}

func (m *mockAIServiceForCommand) GenerateNutritionInsight(_ context.Context, ctx repository.InsightPromptContext) (*repository.NutritionInsightResult, error) {
	m.lastContext = ctx
	return &repository.NutritionInsightResult{
		OverallScore:          80,
		Summary:               "Tiến bộ tốt trong tuần",
		PredictedCookingStyle: "Chiên xào ít dầu",
		PreferredIngredients:  []string{"Thịt bò", "Trứng"},
		ImprovementAreas: []repository.ImprovementArea{
			{
				Nutrient:    "CALORIES",
				Action:      "DECREASE",
				DeltaAmount: 300,
				Suggestion:  "Giảm một chút tinh bột",
			},
		},
	}, nil
}

type mockPlanRepoForCmd struct {
	plan *aggregate.NutritionPlan
}

func (m *mockPlanRepoForCmd) FindByUserIDAndDate(_ context.Context, _ string, _ time.Time) (*aggregate.NutritionPlan, error) {
	return m.plan, nil
}
func (m *mockPlanRepoForCmd) FindByUserIDAndDates(_ context.Context, _ string, _ []time.Time) ([]*aggregate.NutritionPlan, error) {
	if m.plan != nil {
		return []*aggregate.NutritionPlan{m.plan}, nil
	}
	return nil, nil
}
func (m *mockPlanRepoForCmd) Save(_ context.Context, _ *aggregate.NutritionPlan) error   { return nil }
func (m *mockPlanRepoForCmd) Update(_ context.Context, _ *aggregate.NutritionPlan) error { return nil }
func (m *mockPlanRepoForCmd) FindActiveUserIDs(_ context.Context, _ int) ([]string, error) {
	return nil, nil
}
func (m *mockPlanRepoForCmd) FindAllUserIDs(_ context.Context) ([]string, error) {
	return nil, nil
}
func (m *mockPlanRepoForCmd) FindPlansForDate(_ context.Context, _ time.Time) ([]*aggregate.NutritionPlan, error) {
	return nil, nil
}
func (m *mockPlanRepoForCmd) GetUserMealSchedules(_ context.Context, _ string) (map[string]string, error) {
	return nil, nil
}
func (m *mockPlanRepoForCmd) SaveUserMealSchedules(_ context.Context, _ string, _ map[string]string) error {
	return nil
}

type mockHistoryRepoForCmd struct {
	history *aggregate.MealHistory
}

func (m *mockHistoryRepoForCmd) FindByUserID(_ context.Context, _ string) (*aggregate.MealHistory, error) {
	return m.history, nil
}
func (m *mockHistoryRepoForCmd) FindByUserIDAndDateRange(_ context.Context, _ string, _, _ time.Time) (*aggregate.MealHistory, error) {
	return m.history, nil
}
func (m *mockHistoryRepoForCmd) Save(_ context.Context, _ *aggregate.MealHistory) error { return nil }

func TestGenerateWeeklyInsightHandler(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	now := time.Now()

	alloc, _ := vo.NewCalorieAllocation(2000, 150, 200, 66)
	plan := aggregate.NewNutritionPlan("p-1", "user-cmd-1", now, alloc, nil)
	planRepo := &mockPlanRepoForCmd{plan: plan}

	history := aggregate.NewMealHistory("h-1", "user-cmd-1")
	log1 := aggregate.NewMealLog("l-1", "h-1", "user-cmd-1", "Breakfast", "Phở bò", "1 bát", 500, 25, 60, 15, now)
	log2 := aggregate.NewMealLog("l-2", "h-1", "user-cmd-1", "Lunch", "Cơm tấm", "1 đĩa", 700, 35, 80, 20, now)
	history.AddMealLog(log1)
	history.AddMealLog(log2)
	historyRepo := &mockHistoryRepoForCmd{history: history}

	weeklyRepo := &mockWeeklyInsightRepo{}
	aiMock := &mockAIServiceForCommand{}

	handler := command.NewGenerateWeeklyInsightHandler(planRepo, historyRepo, weeklyRepo, aiMock)

	// 1. Validation error: empty user_id
	_, err := handler.Handle(ctx, command.GenerateWeeklyInsightCommand{UserID: ""})
	if err == nil {
		t.Fatal("expected error for empty user_id, got nil")
	}

	// 2. Happy path
	result, err := handler.Handle(ctx, command.GenerateWeeklyInsightCommand{
		UserID:   "user-cmd-1",
		GoalType: "WEIGHT_LOSS",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.OverallScore() != 80 {
		t.Errorf("got score %d, want 80", result.OverallScore())
	}
	if result.PredictedCookingStyle() != "Chiên xào ít dầu" {
		t.Errorf("got cooking style %s, want Chiên xào ít dầu", result.PredictedCookingStyle())
	}
	if len(result.ImprovementAreas()) != 1 {
		t.Fatalf("got %d improvement areas, want 1", len(result.ImprovementAreas()))
	}
	if weeklyRepo.saved == nil {
		t.Fatal("expected insight to be saved to repository")
	}
	if weeklyRepo.saved.UserID() != "user-cmd-1" {
		t.Errorf("got saved user ID %s, want user-cmd-1", weeklyRepo.saved.UserID())
	}
	if aiMock.lastContext.GoalType != "WEIGHT_LOSS" {
		t.Errorf("got goal %s, want WEIGHT_LOSS", aiMock.lastContext.GoalType)
	}
}

func TestGenerateWeeklyInsightHandler_SkipWhenNoData(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	planRepo := &mockPlanRepoForCmd{}

	// Case 1: Người dùng không có meal_history (nil)
	historyRepoNil := &mockHistoryRepoForCmd{history: nil}
	weeklyRepo1 := &mockWeeklyInsightRepo{}
	aiMock1 := &mockAIServiceForCommand{}

	handler1 := command.NewGenerateWeeklyInsightHandler(planRepo, historyRepoNil, weeklyRepo1, aiMock1)
	res1, err := handler1.Handle(ctx, command.GenerateWeeklyInsightCommand{UserID: "empty-user-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res1 != nil {
		t.Fatalf("expected nil result when history is nil, got %v", res1)
	}
	if aiMock1.lastContext.UserID != "" {
		t.Fatal("expected LLM not to be called when history is nil")
	}
	if weeklyRepo1.saved != nil {
		t.Fatal("expected no insight saved in DB when history is nil")
	}

	// Case 2: Người dùng có meal_history nhưng 7 ngày qua liên tục không log bữa ăn nào
	historyEmpty := aggregate.NewMealHistory("h-empty", "empty-user-2")
	historyRepoEmpty := &mockHistoryRepoForCmd{history: historyEmpty}
	weeklyRepo2 := &mockWeeklyInsightRepo{}
	aiMock2 := &mockAIServiceForCommand{}

	handler2 := command.NewGenerateWeeklyInsightHandler(planRepo, historyRepoEmpty, weeklyRepo2, aiMock2)
	res2, err := handler2.Handle(ctx, command.GenerateWeeklyInsightCommand{UserID: "empty-user-2"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res2 != nil {
		t.Fatalf("expected nil result when 7 days have no meals, got %v", res2)
	}
	if aiMock2.lastContext.UserID != "" {
		t.Fatal("expected LLM not to be called when 7 days have no meals")
	}
	if weeklyRepo2.saved != nil {
		t.Fatal("expected no insight saved in DB when 7 days have no meals")
	}
}
